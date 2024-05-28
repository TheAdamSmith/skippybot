package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

const (
	// ai functions
	GetStockPrice        string = "get_stock_price"
	SendChannelMessage   string = "send_channel_message"
	SetReminder          string = "set_reminder"
	GenerateImage        string = "generate_image"
	ToggleMorningMessage string = "toggle_morning_message"
)

type FuncArgs struct {
	JsonValue string
	FuncName  string
	ToolID    string
}

type StockFuncArgs struct {
	Symbol string
}

type GenerateImageFuncArgs struct {
	Prompt string `json:"prompt"`
}

type MorningMsgFuncArgs struct {
	Enable bool   `json:"enable"`
	Time   string `json:"time"`
}

func GetResponse(
	ctx context.Context,
	dg *discordgo.Session,
	messageReq openai.MessageRequest,
	state *State,
	client *openai.Client,
	additionalInstructions string,
) (string, error) {

	threadID, ok := ctx.Value(ThreadID).(string)
	if !ok {
		return "", fmt.Errorf("could not find context value: %s", string(ThreadID))
	}

	dgChannID, ok := ctx.Value(DGChannelID).(string)
	if !ok {
		return "", fmt.Errorf("could not find context value: %s", string(DGChannelID))
	}

	assistantID, ok := ctx.Value(AssistantID).(string)
	if !ok {
		return "", fmt.Errorf("could not find context value: %s", string(DGChannelID))
	}

	disableFunctions, ok := ctx.Value(DisableFunctions).(bool)
	if !ok {
		disableFunctions = false
	}

	_, err := client.CreateMessage(ctx, threadID, messageReq)
	if err != nil {
		log.Println("Unable to create message", err)
	}

	runReq := openai.RunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
		Model:                  openai.GPT4o,
	}

	if disableFunctions {
		runReq.ToolChoice = "none"
	}

	log.Printf("TOOLS: %v\n", runReq.Tools)
	run, err := client.CreateRun(ctx, threadID, runReq)
	if err != nil {
		log.Println("Unable to create run:", err)
		return "", err
	}

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
				return "", fmt.Errorf("unable to get messages: %s", err)
			}

			log.Println("Recieived message from thread: ", threadID)
			message, err := getFirstMessage(messageList)
			if err != nil {
				return "", fmt.Errorf("unable to get first message: %s", err)
			}
			log.Println("Received response from ai: ", message)
			return message, nil

		}

		if run.Status == openai.RunStatusRequiresAction {
			handleRequiresAction(dg, run, dgChannID, threadID, state, client)
		}

		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
}

func handleRequiresAction(
	dg *discordgo.Session,
	run openai.Run,
	dgChannID string,
	threadID string,
	state *State,
	client *openai.Client,
) (openai.Run, error) {

	// TODO: this used across multiple function could cause issues
	channelMsg := ChannelMessage{}

	funcArgs := GetFunctionArgs(run)
	toolOutputs := make([]openai.ToolOutput, len(funcArgs))

	sendChannelMessage := func() {
		log.Println("attempting to send message on: ", channelMsg.ChannelID)
		dg.ChannelMessageSend(channelMsg.ChannelID, channelMsg.Message)
		if channelMsg.IsReminder {
			state.threadMap[channelMsg.ChannelID].awaitsResponse = true
			go waitForReminderResponse(
				dg,
				channelMsg.ChannelID,
				channelMsg.UserID,
				client,
				state,
			)

		}
	}

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

			log.Println("Recieved channel message. sending after desired timeout")

			time.AfterFunc(
				time.Duration(channelMsg.TimerLength)*time.Second,
				sendChannelMessage,
			)
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

			err := json.Unmarshal([]byte(funcArg.JsonValue), &channelMsg)
			if err != nil {
				log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
				toolOutputs[i] = openai.ToolOutput{
					ToolCallID: funcArg.ToolID,
					Output:     "error deserializing data",
				}
			}

			time.AfterFunc(
				time.Duration(channelMsg.TimerLength)*time.Second,
				sendChannelMessage,
			)

			toolOutputs[i] = openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: "worked"}

		case ToggleMorningMessage:
			log.Println("toggle_morning_message()")
			morningMsgFuncArgs := MorningMsgFuncArgs{}
			err := json.Unmarshal([]byte(funcArg.JsonValue), &morningMsgFuncArgs)
			if err != nil {
				log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
				toolOutputs[i] = openai.ToolOutput{
					ToolCallID: funcArg.ToolID,
					Output:     "error deserializing data",
				}
				break
			}
			output := toggleMorningMsg(
				dg,
				morningMsgFuncArgs,
				dgChannID,
				client,
				state,
			)

			toolOutputs[i] = openai.ToolOutput{
				ToolCallID: funcArg.ToolID,
				Output:     output,
			}
		default:
			toolOutputs[i] = openai.ToolOutput{
				ToolCallID: funcArg.ToolID,
				Output:     "error. Pretend like you had a problem with submind",
			}
		}
	}
	return submitToolOutputs(client, toolOutputs, threadID, run.ID)

}

func waitForReminderResponse(
	dg *discordgo.Session,
	channelID string,
	userID string,
	client *openai.Client,
	state *State,
) {

	maxRemind := 5
	timeoutMin := 2 * time.Minute
	timer := time.NewTimer(timeoutMin)

	reminds := 0
	for {

		<-timer.C

		// this value is reset on messageCreate
		if !state.threadMap[channelID].awaitsResponse || reminds == maxRemind {
			timer.Stop()
			return
		}

		reminds++
		timeoutMin = timeoutMin * 2
		timer.Reset(timeoutMin)

		log.Println("sending another reminder")

		if userID == "" {
			userID = "they"
		}

		message := fmt.Sprintf(
			"It looks %s haven't responsed to this reminder can you generate a response nagging them about it. This is not a tool request.",
			mention(userID),
		)

		messageReq := openai.MessageRequest{
			Role:    openai.ChatMessageRoleAssistant,
			Content: message,
		}

		getAndSendResponse(context.Background(), dg, channelID, messageReq, client, state)
	}

}

func mention(userID string) string {
	return fmt.Sprint("<@%s>", userID)
}

func toggleMorningMsg(
	dg *discordgo.Session,
	morningMsgFuncArgs MorningMsgFuncArgs,
	channelID string,
	client *openai.Client,
	state *State,
) string {

	const timeFmt = "15:04"
	log.Println("Given time is: ", morningMsgFuncArgs.Time)
	givenTime, err := time.Parse(timeFmt, morningMsgFuncArgs.Time)
	if err != nil {
		log.Println("could not format time: ", err)
		return "could not format time"

	}
	now := time.Now()
	givenTime = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		givenTime.Hour(),
		givenTime.Minute(),
		0,
		0,
		now.Location(),
	)

	if givenTime.Before(now) {
		givenTime = givenTime.Add(24 * time.Hour)
	}

	duration := givenTime.Sub(now)
	log.Println("sending morning message in: ", duration)
	// TODO: add cancel
	ctx := context.Background()
	go startMorningMessageLoop(
		ctx,
		dg,
		morningMsgFuncArgs,
		duration,
		channelID,
		client,
		state)

	return "worked"
}

// TODO: add context with cancel
// TODO: group all timer calls into a common scheduler
func startMorningMessageLoop(
	ctx context.Context,
	dg *discordgo.Session,
	morningMsgFuncArgs MorningMsgFuncArgs,
	duration time.Duration,
	channelID string,
	client *openai.Client,
	state *State,
) {
	ticker := time.NewTicker(duration)

	for {
		<-ticker.C

		log.Println("ticker expired sending morning message ...")

		messageReq := openai.MessageRequest{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "Please tell @here goodmorning. this is not a function call",
		}

		ctx = context.WithValue(ctx, DisableFunctions, true)
		getAndSendResponse(ctx, dg, channelID, messageReq, client, state)

		if duration != 24*time.Hour {
			duration = 24 * time.Hour
			ticker.Reset(duration)
		}
	}
	// ticker.Stop()
}
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
