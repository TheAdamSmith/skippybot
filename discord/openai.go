package discord

import (
	"context"
	"encoding/json"
	"log"
	models "skippybot/models"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// ai functions
	GetStockPrice      string = "get_stock_price"
	SendChannelMessage string = "send_channel_message"
	SetReminder        string = "set_reminder"
)

type StockFuncArgs struct {
	Symbol string
}

type FuncArgs struct {
	JsonValue string
	FuncName  string
	ToolID    string
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

		log.Printf("Run status: %s\n", run.Status)

		if run.Status == openai.RunStatusCompleted {

			messageList, err := client.ListMessage(ctx, threadID, nil, nil, nil, nil)
			if err != nil {
				log.Println("Unable to get messages: ", err)
			}
			log.Println("Recieived message from thread: ", threadID)
			message := getFirstMessage(messageList)
			log.Println("Received response: ", message)
			return message

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

// TODO: construct tool ouput struct during looping
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
