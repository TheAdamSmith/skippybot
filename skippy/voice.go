package skippy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	realtime "github.com/WqyJh/go-openai-realtime"
	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

const (
	channels  int = 1                   // 1 for mono, 2 for stereo
	frameRate int = 24000               // audio sampling rate
	frameSize int = 480                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

type VoiceConnection struct {
	DiscordVoiceConn *discordgo.VoiceConnection
	RealtimeConn     *realtime.Conn
	Decoder          *gopus.Decoder
	Encoder          *gopus.Encoder
	Mu               sync.Mutex
}

func (vc *VoiceConnection) Close() (err error) {
	err = vc.DiscordVoiceConn.Disconnect()
	if err != nil {
		log.Println("Error disconnecting from voice channel: ", err)
	}

	if vc.DiscordVoiceConn.OpusRecv != nil {
		close(vc.DiscordVoiceConn.OpusRecv)
	}

	err = vc.RealtimeConn.Close()
	if err != nil {
		log.Println("error disconecting from realtime connection: ", err)
	}
	return
}

/**
TODO:
- [x] wait for connection to be ready before sending audio
- [x] store voice connection in state
- [x] proper cleanup of sockets/connections
- [x] better async handler
- [x] error handling
- create real discord command
- connect to eleven labs
- implement automatic disconnect
**/

// JoinVoiceChannel joins the voice channel that the message author is currently in
func JoinVoiceChannel(ctx context.Context, m *discordgo.MessageCreate, s *Skippy) (*VoiceConnection, error) {

	// Create a new Opus decoder
	decoder, err := gopus.NewDecoder(frameRate, channels)
	if err != nil {
		return nil, fmt.Errorf("error creating decoder: %w", err)
	}

	// Create a new Opus decoder
	encoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	if err != nil {
		return nil, fmt.Errorf("error creating encoder: %w", err)
	}

	conn, err := s.RealtimeClient.Connect(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error creating realtime connection: %w", err)

	}

	err = conn.SendMessage(context.Background(), &realtime.SessionUpdateEvent{
		Session: realtime.ClientSession{
			Modalities:              []realtime.Modality{realtime.ModalityText, realtime.ModalityAudio},
			InputAudioFormat:        realtime.AudioFormatPcm16,
			InputAudioTranscription: &realtime.InputAudioTranscription{Model: realtime.Whisper1},
			TurnDetection: &realtime.ClientTurnDetection{
				Type: realtime.ClientTurnDetectionTypeServerVad,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error sending session update event: %w", err)
	}

	waitForRealtimeReady(ctx, conn, 30*time.Second)

	channel, err := GetUserVoiceChannel(m.Author.ID, m.GuildID, s)
	if err != nil {
		conn.Close()
		return nil, err
	}

	log.Printf("Joining voice channel: %s (ID: %s) in guild: %s", channel.Name, channel.ID, m.GuildID)

	discordVC, err := s.DiscordSession.ChannelVoiceJoin(m.GuildID, channel.ID, false, false)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to join voice channel: %w", err)
	}

	log.Printf("Successfully joined voice channel: %s", channel.Name)

	vc := &VoiceConnection{
		DiscordVoiceConn: discordVC,
		RealtimeConn:     conn,
		Decoder:          decoder,
		Encoder:          encoder,
		Mu:               sync.Mutex{},
	}

	go HandleVoice(m.GuildID, vc)
	go addRealtimeHandlers(ctx, vc)

	return vc, nil
}

func addRealtimeHandlers(ctx context.Context, vc *VoiceConnection) {
	pcmBuffer := NewPCMBuffer()
	responseDeltaHandler := func(ctx context.Context, event realtime.ServerEvent) {
		switch event.ServerEventType() {
		case realtime.ServerEventTypeResponseAudioDelta:
			event, ok := event.(realtime.ResponseAudioDeltaEvent)
			if !ok {
				log.Println("could not cast message to type")
				return
			}

			if err := handleAudioDelta(event.Delta, pcmBuffer, vc); err != nil {
				log.Println("unabled to handle audio event: ", err)
			}
		}
	}

	connHandler := realtime.NewConnHandler(ctx, vc.RealtimeConn, responseDeltaHandler)
	connHandler.Start()
}

func handleAudioDelta(b64 string, pcmBuffer *PCMBuffer, vc *VoiceConnection) error {
	pcmArr, err := base64ToPCM16(b64)
	if err != nil {
		return fmt.Errorf("error could note decode base64 string to pcm: %w", err)
	}
	vc.Mu.Lock()
	defer vc.Mu.Unlock()

	pcmBuffer.Write(pcmArr)

	for {
		frame, ok := pcmBuffer.ReadFrame(frameSize)
		if !ok {
			return nil
		}

		opusFrame, err := vc.Encoder.Encode(frame, frameSize, maxBytes)
		if err != nil {
			return fmt.Errorf("opus encode at offset:  %w", err)
		}

		vc.DiscordVoiceConn.OpusSend <- opusFrame
	}
}

func waitForRealtimeReady(ctx context.Context, conn *realtime.Conn, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		msg, err := conn.ReadMessage(ctx)
		if err != nil {
			return fmt.Errorf("error reading message from realtime: %w", err)
		}
		if msg.ServerEventType() == realtime.ServerEventTypeSessionUpdated {
			log.Println("Realtime is ready")
			return nil
		}
	}
}

func HandleVoice(guildID string, vc *VoiceConnection) {
	for packet := range vc.DiscordVoiceConn.OpusRecv {
		// Decode Opus to PCM
		decodedPacket, err := vc.Decoder.Decode(packet.Opus, frameSize, false)
		if err != nil {
			log.Printf("Decoding error: %v", err)
		}

		pcm := new(bytes.Buffer)

		binary.Write(pcm, binary.LittleEndian, decodedPacket)

		packet := base64.StdEncoding.EncodeToString(pcm.Bytes())

		if err := vc.RealtimeConn.SendMessage(context.Background(), &realtime.InputAudioBufferAppendEvent{
			Audio: packet,
		}); err != nil {
			log.Printf("send audio error: %v", err)
		}
	}
	log.Println("Handle voice done channel closed for guild id: ", guildID)
}

// GetUserVoiceChannel returns the voice channel ID that a user is currently in
func GetUserVoiceChannel(userID, guildID string, s *Skippy) (*discordgo.Channel, error) {
	if guildID == "" {
		return nil, errors.New("voice channels are only available in guilds, not DMs")
	}

	guild, err := s.DiscordSession.GetState().Guild(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild: %w", err)
	}

	var userVoiceChannelID string
	for _, voiceState := range guild.VoiceStates {
		if voiceState.UserID == userID {
			userVoiceChannelID = voiceState.ChannelID
			break
		}
	}

	if userVoiceChannelID == "" {
		return nil, errors.New("user is not currently in a voice channel")
	}

	channel, err := s.DiscordSession.GetState().Channel(userVoiceChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get voice channel: %w", err)
	}

	if channel.Type != discordgo.ChannelTypeGuildVoice {
		return nil, errors.New("channel is not a voice channel")
	}

	return channel, nil
}

func base64ToPCM16(deltaB64 string) ([]int16, error) {
	// 1. Decode Base64 into raw PCM bytes (little-endian int16)
	pcmBytes, err := base64.StdEncoding.DecodeString(deltaB64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	r := bytes.NewReader(pcmBytes)
	pcm := make([]int16, len(pcmBytes)/2)
	if err := binary.Read(r, binary.LittleEndian, &pcm); err != nil {
		return nil, err
	}

	return pcm, nil
}
