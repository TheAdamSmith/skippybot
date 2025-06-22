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
	"sync"

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

var (
	pcmOut          *os.File
	voiceConnection *discordgo.VoiceConnection
	rtConn          *realtime.Conn
	gopusDecoder    *gopus.Decoder
	gopusEncoder    *gopus.Encoder
	mu              sync.Mutex
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

	// Create a new Opus decoder
	decoder, err := gopus.NewDecoder(frameRate, channels)
	if err != nil {
		log.Fatalf("Failed to create decoder: %v", err)
	}
	gopusDecoder = decoder

	// Create a new Opus decoder
	encoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	if err != nil {
		log.Fatalf("Failed to create decoder: %v", err)
	}
	gopusEncoder = encoder
	log.Printf("Successfully joined voice channel: %s", channel.Name)

	mu = sync.Mutex{}

	// createResponse := true
	// interruptResponse := false
	err = conn.SendMessage(context.Background(), &realtime.SessionUpdateEvent{
		Session: realtime.ClientSession{
			Modalities:              []realtime.Modality{realtime.ModalityText, realtime.ModalityAudio},
			InputAudioFormat:        realtime.AudioFormatPcm16,
			InputAudioTranscription: &realtime.InputAudioTranscription{Model: realtime.Whisper1},
			TurnDetection: &realtime.ClientTurnDetection{
				Type: realtime.ClientTurnDetectionTypeServerVad,
				// TurnDetectionParams: realtime.TurnDetectionParams{
				// 	// SilenceDurationMs: 1000, // <-- wait 500 ms of silence
				// 	// PrefixPaddingMs:   300,
				// 	// Threshold:         0.5,
				// 	// Eagerness: "low",
				// 	// InterruptResponse: &interruptResponse,
				// 	// CreateResponse:    &createResponse,
				// },
			},
		},
	})
	if err != nil {
		log.Printf("session update error: %v", err)
	}

	pcmOut, err = os.Create("response.opus")
	if err != nil {
		log.Fatalf("failed to create PCM output file: %v", err)
	}

	go HandleVoice(s)
	go Read(rtConn)
	return vc, nil
}

func Read(conn *realtime.Conn) {
	defer func() {
		if pcmOut != nil {
			pcmOut.Close()
		}
	}()
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
		case realtime.ServerEventTypeResponseAudioDelta:
			log.Println("packet")
			event, ok := msg.(realtime.ResponseAudioDeltaEvent)
			if !ok {
				log.Println("could not cast message to type")
			}
			SendResponsePacket(event.Delta)
		case realtime.ServerEventTypeResponseAudioTranscriptDone:
			log.Println(msg.(realtime.ResponseAudioTranscriptDoneEvent).Transcript)
		case realtime.ServerEventTypeResponseTextDelta:
			// ev.Content is the incremental text
			log.Println(msg.(realtime.ResponseTextDeltaEvent).Delta)
		case realtime.ServerEventTypeRateLimitsUpdated:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("[%s] %s\n", msg.ServerEventType(), string(data))
		case realtime.ServerEventTypeConversationItemInputAudioTranscriptionCompleted:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("[%s] %s\n", msg.ServerEventType(), string(data))
		case realtime.ServerEventTypeSessionUpdated:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("[%s] %s\n", msg.ServerEventType(), string(data))
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

func SendResponsePacket(b64 string) {
	mu.Lock()
	defer mu.Unlock()
	if err := voiceConnection.Speaking(true); err != nil {
		log.Println("error speaking: ", err)
	}
	_, err := pcm16Base64ToOpus(b64, gopusEncoder)
	if err != nil {
		log.Println("unable to convert packet to opus: ", err)
	}

	if err := voiceConnection.Speaking(false); err != nil {
		log.Println("error speaking: ", err)
	}
}

func HandleVoice(s *Skippy) {

	for packet := range voiceConnection.OpusRecv {
		// Decode Opus to PCM
		decodedPacket, err := gopusDecoder.Decode(packet.Opus, frameSize, false)
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

func pcm16Base64ToOpus(deltaB64 string, encoder *gopus.Encoder) ([]byte, error) {
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
	// // 2. Convert []byte to []int16 samples
	// sampleCount := len(pcmBytes) / 2
	// samples := make([]int16, sampleCount)
	// for i := 0; i < sampleCount; i++ {
	// 	samples[i] = int16(binary.LittleEndian.Uint16(pcmBytes[2*i:]))
	// }

	// 3. Chunk into frameSize blocks and encode
	var opusBuf bytes.Buffer
	for off := 0; off < len(pcm); off += frameSize {
		end := off + frameSize

		var frame []int16
		if end > len(pcm) {
			// pad the final frame with zeros
			frame = make([]int16, frameSize)
			copy(frame, pcm[off:])
		} else {
			frame = pcm[off:end]
		}

		opusFrame, err := encoder.Encode(frame, frameSize, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("opus encode at offset %d: %w", off, err)
		}

		voiceConnection.OpusSend <- opusFrame
		opusBuf.Write(opusFrame)
		// optional: write each opus packet to a file
		if _, err := pcmOut.Write(opusFrame); err != nil {
			log.Printf("write opus to file: %v", err)
		}
	}
	return opusBuf.Bytes(), nil
}
