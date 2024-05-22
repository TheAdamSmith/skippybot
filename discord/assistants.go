package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// ai functions
	GetStockPrice      string = "get_stock_price"
	SendChannelMessage string = "send_channel_message"
	SetReminder        string = "set_reminder"
	GenerateImage      string = "generate_image"
)

type StockFuncArgs struct {
	Symbol string
}

type GenerateImageFuncArgs struct {
	Prompt string `json:"prompt"`
}

type FuncArgs struct {
	JsonValue string
	FuncName  string
	ToolID    string
}

func GetResponse(
	ctx context.Context,
	messageString string,
	messageCH chan ChannelMessage,
	client *openai.Client,
	additionalInstructions string,
) (string, error) {

	mesgReq := openai.MessageRequest{
		Role:    "user",
		Content: messageString,
	}

	threadID, ok := ctx.Value(ThreadID).(string)
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find context value: %s", string(ThreadID)))
	}

	dgChannID, ok := ctx.Value(DGChannelID).(string)
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find context value: %s", string(DGChannelID)))
	}

	assistantID, ok := ctx.Value(AssistantID).(string)
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find context value: %s", string(DGChannelID)))
	}

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
		return "", err
	}

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
				return "", fmt.Errorf("Unable to get messages: ", err)
			}

			log.Println("Recieived message from thread: ", threadID)
			message, err := getFirstMessage(messageList)
			if err != nil {
				return "", fmt.Errorf("Unable to get first message: ", err)
			}
			log.Println("Received response from ai: ", message)
			return message, nil

		}

		// TODO: this used across multiple function could cause issues
		channelMsg := ChannelMessage{}

		if run.Status == openai.RunStatusRequiresAction {

			funcArgs := GetFunctionArgs(run)
			outputs := make(map[string]string)
			toolOutputs := make([]openai.ToolOutput, len(funcArgs))

			for i, funcArg := range funcArgs {

				switch funcName := funcArg.FuncName; funcName {

				case GetStockPrice:
					log.Println("get_stock_price(): sending 150: ")
					toolOutputs[i] = openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: "150"}
				case GenerateImage:
					generateImageFuncArgs := GenerateImageFuncArgs{}
					err := json.Unmarshal([]byte(funcArg.JsonValue), &generateImageFuncArgs)
					if err != nil {
						log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
						toolOutputs[i] = openai.ToolOutput{
							ToolCallID: funcArg.ToolID,
							Output:     "error deserializing data",
						}
						continue
					}

					imgUrl, err := GetImgUrl(generateImageFuncArgs.Prompt, client)
					if err != nil {
						log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
						toolOutputs[i] = openai.ToolOutput{
							ToolCallID: funcArg.ToolID,
							Output:     "error getting image data",
						}
						continue
					}
					channelMsg.ChannelID = dgChannID
					channelMsg.Message = imgUrl
					messageCH <- channelMsg
					toolOutputs[i] = openai.ToolOutput{
						ToolCallID: funcArg.ToolID,
						Output:     "worked",
					}

				case SetReminder:
					log.Println("set_reminder()")
					channelMsg.ChannelID = dgChannID
					channelMsg.IsReminder = true
					fallthrough

				case SendChannelMessage:
					log.Println("send_channel_message()")

					if messageCH == nil {
						log.Println("no channel to send message on")
						toolOutputs[i] = openai.ToolOutput{
							ToolCallID: funcArg.ToolID,
							Output:     "cannot send message with a nil go channel",
						}
						continue
					}

					err := json.Unmarshal([]byte(funcArg.JsonValue), &channelMsg)
					if err != nil {
						log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
						outputs[funcArg.ToolID] = "error deserializing data"
					}

					messageCH <- channelMsg

					toolOutputs[i] = openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: "worked"}
					outputs[funcArg.ToolID] = "Great Success!!"

				default:
					toolOutputs[i] = openai.ToolOutput{
						ToolCallID: funcArg.ToolID,
						Output:     "error. Pretend like you had a problem with submind",
					}
				}
			}
			run, err = submitToolOutputs(client, toolOutputs, threadID, runId)
			if err != nil {
				return "", fmt.Errorf("Unable to submit tool outputs: %s", err)
			}

		}
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
}

// TODO: construct tool ouput struct during looping
func submitToolOutputs(
	client *openai.Client,
	toolOutputs []openai.ToolOutput,
	threadID string,
	runID string) (run openai.Run, err error) {

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

func getFirstMessage(messageList openai.MessagesList) (string, error) {
	if len(messageList.Messages) <= 0 || messageList.FirstID == nil {
		return "", errors.New("recieved zero length message list")

	}
	firstId := messageList.FirstID
	for _, message := range messageList.Messages {
		if message.ID == *firstId {
			return message.Content[0].Text.Value, nil
		}
	}
	return "", fmt.Errorf("could not find message with id: %s", *firstId)
}