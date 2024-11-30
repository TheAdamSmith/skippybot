package skippy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	ToggleMorningMessage string = "set_morning_message"
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

type EventFuncArgs struct {
	Description         string `json:"description"`
	Name                string `json:"name"`
	NotificationMessage string `json:"notification_message"`
}

func GetToolOutputs(
	ctx context.Context,
	toolCalls []openai.ToolCall,
	dgChannID string,
	s *Skippy,
) []openai.ChatCompletionMessage {
	var toolOutputs []openai.ChatCompletionMessage
outerloop:
	for _, toolCall := range toolCalls {
		funcArg := FuncArgs{
			ToolID:    toolCall.ID,
			FuncName:  toolCall.Function.Name,
			JsonValue: toolCall.Function.Arguments,
		}

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
			toolOutputs = makeNoOpToolMessage(toolCalls, funcArg.ToolID, output)
			break outerloop
		case GetStockPriceKey:
			log.Println("get_stock_price()")

			output, err := handleGetStockPrice(funcArg, s.Config.StockAPIKey)
			if err != nil {
				log.Println("error handling get_stock_price: ", err)
			}

			toolOutputs = append(
				toolOutputs,
				openai.ChatCompletionMessage{ToolCallID: funcArg.ToolID, Content: output, Role: openai.ChatMessageRoleTool},
			)

		case GetWeatherKey:
			log.Println(GetWeatherKey)

			output, err := handleGetWeather(funcArg, s)
			if err != nil {
				log.Println("error handling get_weather: ", err)
			}

			toolOutputs = append(
				toolOutputs,
				openai.ChatCompletionMessage{ToolCallID: funcArg.ToolID, Content: output, Role: openai.ChatMessageRoleTool},
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
				openai.ChatCompletionMessage{ToolCallID: funcArg.ToolID, Content: output, Role: openai.ChatMessageRoleTool},
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
				openai.ChatCompletionMessage{ToolCallID: funcArg.ToolID, Content: output, Role: openai.ChatMessageRoleTool},
			)
		default:
			toolOutputs = append(
				toolOutputs,
				openai.ChatCompletionMessage{ToolCallID: funcArg.ToolID, Content: "unkown tool used", Role: openai.ChatMessageRoleTool},
			)
		}
	}

	return toolOutputs
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

	log.Println("Given time is: ", morningMsgFuncArgs.Time)
	givenTime, err := ParseCommonTime(morningMsgFuncArgs.Time)
	if err != nil {
		return "could not format time"
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

	getAndSendResponse(
		ctx,
		s,
		ResponseReq{
			ChannelID:    channelID,
			UserID:       userID,
			Message:      message,
			DisableTools: true,
		},
	)
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

	// TODO: remove
	// reset the thread every morning
	s.State.NewThread(channelID)

	getAndSendResponse(
		ctx,
		s,
		ResponseReq{
			ChannelID:              channelID,
			Message:                message,
			AdditionalInstructions: MORNING_MESSAGE_INSTRUCTIONS,
			DisableTools: true,
		},
	)
}

// Creates a list of no-op tool outputs except for the provided toolID and output
func makeNoOpToolMessage(tools []openai.ToolCall, toolID string, output string) []openai.ChatCompletionMessage {
	var toolOutputs []openai.ChatCompletionMessage
	for _, tool := range tools {
		if tool.ID == toolID {
			toolOutputs = append(
				toolOutputs,
				openai.ChatCompletionMessage{ToolCallID: toolID, Content: output, Role: openai.ChatMessageRoleTool},
			)
		} else {
			toolOutputs = append(
				toolOutputs,
				openai.ChatCompletionMessage{ToolCallID: tool.ID, Content: "no-op", Role: openai.ChatMessageRoleTool},
			)
		}
	}

	return toolOutputs
}
