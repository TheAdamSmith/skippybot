package skippy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	realtime "github.com/WqyJh/go-openai-realtime"
	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

var (
	voiceConnection *discordgo.VoiceConnection
	rtConn          *realtime.Conn
)

// JoinVoiceChannel joins the voice channel that the message author is currently in
func JoinVoiceChannel(m *discordgo.MessageCreate, s *Skippy) (*discordgo.VoiceConnection, error) {
	if m.GuildID == "" {
		return nil, errors.New("voice channels are only available in guilds, not DMs")
	}

	guild, err := s.DiscordSession.GetState().Guild(m.GuildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild: %w", err)
	}

	var userVoiceChannelID string
	for _, voiceState := range guild.VoiceStates {
		if voiceState.UserID == m.Author.ID {
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

	log.Printf("Joining voice channel: %s (ID: %s) in guild: %s", channel.Name, userVoiceChannelID, m.GuildID)

	vc, err := s.DiscordSession.ChannelVoiceJoin(m.GuildID, userVoiceChannelID, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to join voice channel: %w", err)
	}

	openAIKey := os.Getenv("OPEN_AI_KEY")
	client := realtime.NewClient(openAIKey)
	conn, err := client.Connect(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error creating realtime connection: %w", err)

	}
	rtConn = conn
	voiceConnection = vc

	log.Printf("Successfully joined voice channel: %s", channel.Name)

	// createResponse := true
	err = conn.SendMessage(context.Background(), &realtime.SessionUpdateEvent{
		Session: realtime.ClientSession{
			Modalities:              []realtime.Modality{realtime.ModalityText},
			InputAudioFormat:        realtime.AudioFormatPcm16,
			InputAudioTranscription: &realtime.InputAudioTranscription{Model: realtime.Whisper1},
			TurnDetection: &realtime.ClientTurnDetection{
				Type: realtime.ClientTurnDetectionTypeServerVad,
				// TurnDetectionParams: realtime.TurnDetectionParams{
				// 	SilenceDurationMs: 500, // <-- wait 500 ms of silence
				// 	PrefixPaddingMs:   300,
				// 	Threshold:         0,
				// 	CreateResponse:    &createResponse,
				// },
			},
		},
	})
	if err != nil {
		log.Printf("session update error: %v", err)
	}
	go HandleVoice(s)
	go Read(rtConn)
	return vc, nil
}

func Read(conn *realtime.Conn) {
	for {
		msg, err := conn.ReadMessage(context.Background())
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}
		log.Println(msg.ServerEventType())
		// data, err := json.Marshal(msg)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// fmt.Printf("[%s] %s\n\n", msg.ServerEventType(), string(data))
		// fmt.Printf("[%s]\n\n", event.ServerEventType())
		switch msg.ServerEventType() {
		case realtime.ServerEventTypeResponseAudioTranscriptDone:
			fmt.Printf("[response] %s\n", msg.(realtime.ResponseAudioTranscriptDoneEvent).Transcript)
		case realtime.ServerEventTypeResponseTextDelta:
			// ev.Content is the incremental text
			fmt.Print(msg.(realtime.ResponseTextDeltaEvent).Delta)
		case realtime.ServerEventTypeError:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("[%s] %s\n", msg.ServerEventType(), string(data))
		default:
			// ignore other event types
		}
	}
}

func HandleVoice(s *Skippy) {
	// Create a new Opus decoder
	decoder, err := gopus.NewDecoder(48000, 2)
	if err != nil {
		log.Fatalf("Failed to create decoder: %v", err)
	}

	for packet := range voiceConnection.OpusRecv {
		// Decode Opus to PCM
		decodedPacket, err := decoder.Decode(packet.Opus, 960, false)
		if err != nil {
			log.Printf("Decoding error: %v", err)
		}

		pcm := new(bytes.Buffer)

		binary.Write(pcm, binary.LittleEndian, decodedPacket)

		packet := base64.StdEncoding.EncodeToString(pcm.Bytes())
		// if len(packet) == 0 {
		// 	continue
		// }

		if err := rtConn.SendMessage(context.Background(), &realtime.InputAudioBufferAppendEvent{
			Audio: packet,
		}); err != nil {
			log.Printf("send audio error: %v", err)
		} // signal end of audio
		// buffer.Write(pcm.Bytes())
		// // Convert PCM []int16 to []byte
		// buf := make([]byte, len(pcm)*2)
		// for i, v := range pcm {
		// 	buf[i*2] = byte(v)
		// 	buf[i*2+1] = byte(v >> 8)
		// }

		// var dur float32 = (float32(len(buffer.Bytes())) / 48000 / 2) // duration of audio
		//
		// // When silence packet detected, send result (skip audio shorter than 500ms)
		// if dur > 0.5 && len(packet.Opus) == 3 && packet.Opus[0] == 248 && packet.Opus[1] == 255 && packet.Opus[2] == 254 {
		// 	packet := base64.StdEncoding.EncodeToString(buffer.Bytes())
		// 	if err := rtConn.SendMessage(context.Background(), &realtime.InputAudioBufferAppendEvent{
		// 		Audio: packet,
		// 	}); err != nil {
		// 		log.Printf("send audio error: %v", err)
		// 	} // signal end of audio

		// err = rtConn.SendMessage(context.Background(), &realtime.InputAudioBufferCommitEvent{})
		// if err != nil {
		// 	log.Println("commit audio error", err)
		// }
		// buffer.Reset()
		// rtConn.SendMessage(context.Background(), &realtime.ResponseCreateEvent{
		// 	Response: realtime.ResponseCreateParams{
		// 		Modalities:      []realtime.Modality{realtime.ModalityAudio, realtime.ModalityText},
		// 		MaxOutputTokens: 4000,
		// 	},
		// })
		// }

	}
	log.Println("channel closed")
}

// LeaveVoiceChannel leaves the voice channel using the global voice connection
func LeaveVoiceChannel(guildID string, s *Skippy) error {
	if guildID == "" {
		return errors.New("guild ID is required")
	}

	log.Printf("Leaving voice channel in guild: %s", guildID)

	vc := voiceConnection

	if vc == nil {
		return errors.New("no active voice connection found")
	}

	if vc.GuildID != guildID {
		return fmt.Errorf("voice connection is for guild %s, not %s", vc.GuildID, guildID)
	}

	err := vc.Disconnect()
	if err != nil {
		log.Printf("Error disconnecting from voice channel: %v", err)
	}

	if vc.OpusRecv != nil {
		close(vc.OpusRecv)
	}

	vc.Close()
	voiceConnection = nil
	rtConn.Close()
	log.Printf("Successfully left voice channel in guild: %s", guildID)
	return nil
}

// GetUserVoiceChannel returns the voice channel ID that a user is currently in
func GetUserVoiceChannel(userID, guildID string, s *Skippy) (string, error) {
	if guildID == "" {
		return "", errors.New("guild ID is required")
	}

	// Get the guild from the state
	guild, err := s.DiscordSession.GetState().Guild(guildID)
	if err != nil {
		return "", fmt.Errorf("failed to get guild: %w", err)
	}

	// Find the voice channel the user is currently in
	for _, voiceState := range guild.VoiceStates {
		if voiceState.UserID == userID {
			return voiceState.ChannelID, nil
		}
	}

	return "", errors.New("user is not currently in a voice channel")
}
