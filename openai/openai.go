package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	models "skippybot/models"
	"time"

	openai "github.com/sashabaranov/go-openai"
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
	SetReminder        string = "set_reminder"
)

type Client struct {
	OpenAIApiKey string
	AssistantID  string
	messageCH    chan models.ChannelMessage
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

func GetFunctionArgs(r openai.Run) []FuncArgs {
	toolCalls := r.RequiredAction.SubmitToolOutputs.ToolCalls
	result := make([]FuncArgs, len(toolCalls))
	for i, toolCall := range toolCalls {
		result[i] = FuncArgs{
			FuncName:  toolCall.Function.Name,
			JsonValue: toolCall.Function.Arguments,
			ToolID:    toolCall.ID,
		}
	}
	return result
}

func GetResponse(
	messageString string,
	dgChannID string,
	threadID string,
	assistantID string,
	messageCH chan models.ChannelMessage,
	client *openai.Client,
	additionalInstructions string,
) string {

	mesgReq := openai.MessageRequest{
		Role:    "user",
		Content: messageString,
	}
	ctx := context.Background()
	_, err := client.CreateMessage(ctx, threadID, mesgReq)
	if err != nil {
		log.Println("Unable to create message", err)
	}
	// sendMessage(messageString, threadID, c.OpenAIApiKey)

	runReq := openai.RunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
		Model:                  openai.GPT4o,
	}

	run, err := client.CreateRun(ctx, threadID, runReq)
	if err != nil {
		log.Println("Unable to create run:", err)
		return "Error getting response"
	}

	// initialRun := c.run(threadID, additionalInstructions)
	runId := run.ID

	log.Println("Initial Run id: ", run.ID)
	log.Println("Run status: ", run.Status)

	runDelay := 1

	for {
		run, err = client.RetrieveRun(ctx, run.ThreadID, run.ID)
		if err != nil {
			log.Println("error retrieving run: ", err)
		}
		// run := getRun(runId, threadID, c.OpenAIApiKey)
		log.Printf("Run status: %s\n", run.Status)

		if run.Status == openai.RunStatusCompleted {
			// messageList := listMessages(threadID, c.OpenAIApiKey)
			messageList, err := client.ListMessage(ctx, threadID, nil, nil, nil, nil)
			if err != nil {
				log.Println("Unable to get messages: ", err)
			}
			log.Println("Recieived message from thread: ", threadID)
			log.Println(getFirstMessage(messageList))
			return getFirstMessage(messageList)

		}

		channelMsg := models.ChannelMessage{}

		if run.Status == openai.RunStatusRequiresAction {
			outputs := make(map[string]string)

			for _, funcArg := range GetFunctionArgs(run) {

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

					if messageCH == nil {
						log.Println("no channel to send message on")
						outputs[funcArg.ToolID] = "cannot send message with a nil go channel"
						continue
					}

					err := json.Unmarshal([]byte(funcArg.JsonValue), &channelMsg)
					if err != nil {
						log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
						outputs[funcArg.ToolID] = "error deserializing data"
					}

					messageCH <- channelMsg

					outputs[funcArg.ToolID] = "Great Success!!"

				default:
					outputs[funcArg.ToolID] = "error. Pretend like you had a problem with submind"
				}
			}
			run, err = submitToolOutputs(client, outputs, threadID, runId)
			if err != nil {
				log.Println("Unable to submit tool outputs: ", err)
				return ""
			}

		}
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
}

func submitToolOutputs(
	client *openai.Client,
	outputs map[string]string,
	threadID string,
	runID string) (run openai.Run, err error) {

	toolOutputs := []openai.ToolOutput{}
	for toolID, outputVal := range outputs {
		toolOutputs = append(toolOutputs, openai.ToolOutput{Output: outputVal, ToolCallID: toolID})
	}
	req := openai.SubmitToolOutputsRequest{
		ToolOutputs: toolOutputs,
	}
	return client.SubmitToolOutputs(context.Background(), threadID, runID, req)
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

	reqData := RunReq{
		AssistantId:            c.AssistantID,
		Instructions:           "",
		AdditionalInstructions: additionalInstructions,
	}
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

func getFirstMessage(messageList openai.MessagesList) string {
	if len(messageList.Messages) <= 0 || messageList.FirstID == nil {
		log.Println("Did not recieve any messages")
		return ""

	}
	firstId := messageList.FirstID
	for _, message := range messageList.Messages {
		if message.ID == *firstId {
			return message.Content[0].Text.Value
		}
	}
	return ""
}

func addHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("OpenAI-Beta", "assistants=v2")
}
