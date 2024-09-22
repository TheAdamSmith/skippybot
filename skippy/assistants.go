package skippy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// ai functions
	GetStockPriceKey     string = "get_stock_price"
	GetWeatherKey        string = "get_weather"
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

type WeatherFuncArgs struct {
	Location string
}

type GenerateImageFuncArgs struct {
	Prompt string `json:"prompt"`
}

type MorningMsgFuncArgs struct {
	Enable           bool     `json:"enable"`
	Time             string   `json:"time"`
	WeatherLocations []string `json:"weather_locations,omitempty"`
	Stocks           []string `json:"stocks,omitempty"`
}

// used for
type ReminderFuncArgs struct {
	Message     string `json:"message"`
	TimerLength int    `json:"timer_length,omitempty"`
	UserID      string `json:"user_id,omitempty"`
}

// Gets a response from the ai.
//
// Sends message to thread, generates and executes a run.
//
// scheduler and config are nullable when disableFunctions is true.
// Will handle functions calls otherwise
func GetResponse(
	ctx context.Context,
	dg DiscordSession,
	threadID string,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	disableFunctions bool,
	client *openai.Client,
	state *State,
	// nullable if disableFunctions is true
	scheduler *Scheduler,
	// nullable if disableFunctions is true
	config *Config,
) (string, error) {
	assistantID := state.GetAssistantID()

	_, err := client.CreateMessage(ctx, threadID, messageReq)
	if err != nil {
		log.Println("Unable to create message", err)
		return "", err
	}

	runReq := openai.RunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
		Model:                  state.openAIModel,
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
	prevStatus := run.Status
	for {
		dg.ChannelTyping(dgChannID)
		run, err = client.RetrieveRun(ctx, run.ThreadID, run.ID)
		if err != nil {
			log.Println("error retrieving run: ", err)
		}

		if prevStatus != run.Status {
			log.Printf("Run status: %s\n", run.Status)
			prevStatus = run.Status
		}

		switch run.Status {
		case openai.RunStatusInProgress, openai.RunStatusQueued:
			continue
		case openai.RunStatusFailed:
			errorMsg := fmt.Sprintf(
				"openai run failed with code code (%s): %s",
				run.LastError.Code,
				run.LastError.Message,
			)
			log.Println(errorMsg)
			return "", fmt.Errorf(errorMsg)
		case openai.RunStatusCompleted:
			log.Println("Usage: ", run.Usage.TotalTokens)
			messageList, err := client.ListMessage(ctx, threadID, nil, nil, nil, nil)
			if err != nil {
				return "", fmt.Errorf("unable to get messages: %s", err)
			}

			log.Println("Recieived message from thread: ", threadID)

			message, err := getFirstMessage(messageList)
			if err != nil {
				return "", fmt.Errorf("unable to get first message: %s", err)
			}
			log.Println("Received response from ai.")
			return message, nil

		case openai.RunStatusRequiresAction:
			if disableFunctions || config == nil || scheduler == nil {
				return "", fmt.Errorf("recieved required action when tools disabled")
			}
			run, err = handleRequiresAction(ctx, dg, run, dgChannID, threadID, client, state, scheduler, config)
			if err != nil {
				return "", err
			}
		default:
			log.Println("recieved unkown status from openai")
			return "", fmt.Errorf("receieved unknown status from openai")

		}

		// TODO: make this a const duration
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
}

func handleRequiresAction(
	ctx context.Context,
	dg DiscordSession,
	run openai.Run,
	dgChannID string,
	threadID string,
	client *openai.Client,
	state *State,
	scheduler *Scheduler,
	config *Config,
) (openai.Run, error) {
	funcArgs := GetFunctionArgs(run)

	var toolOutputs []openai.ToolOutput
outerloop:
	for _, funcArg := range funcArgs {

		log.Printf("recieved function request:%+v", funcArg)
		switch funcName := funcArg.FuncName; funcName {
		case ToggleMorningMessage:
			log.Println("toggle_morning_message()")
			output, err := handleMorningMessage(
				ctx,
				dg,
				funcArg,
				dgChannID,
				client,
				state,
				scheduler,
			)
			if err != nil {
				log.Println("error handling morning message: ", err)
			}
			// the bot will confuse multiple functions calls with this one so
			// we only want to set the morning message if it is called
			toolOutputs = makeNoOpToolOutputs(funcArgs, funcArg.ToolID, output)
			break outerloop
		case GetStockPriceKey:
			log.Println("get_stock_price()")

			output, err := handleGetStockPrice(funcArg, state)
			if err != nil {
				log.Println("error handling get_stock_price: ", err)
			}

			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)

		case GetWeatherKey:
			log.Println(GetWeatherKey)

			output, err := handleGetWeather(funcArg, state)
			if err != nil {
				log.Println("error handling get_stock_price: ", err)
			}

			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)
		case GenerateImage:
			log.Println(GenerateImage)
			output, err := getAndSendImage(
				context.Background(),
				dg,
				funcArg,
				dgChannID,
				client,
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
			output, err := setReminder(
				context.Background(),
				dg,
				funcArg,
				dgChannID,
				client,
				state,
				scheduler,
				config,
			)
			if err != nil {
				log.Println("error sending channel message: ", err)
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

func handleGetWeather(
	funcArg FuncArgs,
	state *State,
) (string, error) {
	weatherFuncArgs := WeatherFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &weatherFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	log.Println("getting weather for: ", weatherFuncArgs.Location)

	output, err := getWeather(weatherFuncArgs.Location, state.GetWeatherAPIKey())
	if err != nil {
		log.Println("Unable to get stock price: ", err)
		return "There was a problem making that api call", err
	}

	return output, nil
}

func handleGetStockPrice(
	funcArg FuncArgs,
	state *State,
) (string, error) {
	stockFuncArgs := StockFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &stockFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	log.Println("getting price for: ", stockFuncArgs.Symbol)

	output, err := getStockPrice(stockFuncArgs.Symbol, state.GetStockAPIKey())
	if err != nil {
		log.Println("Unable to get stock price: ", err)
		return "There was a problem making that api call", err
	}

	return output, nil
}

func handleMorningMessage(
	ctx context.Context,
	dg DiscordSession,
	funcArg FuncArgs,
	dgChannID string,
	client *openai.Client,
	state *State,
	scheduler *Scheduler,
) (string, error) {
	morningMsgFuncArgs := MorningMsgFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &morningMsgFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	output := toggleMorningMsg(
		ctx,
		dg,
		morningMsgFuncArgs,
		dgChannID,
		client,
		state,
		scheduler,
	)

	return output, nil
}

func getAndSendImage(
	ctx context.Context,
	dg DiscordSession,
	funcArg FuncArgs,
	channelID string,
	client *openai.Client,
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

	log.Printf("recieved image url (%s) attempting to send on channel %s\n", imgUrl, channelID)
	err = sendChunkedChannelMessage(dg, channelID, imgUrl)
	return "image generated", err
}

func setReminder(
	ctx context.Context,
	dg DiscordSession,
	funcArg FuncArgs,
	channelID string,
	client *openai.Client,
	state *State,
	scheduler *Scheduler,
	config *Config,
) (string, error) {
	var channelMsg ReminderFuncArgs

	err := json.Unmarshal([]byte(funcArg.JsonValue), &channelMsg)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Unable to deserialize data", err
	}

	duration := time.Duration(channelMsg.TimerLength) * time.Second

	log.Printf(
		"attempting to send reminder on %s in %s\n",
		channelID,
		duration,
	)

	scheduler.AddReminderJob(
		channelID,
		duration,
		func() {
			sendChunkedChannelMessage(dg, channelID, channelMsg.Message)
			state.SetAwaitsResponse(channelID, true, client)
			for _, duration := range config.ReminderDurations {
				scheduler.AddReminderJob(channelID, duration, func() {
					sendAdditionalReminder(
						ctx,
						dg,
						channelID,
						channelMsg.UserID,
						client,
						state,
					)
				})
			}
		},
	)

	return "worked", nil
}

func sendAdditionalReminder(
	ctx context.Context,
	dg DiscordSession,
	channelID string,
	userID string,
	client *openai.Client,
	state *State,
) {
	log.Println("sending another reminder")

	// TODO: they should not get mentioned
	if userID == "" {
		userID = "they"
	}

	// TODO: maybe add this to additional_instruction
	message := fmt.Sprintf(
		"It looks %s haven't responsed to this reminder can you generate a response nagging them about it. This is not a tool request.",
		UserMention(userID),
	)

	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleAssistant,
		Content: message,
	}

	getAndSendResponseWithoutTools(
		ctx,
		dg,
		channelID,
		messageReq,
		DEFAULT_INSTRUCTIONS,
		client,
		state,
	)
}

func UserMention(userID string) string {
	if strings.HasPrefix(userID, "<@") && strings.HasSuffix(userID, ">") {
		return userID
	}
	return fmt.Sprintf("<@%s>", userID)
}

func toggleMorningMsg(
	ctx context.Context,
	dg DiscordSession,
	morningMsgFuncArgs MorningMsgFuncArgs,
	channelID string,
	client *openai.Client,
	state *State,
	scheduler *Scheduler,
) string {
	if !morningMsgFuncArgs.Enable {
		scheduler.CancelMorningMsgJob(channelID)
		return "worked"
	}

	// TODO: this is hacky
	// this should be fixed in the ai function definition
	timeFmt := "15:04"
	log.Println("Given time is: ", morningMsgFuncArgs.Time)
	givenTime, err := time.Parse(timeFmt, morningMsgFuncArgs.Time)
	if err != nil {
		log.Printf("could not format time: %s. attempting to use AM/PM format", err)
		timeFmt := "3:04 PM"
		givenTime, err = time.Parse(timeFmt, morningMsgFuncArgs.Time)
		if err != nil {
			log.Printf("could not format time: %s.", err)
			return "could not format time"
		}

	}

	if scheduler.HasMorningMsgJob(channelID) {
		scheduler.CancelMorningMsgJob(channelID)
	}

	log.Println("Setting the Morning Msg for: ", givenTime)
	scheduler.AddMorningMsgJob(
		channelID,
		givenTime,
		func() {
			sendMorningMsg(ctx, dg, morningMsgFuncArgs, channelID, client, state)
		},
	)

	return "worked"
}

func sendMorningMsg(
	ctx context.Context,
	dg DiscordSession,
	morningMsgFuncArgs MorningMsgFuncArgs,
	channelID string,
	client *openai.Client,
	state *State,
) {
	message := "Please tell everyone @here good morning."
	for _, location := range morningMsgFuncArgs.WeatherLocations {
		weather, err := getWeather(location, state.GetWeatherAPIKey())
		if err != nil {
			log.Printf("unable to get weather for %s: %s\n", location, err)
			continue
		}
		message += location + ":" + weather
	}

	for _, stock := range morningMsgFuncArgs.Stocks {
		stockPrice, err := getStockPrice(stock, state.GetStockAPIKey())
		if err != nil {
			log.Printf("unable to get weather for %s: %s\n", stock, err)
			continue
		}
		message += stock + ":" + stockPrice
	}

	log.Println("getting morning message with prompt: ", message)

	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleAssistant,
		Content: message,
	}

	// reset the thread every morning
	state.ResetOpenAIThread(channelID, client)

	getAndSendResponseWithoutTools(
		ctx,
		dg,
		channelID,
		messageReq,
		MORNING_MESSAGE_INSTRUCTIONS,
		client,
		state,
	)
}

func submitToolOutputs(
	client *openai.Client,
	toolOutputs []openai.ToolOutput,
	threadID string,
	runID string,
) (run openai.Run, err error) {
	req := openai.SubmitToolOutputsRequest{
		ToolOutputs: toolOutputs,
	}
	return client.SubmitToolOutputs(context.Background(), threadID, runID, req)
}

// Creates a list of no-op tool outputs except for the provided toolID and output
func makeNoOpToolOutputs(funcArgs []FuncArgs, toolID string, output string) []openai.ToolOutput {
	var toolOutputs []openai.ToolOutput
	for _, funcArg := range funcArgs {
		if funcArg.ToolID == toolID {
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: toolID, Output: output},
			)
		} else {
			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: "no-op"},
			)
		}
	}
	return toolOutputs
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
