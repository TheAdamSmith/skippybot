package skippy

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
	"layeh.com/gopus"
)

type STTResult struct {
	Text string `json:"text"`
}

func joinVoice(dg DiscordSession, guildID string, userID string, channelID string, client *openai.Client, state *State) error {
	guild, err := dg.GetState().Guild(guildID)
	if err != nil {
		return fmt.Errorf("error could not join voice %w", err)
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			// join voice
			vc, err := dg.ChannelVoiceJoin(guildID, vs.ChannelID, false, false)
			if err != nil {
				return fmt.Errorf("error could not join voice channel %w", err)
			}
			state.AddVoiceConnection(vs.ChannelID, vc)
			go handleVoice(dg, channelID, vc.OpusRecv, client, state)
		}
	}

	return nil
}

func handleVoice(dg DiscordSession, channelID string, c chan *discordgo.Packet, client *openai.Client, state *State) {
	// TODO only instantiate one
	model, _ := vosk.NewModel("./vosk_models/en") // path to Vosk model (default: english-very-small)
	defer model.Free()
	stt, _ := vosk.NewRecognizer(model, 48000) // 48kHz
	defer stt.Free()
	speakers, _ := gopus.NewDecoder(48000, 1) // 48kHz and mono-channel

	buffer := new(bytes.Buffer)
	useResponse := false
	for {
		select {
		case s, ok := <-c:
			if !ok {
				break
			}
			if buffer == nil {
				buffer = new(bytes.Buffer)
			}
			packet, _ := speakers.Decode(s.Opus, 960, false) // frameSize is 960 (20ms)
			pcm := new(bytes.Buffer)
			binary.Write(pcm, binary.LittleEndian, packet)
			buffer.Write(pcm.Bytes())
			stt.AcceptWaveform(pcm.Bytes())

			var dur float32 = (float32(len(buffer.Bytes())) / 48000 / 2) // duration of audio

			// When silence packet detected, send result (skip audio shorter than 500ms)
			if dur > 0.5 && len(s.Opus) == 3 && s.Opus[0] == 248 && s.Opus[1] == 255 && s.Opus[2] == 254 {
				var result STTResult
				json.Unmarshal([]byte(stt.FinalResult()), &result)
				if len(result.Text) > 0 {
					log.Println(result.Text, useResponse)
					// process the transcription result:
					if result.Text == "hey skippy" {
						useResponse = true
					} else if useResponse {
						messageReq := openai.MessageRequest{
							Role:    openai.ChatMessageRoleUser,
							Content: result.Text,
						}
						getAndSendResponseWithoutTools(context.Background(), dg, channelID, messageReq, "", client, state)
						useResponse = false
					}
				}
				buffer.Reset()
			}
		}
	}
}

func leaveVoice(dg DiscordSession, guildID string, userID string, state *State) error {
	guild, err := dg.GetState().Guild(guildID)
	if err != nil {
		return fmt.Errorf("error could not join voice %w", err)
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			vc := state.GetVoiceConnection(vs.ChannelID)
			if vc == nil {
				return fmt.Errorf("error could not leave voice channel as it does not exist in state")
			}
			err := vc.Disconnect()
			if err != nil {
				return fmt.Errorf("error could not leave voice %w", err)
			}

			if vc.OpusRecv != nil {
				close(vc.OpusRecv)
			}

			vc.Close()
			state.RemoveVoiceConnection(vs.ChannelID)
		}
	}

	return nil
}
