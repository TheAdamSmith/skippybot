package skippy

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"github.com/dh1tw/gosamplerate"
	"github.com/haguro/elevenlabs-go"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hraban/opus"
	"github.com/sashabaranov/go-openai"
	"layeh.com/gopus"
)

const (
	sampleRate = 48000 // Discord requires 48kHz
	channels   = 2     // Discord uses stereo
	frameSize  = 960   // 20ms of audio (48kHz * 0.02s)
	maxBytes   = frameSize * 2 * 2
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
			go handleVoice(dg, channelID, vc.OpusRecv, vc, client, state)
		}
	}

	return nil
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

// TODO: just pass in connection
func handleVoice(dg DiscordSession, channelID string, c chan *discordgo.Packet, vc *discordgo.VoiceConnection, client *openai.Client, state *State) {
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
						textToSpeech(vc, "What is it?!")
						useResponse = true
					} else if useResponse {
						messageReq := openai.MessageRequest{
							Role:    openai.ChatMessageRoleUser,
							Content: result.Text,
						}

						startTime := time.Now()
						response, err := GetResponse(
							context.Background(),
							dg,
							channelID,
							messageReq,
							"You are in a discord voice call. This message is coming from speech to text and the output will be going to speech to text. Be Brief and conversational with your reply",
							true,
							client,
							state,
							nil,
							nil,
						)
						log.Println("getting response took", time.Now().Sub(startTime))
						if err != nil {
							log.Println(err)
						}
						textToSpeech(vc, response)
						useResponse = false
					}
				}
				buffer.Reset()
			}
		}
	}
}

func textToSpeech(vc *discordgo.VoiceConnection, text string) {
	apiKey := os.Getenv("ELEVEN_LABS_API_KEY")
	client := elevenlabs.NewClient(context.Background(), apiKey, 30*time.Second)
	ttsReq := elevenlabs.TextToSpeechRequest{
		Text:    text,
		ModelID: "eleven_multilingual_v2",
	}

	log.Println("generating audio")
	startTime := time.Now()
	audio, err := client.TextToSpeech("ITkrDjydYTCnhgCuIWlv", ttsReq)
	if err != nil {
		log.Println(err)
	}
	log.Println("cloning voice took", time.Now().Sub(startTime))

	startTime = time.Now()
	if err := os.WriteFile("eleven_labs_out.mp3", audio, 0644); err != nil {
		log.Println(err)
	}
	log.Println("writing file took", time.Now().Sub(startTime))

	stop := make(chan bool)
	defer close(stop)
	log.Println("starting audio playback")
	dgvoice.PlayAudioFile(vc, "eleven_labs_out.mp3", stop)
}

func decodeMP3ToPCM(mp3Data []byte) ([]float32, int, int, error) {
	reader := bytes.NewReader(mp3Data)
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		return nil, 0, 0, err
	}
	sampleRate := decoder.SampleRate()
	channels := 2 // Assuming stereo audio

	buf := make([]byte, 4096)
	samples := make([]float32, 0)

	for {
		n, err := decoder.Read(buf)
		if n > 0 {
			// Convert bytes to float32 samples
			for i := 0; i < n; i += 2 {
				if i+1 >= len(buf) {
					break
				}
				sample := int16(buf[i]) | int16(buf[i+1])<<8
				// Normalize to range [-1.0, 1.0]
				fSample := float32(sample) / 32768.0
				samples = append(samples, fSample)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, err
		}
	}
	return samples, sampleRate, channels, nil
}

func resamplePCM(samples []float32, inSampleRate, outSampleRate, channels int) ([]float32, error) {
	if inSampleRate == outSampleRate {
		// If the input sample rate is the same as the output, return the samples directly
		return samples, nil
	}

	// Create a new resampler with the given input and output sample rates and channels
	// resampler, err := gosamplerate.New(gosamplerate.BestQuality, channels)
	// if err != nil {
	// 	return nil, err
	// }
	// defer resampler.Close()

	// Calculate the resampling ratio
	ratio := float64(outSampleRate) / float64(inSampleRate)

	return gosamplerate.Simple(samples, ratio, 2, gosamplerate.SRC_LINEAR)
	// // Perform the resampling
	// resampledSamples, err := resampler.Process(samples, ratio)
	// if err != nil {
	// 	return nil, err
	// }
}

func float32ToInt16(samples []float32) []int16 {
	intSamples := make([]int16, len(samples))
	for i, sample := range samples {
		// Clip the sample to [-1.0, 1.0]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		intSamples[i] = int16(sample * 32767)
	}
	return intSamples
}

func encodePCMToOpus(samples []int16, sampleRate, channels int) ([]byte, error) {
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, err
	}
	output := bytes.NewBuffer(nil)
	frameSize := 960     // 20ms frames at 48000 Hz
	maxDataBytes := 4000 // Maximum size of the Opus packet

	for i := 0; i < len(samples); i += frameSize * channels {
		var frame []int16
		if i+frameSize*channels > len(samples) {
			// Pad the last frame with zeros
			frame = make([]int16, frameSize*channels)
			copy(frame, samples[i:])
		} else {
			frame = samples[i : i+frameSize*channels]
		}
		data := make([]byte, maxDataBytes)
		n, err := enc.Encode(frame, data)
		if err != nil {
			return nil, err
		}
		output.Write(data[:n])
	}
	return output.Bytes(), nil
}
