package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	models "skippybot/models"
)

const (
	// run statuses TODO: group into struct
	InProgress     string = "in_progress"
	Queued         string = "queued"
	Compeleted     string = "completed"
	RequiresAction string = "requires_action"
	// ai functions
	GetStockPrice      string = "get_stock_price"
	SendChannelMessage string = "send_channel_message"
	SetReminder string = "set_reminder"
)

type Client struct {
	OpenAIApiKey string
	AssistantID  string
  messageCH chan models.ChannelMessage
}

func NewClient(apiKey string, assistantID string) *Client {
	return &Client{OpenAIApiKey: apiKey, AssistantID: assistantID}
}

func (c *Client) SetMessageCH(ch chan models.ChannelMessage) {
  c.messageCH = ch
}

type StockFuncArgs struct {
	Symbol string
}

func (c *Client) GetResponse(messageString string, dgChannID string,  threadID string, additionalInstructions string) string {
	sendMessage(messageString, threadID, c.OpenAIApiKey)
	initialRun := c.run(threadID, additionalInstructions)
	runId := initialRun.ID

	log.Println("Initial Run id: ", initialRun.ID)
	log.Println("Run status: ", initialRun.Status)

	runDelay := 1

	for {
		run := getRun(runId, threadID, c.OpenAIApiKey)
		log.Printf("Run status: %s\n", run.Status)
		if run.Status == Compeleted {
			messageList := listMessages(threadID, c.OpenAIApiKey)
			log.Println("Recieived message from thread: ", threadID)
			log.Println(getFirstMessage(messageList))
			return getFirstMessage(messageList)

		}

    channelMsg := models.ChannelMessage{}
    
		if run.Status == RequiresAction {
			funcArgs := run.GetFunctionArgs()[0]
			outputs := make(map[string]string)
			for _, funcArg := range run.GetFunctionArgs() {
				log.Println(funcArgs.FuncName)
				log.Println(funcArgs.ToolID)
        log.Println(funcArgs.JsonValue)
				switch funcName := funcArg.FuncName; funcName {
				case GetStockPrice:
					log.Println("get_stock_price(): sending 150: ")
					outputs[funcArg.ToolID] = "150"

        case SetReminder: 
					log.Println("set_reminder()")
          channelMsg.ChannelID = dgChannID

          fallthrough
				case SendChannelMessage:
					log.Println("send_channel_message()")

          if c.messageCH == nil {
            log.Println("no channel to send message on")
            outputs[funcArgs.ToolID] = "cannot send message with a nil go channel"
            continue
          }

					err := json.Unmarshal([]byte(funcArgs.JsonValue), &channelMsg)
					if err != nil {
						log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
					  outputs[funcArg.ToolID] = "error deserializing data"
					}

          c.messageCH <- channelMsg

					outputs[funcArg.ToolID] = "Great Success!!"

				default:
					outputs[funcArg.ToolID] = "error. Pretend like you had a problem with submind"
				}
			}
			submitToolOutputs(outputs, threadID, runId, c.OpenAIApiKey)

		}
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
}

func submitToolOutputs(outputs map[string]string, threadID string, runID, apiKey string) {
	url := "https://api.openai.com/v1/threads/" + threadID + "/runs/" + runID + "/submit_tool_outputs"

	toolOutputs := []ToolOutput{}
	for toolID, outputVal := range outputs {
		toolOutputs = append(toolOutputs, ToolOutput{Output: outputVal, ToolCallID: toolID})
	}

	tooResponse := ToolResponse{ToolOutputs: toolOutputs}
	jsonData, err := json.Marshal(tooResponse)
	if err != nil {
		fmt.Printf("Error Marshal %s\n", err)
	}

	log.Println(string(jsonData))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	addHeaders(req, apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("POST error: ", err)
	}

	defer resp.Body.Close()
  log.Println("submit_tool_outputs responded with: ", resp.Status)
}

func (c *Client) StartThread() Thread {
	url := "https://api.openai.com/v1/threads"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
	addHeaders(req, c.OpenAIApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("POST error %s\n", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading resp %s\n", err)
	}

	var thread Thread
	if err := json.Unmarshal(responseBody, &thread); err != nil {
		log.Printf("Error unmarshalling thread:  %s\n", err)
	}
	log.Println("Successfully started thread: ", thread.ID)
	return thread
}

func (c *Client) ListAssistants() {
	url := "https://api.openai.com/v1/assistants"
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, c.OpenAIApiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("GET error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
		return
	}
	fmt.Println("Response:", string(responseBody))
}

func (c *Client) run(threadID string, additionalInstructions string) Run {
	url := "https://api.openai.com/v1/threads/" + threadID + "/runs"

	reqData := RunReq{AssistantId: c.AssistantID, Instructions: "", AdditionalInstructions: additionalInstructions}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		log.Println("Error Marshalling: ", err)
	}
	log.Println("Request Data: ", string(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	addHeaders(req, c.OpenAIApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("POST error: ", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading resp: ", err)
	}
	var run Run
	if err := json.Unmarshal(responseBody, &run); err != nil {
		log.Println("Error unmarshalling: ", err)
	}

	return run
}

func sendMessage(messageString string, threadId string, apiKey string) {
	url := "https://api.openai.com/v1/threads/" + threadId + "/messages"

	message := MessageReq{Role: "user", Content: messageString}
	jsonData, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("Error Marshal %s\n", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	addHeaders(req, apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("POST error: ", err)
	}

	defer resp.Body.Close()

}

func getRun(runId string, threadId string, apiKey string) Run {
	url := "https://api.openai.com/v1/threads/" + threadId + "/runs/" + runId
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("GET error: ", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading resp: ", err)
	}

	var run Run
	if err := json.Unmarshal(responseBody, &run); err != nil {
		log.Println("Error unmarshalling:", err)
	}

	return run
}

func listMessages(threadId string, apiKey string) MessageList {
	url := "https://api.openai.com/v1/threads/" + threadId + "/messages"
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("GET error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading resp: ", err)
	}

	var messageList MessageList
	if err := json.Unmarshal(responseBody, &messageList); err != nil {
		log.Println("Error unmarshalling: ", err)
	}

	return messageList
}

func getFirstMessage(messageList MessageList) string {
	if len(messageList.Data) <= 0 || messageList.FirstID == "" {
		log.Println("Did not recieve any messages")
		return ""

	}
	firstId := messageList.FirstID
	for _, message := range messageList.Data {
		if message.ID == firstId {
			return message.Content[0].Text.Value
		}
	}
	return ""
}

func addHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("OpenAI-Beta", "assistants=v1")
}
