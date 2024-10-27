package skippy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

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
				log.Println("error handling get_weather: ", err)
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
