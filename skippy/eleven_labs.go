package skippy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"layeh.com/gopus"
)

/*
*
TODO:
- [] fix audio stability issues
- [x] make sure to recieve the whole response
*
*/
type LabsClient struct {
	apiKey       string
	voiceID      string
	modelID      string
	outputFormat string
	conn         *websocket.Conn
	Text         chan string
	VC           *discordgo.VoiceConnection
	done         chan struct{}
	encoder      *gopus.Encoder
	// decoder      *gopus.Decoder TODO:
}

func NewLabsClient(apiKey, voiceID, modelID, outputFormat string, vc *discordgo.VoiceConnection) *LabsClient {
	return &LabsClient{
		apiKey:       apiKey,
		voiceID:      voiceID,
		modelID:      modelID,
		outputFormat: outputFormat,
		Text:         make(chan string),
		VC:           vc,
		done:         make(chan struct{}),
	}
}

func (lc *LabsClient) Start() error {
	url := fmt.Sprintf(
		"wss://api.elevenlabs.io/v1/text-to-speech/%s/stream-input?model_id=%s&output_format=%s",
		lc.voiceID, lc.modelID, lc.outputFormat,
	)
	headers := http.Header{
		"xi-api-key": []string{lc.apiKey},
	}

	conn, resp, err := websocket.DefaultDialer.Dial(url, headers)
	log.Println(resp.StatusCode)
	if err != nil {
		return fmt.Errorf("dial error: %w (http %d)", err, resp.StatusCode)
	}
	lc.conn = conn

	// initialize connection per docs: send blank text first
	if err := lc.conn.WriteJSON(map[string]any{
		"text": " ",
		// "voice_settings": map[string]float64{
		// 	"speed":            1.0,
		// 	"stability":        0.5,
		// 	"similarity_boost": 0.8,
		// },
	},
	); err != nil {
		return fmt.Errorf("failed to send init blank text: %w", err)
	}

	go lc.heartbeat()
	go lc.readLoop()
	go lc.writeLoop()

	return nil
}

func (lc *LabsClient) heartbeat() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Println("sending heartbeat")
			// send a space as heartbeat
			if err := lc.conn.WriteJSON(map[string]string{"text": " "}); err != nil {
				log.Println("error heartbeat: ", err)
			}
		case <-lc.done:
			return
		}
	}
}

func (lc *LabsClient) writeLoop() {
	for text := range lc.Text {
		log.Println("text: ", text)
		if text == "" {
			lc.flush()
		}

		if err := lc.conn.WriteJSON(map[string]any{
			"text":                   text + " ",
			"try_trigger_generation": true,
		}); err != nil {
			log.Println("write error:", err)
			return
		}
	}
	// on Text close, send EOS
	lc.conn.WriteJSON(map[string]string{"text": ""})
}

func (lc *LabsClient) flush() {
	log.Println("flushing...")
	if err := lc.conn.WriteJSON(map[string]any{
		"text":  " ",
		"flush": true,
	}); err != nil {
		log.Println("flush error:", err)
		return
	}
}

func (lc *LabsClient) readLoop() {
	type audioMsg struct {
		Audio   string `json:"audio"`
		IsFinal bool   `json:"is_final"`
	}

	pcmBuffer := NewPCMBuffer()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	// bufferSize := 10
	// opusBuffer := make(chan []byte, bufferSize)
	// frameSent := 0
	// go func() {
	// 	for {
	// 		if frameSent > bufferSize {
	// 			break
	// 		}
	// 	}
	//
	// 	for opusPacket := range opusBuffer {
	// 	}
	// }()

	for {
		_, buf, err := lc.conn.ReadMessage()
		log.Println("recieved message")
		if err != nil {
			// if we're closing, swallow the error
			select {
			case <-lc.done:
				log.Println("done")
				return
			default:
				log.Println("read error:", err)
				return
			}
		}
		var msg audioMsg
		if err := json.Unmarshal(buf, &msg); err != nil {
			log.Println("error unmarshalling", err)
			continue
		}
		if msg.IsFinal {
			log.Println("message is final returning")
			return
		}
		log.Println("read message decoding and sending")

		pcmArr, err := base64ToPCM16(msg.Audio)
		if err != nil {
			log.Println("error could note decode base64 string to pcm: ", err)
		}

		pcmBuffer.Write(pcmArr)

		for {
			frame, ok := pcmBuffer.ReadFrame(frameSize)
			if !ok {
				break
			}

			opusFrame, err := lc.encoder.Encode(frame, frameSize, maxBytes)
			if err != nil {
				log.Println("opus encode error: ", err)
			}

			lc.VC.OpusSend <- opusFrame
			// opusBuffer <- opusFrame
			//
			// frameSent++
			// log.Println(frameSent)
		}
	}
}

// Close shuts down the LabsClient:
// 1. stops writeLoop by closing Text
// 2. signals heartbeat & readLoop via done
// 3. closes the WebSocket itself
func (lc *LabsClient) Close() error {
	close(lc.done)
	close(lc.Text)
	return lc.conn.Close()
}
