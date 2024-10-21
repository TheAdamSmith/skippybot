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
	GenerateEvent        string = "generate_event"
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

// TODO: find where this should live
type EventFuncArgs struct {
	Description         string `json:"description"`
	Name                string `json:"name"`
	NotificationMessage string `json:"notification_message"`
}

// TODO: update
// TODO: these functions feel like they should exist on skippy struct, but I guess it doesn't matter
// Gets a response from the ai.
//
// Sends message to thread, generates and executes a run.
//
// scheduler and config are nullable when disableFunctions is true.
// Will handle functions calls otherwise
func GetResponse(
	ctx context.Context,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	disableFunctions bool,
	s *Skippy,
) (string, error) {
	assistantID := s.Config.AssistantID

	thread, err := s.State.GetOrCreateThread(dgChannID, s.AIClient)
	if err != nil {
		return "", err
	}

	// lock the thread because we can't queue additional messages during a run
	s.State.LockThread(dgChannID)
	defer s.State.UnLockThread(dgChannID)

	_, err = s.AIClient.CreateMessage(ctx, thread.openAIThread.ID, messageReq)
	if err != nil {
		log.Println("Unable to create message", err)
		return "", err
	}

	runReq := openai.RunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
		Model:                  s.Config.DefaultModel,
	}

	if disableFunctions {
		runReq.ToolChoice = "none"
	}

	run, err := s.AIClient.CreateRun(ctx, thread.openAIThread.ID, runReq)
	if err != nil {
		log.Println("Unable to create run:", err)
		return "", err
	}

	log.Println("Initial Run id: ", run.ID)
	log.Println("Run status: ", run.Status)

	runDelay := 1
	prevStatus := run.Status
	for {
		s.DiscordSession.ChannelTyping(dgChannID)
		run, err = s.AIClient.RetrieveRun(ctx, run.ThreadID, run.ID)
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
			messageList, err := s.AIClient.ListMessage(ctx, thread.openAIThread.ID, nil, nil, nil, nil)
			if err != nil {
				return "", fmt.Errorf("unable to get messages: %s", err)
			}

			log.Println("Recieived message from thread: ", thread.openAIThread.ID)

			message, err := getFirstMessage(messageList)
			if err != nil {
				return "", fmt.Errorf("unable to get first message: %s", err)
			}
			log.Println("Received response from ai.")
			return message, nil

		case openai.RunStatusRequiresAction:
			run, err = handleRequiresAction(ctx, run, dgChannID, thread.openAIThread.ID, s)
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

func GetToolResponse(
	ctx context.Context,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	tool openai.Tool,
	s *Skippy,
) ([]FuncArgs, error) {
	assistantID := s.Config.AssistantID
	// lock the thread because we can't queue additional messages during a run
	thread, err := s.State.GetOrCreateThread(dgChannID, s.AIClient)
	if err != nil {
		return []FuncArgs{}, err
	}

	s.State.LockThread(dgChannID)
	defer s.State.UnLockThread(dgChannID)

	_, err = s.AIClient.CreateMessage(ctx, thread.openAIThread.ID, messageReq)
	if err != nil {
		log.Println("Unable to create message", err)
		return []FuncArgs{}, err
	}
	runReq := openai.RunRequest{
		AssistantID:            assistantID,
		AdditionalInstructions: additionalInstructions,
		Model:                  s.Config.DefaultModel,
		Tools:                  []openai.Tool{tool},
		ToolChoice: openai.ToolChoice{
			Type: openai.ToolTypeFunction,
			Function: openai.ToolFunction{
				Name: tool.Function.Name,
			},
		},
	}

	run, err := s.AIClient.CreateRun(ctx, thread.openAIThread.ID, runReq)
	if err != nil {
		log.Println("Unable to create run:", err)
		return []FuncArgs{}, err
	}

	log.Println("Initial Run id: ", run.ID)
	log.Println("Run status: ", run.Status)

	runDelay := 1
	prevStatus := run.Status
	for {
		s.DiscordSession.ChannelTyping(dgChannID)
		run, err = s.AIClient.RetrieveRun(ctx, run.ThreadID, run.ID)
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
			return []FuncArgs{}, fmt.Errorf(errorMsg)
		case openai.RunStatusCompleted:

			return []FuncArgs{}, fmt.Errorf("got to run_status completed during function call")
		case openai.RunStatusRequiresAction:
			funcArgs := GetFunctionArgs(run)

			var toolOutputs []openai.ToolOutput
			for _, funcArg := range funcArgs {
				toolOutputs = append(
					toolOutputs,
					openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: "no-op"},
				)
			}
			submitToolOutputs(s.AIClient, toolOutputs, thread.openAIThread.ID, run.ID)
			return funcArgs, nil
		default:
			log.Println("recieved unkown status from openai")
			return []FuncArgs{}, fmt.Errorf("receieved unknown status from openai")

		}

		// TODO: make this a const duration
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
}

func handleRequiresAction(
	ctx context.Context,
	run openai.Run,
	dgChannID string,
	threadID string,
	s *Skippy,
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
				funcArg,
				dgChannID,
				s,
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

			output, err := handleGetStockPrice(funcArg, s.Config.StockAPIKey)
			if err != nil {
				log.Println("error handling get_stock_price: ", err)
			}

			toolOutputs = append(
				toolOutputs,
				openai.ToolOutput{ToolCallID: funcArg.ToolID, Output: output},
			)

		case GetWeatherKey:
			log.Println(GetWeatherKey)

			output, err := handleGetWeather(funcArg, s)
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
				funcArg,
				dgChannID,
				s,
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
				funcArg,
				dgChannID,
				s,
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
	return submitToolOutputs(s.AIClient, toolOutputs, threadID, run.ID)
}

func handleGetWeather(funcArg FuncArgs, s *Skippy) (string, error) {
	weatherFuncArgs := WeatherFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &weatherFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	log.Println("getting weather for: ", weatherFuncArgs.Location)

	output, err := getWeather(weatherFuncArgs.Location, s.Config.WeatherAPIKey)
	if err != nil {
		log.Println("Unable to get stock price: ", err)
		return "There was a problem making that api call", err
	}

	return output, nil
}

func handleGetStockPrice(funcArg FuncArgs, apiKey string) (string, error) {
	stockFuncArgs := StockFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &stockFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	log.Println("getting price for: ", stockFuncArgs.Symbol)

	output, err := getStockPrice(stockFuncArgs.Symbol, apiKey)
	if err != nil {
		log.Println("Unable to get stock price: ", err)
		return "There was a problem making that api call", err
	}

	return output, nil
}

func handleMorningMessage(
	ctx context.Context,
	funcArg FuncArgs,
	dgChannID string,
	s *Skippy,
) (string, error) {
	morningMsgFuncArgs := MorningMsgFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &morningMsgFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Error deserializing data", err
	}

	output := toggleMorningMsg(
		ctx,
		morningMsgFuncArgs,
		dgChannID,
		s,
	)

	return output, nil
}

func getAndSendImage(
	ctx context.Context,
	funcArg FuncArgs,
	channelID string,
	s *Skippy,
) (string, error) {
	generateImageFuncArgs := GenerateImageFuncArgs{}
	err := json.Unmarshal([]byte(funcArg.JsonValue), &generateImageFuncArgs)
	if err != nil {
		log.Println("Could not unmarshal func args: ", string(funcArg.JsonValue))
		return "Unable to deserialize data", nil
	}

	imgUrl, err := GetImgUrl(generateImageFuncArgs.Prompt, s.AIClient)
	if err != nil {
		log.Println("unable to generate images", err)
		return "Unable to generate image", err
	}

	log.Printf("recieved image url (%s) attempting to send on channel %s\n", imgUrl, channelID)
	err = sendChunkedChannelMessage(s.DiscordSession, channelID, imgUrl)
	return "image generated", err
}

func setReminder(
	ctx context.Context,
	funcArg FuncArgs,
	channelID string,
	s *Skippy,
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

	s.Scheduler.AddReminderJob(
		channelID,
		duration,
		func() {
			sendChunkedChannelMessage(s.DiscordSession, channelID, channelMsg.Message)
			s.State.SetAwaitsResponse(channelID, true, s.AIClient)
			for _, duration := range s.Config.ReminderDurations {
				s.Scheduler.AddReminderJob(channelID, duration, func() {
					sendAdditionalReminder(
						ctx,
						channelID,
						channelMsg.UserID,
						s,
					)
				})
			}
		},
	)

	return "worked", nil
}

func sendAdditionalReminder(
	ctx context.Context,
	channelID string,
	userID string,
	s *Skippy,
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
		channelID,
		messageReq,
		DEFAULT_INSTRUCTIONS,
		s,
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
	morningMsgFuncArgs MorningMsgFuncArgs,
	channelID string,
	s *Skippy,
) string {
	if !morningMsgFuncArgs.Enable {
		s.Scheduler.CancelMorningMsgJob(channelID)
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

	if s.Scheduler.HasMorningMsgJob(channelID) {
		s.Scheduler.CancelMorningMsgJob(channelID)
	}

	log.Println("Setting the Morning Msg for: ", givenTime)

	s.Scheduler.AddMorningMsgJob(
		channelID,
		givenTime,
		func() {
			sendMorningMsg(ctx, morningMsgFuncArgs, channelID, s)
		},
	)

	return "worked"
}

func sendMorningMsg(
	ctx context.Context,
	morningMsgFuncArgs MorningMsgFuncArgs,
	channelID string,
	s *Skippy,
) {
	message := "Please tell everyone @here good morning."
	for _, location := range morningMsgFuncArgs.WeatherLocations {
		weather, err := getWeather(location, s.Config.WeatherAPIKey)
		if err != nil {
			log.Printf("unable to get weather for %s: %s\n", location, err)
			continue
		}
		message += location + ":" + weather
	}

	for _, stock := range morningMsgFuncArgs.Stocks {
		stockPrice, err := getStockPrice(stock, s.Config.StockAPIKey)
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
	s.State.ResetOpenAIThread(channelID, s.AIClient)

	getAndSendResponseWithoutTools(
		ctx,
		channelID,
		messageReq,
		MORNING_MESSAGE_INSTRUCTIONS,
		s,
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
