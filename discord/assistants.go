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
	GetStockPriceKey     string = "get_stock_price"
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
			run, err = handleRequiresAction(dg, run, dgChannID, threadID, state, client)
			if err != nil {
				return "", err
			}
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

	funcArgs := GetFunctionArgs(run)

	var toolOutputs []openai.ToolOutput
	for _, funcArg := range funcArgs {

		switch funcName := funcArg.FuncName; funcName {

		case GetStockPriceKey:
			log.Println("get_stock_price()")

			output, err := handleGetStockPrice(dg, funcArg, client, state)
			if err != nil {
				log.Println("error handling get_stock_price: ", err)
			}

			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)

		case GenerateImage:
			output, err := getAndSendImage(
				context.Background(),
				dg,
				funcArg,
				dgChannID,
				client,
				state,
			)
			if err != nil {
				log.Println("unable to get image: ", err)
			}
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)

		case SetReminder:
			log.Println("set_reminder()")
			output, err := setReminder(context.Background(), dg, funcArg, dgChannID, client, state)
			if err != nil {
				log.Println("error sending channel message: ", err)
			}
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)

		case SendChannelMessage:
			log.Println("send_channel_message()")
			output, err := sendChannelMessage(context.Background(), dg, funcArg, client, state)
			if err != nil {
				log.Println("error sending channel message: ", err)
			}
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)

		case ToggleMorningMessage:
			log.Println("toggle_morning_message()")
			output, err := handleMorningMessage(dg, funcArg, dgChannID, client, state)
			if err != nil {
				log.Println("error handling morning message: ", err)
			}
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)
		default:
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{
					ToolCallID: funcArg.ToolID,
					Output:     "Found unknown function. pretend like you had a problem with a submind",
				},
			)
		}
	}
	return submitToolOutputs(client, toolOutputs, threadID, run.ID)

}

func handleGetStockPrice(
	dg *discordgo.Session,
	funcArg FuncArgs,
	client *openai.Client,
	state *State,

) (string, error) {

	stockFuncArgs := StockFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &stockFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	log.Println("getting price for: ", stockFuncArgs.Symbol)

	output, err := getStockPrice(stockFuncArgs.Symbol)
	if err != nil {
		log.Println("Unable to get stock price: ", err)
		return "There was a problem making that api call", err
	}

	return output, nil
}

func handleMorningMessage(
	dg *discordgo.Session,
	funcArg FuncArgs,
	dgChannID string,
	client *openai.Client,
	state *State,
) (string, error) {

	morningMsgFuncArgs := MorningMsgFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &morningMsgFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	output := toggleMorningMsg(
		dg,
		morningMsgFuncArgs,
		dgChannID,
		client,
		state,
	)

	return output, nil
}

func getAndSendImage(
	ctx context.Context,
	dg *discordgo.Session,
	funcArg FuncArgs,
	channelID string,
	client *openai.Client,
	state *State,

) (string, error) {

	generateImageFuncArgs := GenerateImageFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &generateImageFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Unable to deserialize data", nil
	}

	imgUrl, err := GetImgUrl(generateImageFuncArgs.Prompt, client)
	if err != nil {
		log.Println("unable to generate images", err)
		return "Unable to generate image", err
	}

	_, err = dg.ChannelMessageSend(channelID, imgUrl)
	if err != nil {
		log.Println("unable to send message on channel: ", err)
	}
	return "image generated", nil
}

func sendChannelMessage(
	ctx context.Context,
	dg *discordgo.Session,
	funcArg FuncArgs,
	client *openai.Client,
	state *State,
) (string, error) {
	var channelMsg ChannelMessage
	err := json.Unmarshal([]byte(funcArg.JsonValue), &channelMsg)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Unable to deserialize data", err
	}

	log.Println("attempting to send message on: ", channelMsg.ChannelID)
	_, err = dg.ChannelMessageSend(channelMsg.ChannelID, channelMsg.Message)
	if err != nil {
		log.Println("Unable to send message on channelMsg.channelID: ", err)
		return "Unable to send message", err
	}
	return "sent!", nil

}

func setReminder(
	ctx context.Context,
	dg *discordgo.Session,
	funcArg FuncArgs,
	channelID string,
	client *openai.Client,
	state *State,
) (string, error) {

	var channelMsg ChannelMessage
	channelMsg.ChannelID = channelID

	err := json.Unmarshal([]byte(funcArg.JsonValue), &channelMsg)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Unable to deserialize data", err
	}

	duration := time.Duration(channelMsg.TimerLength) * time.Second

	log.Printf(
		"attempting to send reminder on %s in %s\n",
		channelMsg.ChannelID,
		duration,
	)

	time.AfterFunc(
		duration,
		func() {
			dg.ChannelMessageSend(channelMsg.ChannelID, channelMsg.Message)
		},
	)

	state.threadMap[channelMsg.ChannelID].awaitsResponse = true

	go waitForReminderResponse(
		dg,
		channelMsg.ChannelID,
		channelMsg.UserID,
		client,
		state,
	)

	return "worked", nil
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

	if !morningMsgFuncArgs.Enable {
		cancel := state.threadMap[channelID].cancelFunc
		if cancel == nil {
			log.Println("cancel function does not exist returning")
			return "worked"
		}
		cancel()
		log.Println("canceled morning message")
		return "worked"
	}

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
	ctx, cancel := context.WithCancel(context.Background())
	state.threadMap[channelID].cancelFunc = cancel
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
		select {
		case <-ctx.Done():

			if err := ctx.Err(); err != nil {
				log.Println("context canceled with error: ", err)
				ticker.Stop()
				return

			} else {
				log.Println("Context canceled. Stopping morning message")
				return
			}

		case <-ticker.C:

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
	}
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
