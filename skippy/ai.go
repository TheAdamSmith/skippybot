package skippy

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sashabaranov/go-openai"
)

const (
	TOOL_CHOICE_AUTO     = "auto"
	TOOL_CHOICE_NONE     = "none"
	TOOL_CHOICE_REQUIRED = "required"
)

type ResponseReq struct {
	ChannelID              string
	UserID                 string
	Message                string
	Tools                  []openai.Tool
	AdditionalInstructions string
	DisableTools           bool
	RequireTools           bool
	// TODO: this behavior seems like it could cause issues. either refactor to use a specicific method or test thoroughly
	ReturnToolOutput bool
}

type ToolChoice interface {
	string | openai.ToolChoice
}

func GetResponse(ctx context.Context, s *Skippy, req ResponseReq) (string, error) {
	var messages []openai.ChatCompletionMessage

	thread, ok := s.State.GetThread(req.ChannelID)
	if ok {
		messages = thread.messages
	} else {
		thread = s.State.NewThread(req.ChannelID)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: s.Config.BaseInstructions,
		})
	}
	thread.Lock()
	defer thread.Unlock()

	if req.AdditionalInstructions != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.AdditionalInstructions,
		})
	}

	if req.Tools == nil {
		req.Tools = ALL_TOOLS
	}

	var toolChoice any = TOOL_CHOICE_AUTO
	if req.DisableTools {
		toolChoice = TOOL_CHOICE_NONE
	} else if req.RequireTools {
		if len(req.Tools) == 1 {
			toolChoice = openai.ToolChoice{
				Type: openai.ToolTypeFunction,
				Function: openai.ToolFunction{
					Name: req.Tools[0].Function.Name,
				},
			}
		} else {
			toolChoice = TOOL_CHOICE_REQUIRED
		}
	}

	if req.Message != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: req.Message,
		})
	}

	completionReq := openai.ChatCompletionRequest{
		ToolChoice: toolChoice,
		Model:      s.Config.DefaultModel,
		Messages:   addTimeAndUserID(messages, req.UserID),
		Tools:      req.Tools,
	}

	resp, err := makeRequest(ctx, completionReq, s)
	if err != nil {
		log.Println("error getting response from ai", err)
		return "", err
	}

	log.Println("tokens used: ", resp.Usage.TotalTokens)

	choice := resp.Choices[0]

	messages = append(messages, choice.Message)

	// if prompted only with a system message and tool calls are required the model is returning with 
	// stopped while attempting to call a function
	if choice.FinishReason == openai.FinishReasonToolCalls || (req.RequireTools && choice.FinishReason == openai.FinishReasonStop) {
		log.Println("Recieved tool call")
		if req.ReturnToolOutput {
			return choice.Message.ToolCalls[0].Function.Arguments, nil
		}

		toolOutputs := GetToolOutputs(ctx, choice.Message.ToolCalls, req.ChannelID, s)
		messages = append(messages, toolOutputs...)

		completionReq.Messages = addTimeAndUserID(messages, req.ChannelID)

		resp, err := makeRequest(ctx, completionReq, s)
		if err != nil {
			log.Println("error getting response from ai", err)
			return "", err
		}

		choice = resp.Choices[0]
		messages = append(messages, choice.Message)
	}

	s.State.SetThreadMessages(req.ChannelID, messages)

	return choice.Message.Content, nil
}

func makeRequest(ctx context.Context, req openai.ChatCompletionRequest, s *Skippy) (openai.ChatCompletionResponse, error) {
	startTime := time.Now()
	resp, err := s.AIClient.CreateChatCompletion(ctx, req)
	log.Println("Request took: ", time.Since(startTime))

	return resp, err
}

// adds the current user id and the timestamp to the message list
func addTimeAndUserID(messages []openai.ChatCompletionMessage, userID string) []openai.ChatCompletionMessage {
	format := "Monday, Jan 02 at 03:04 PM"
	currTime := time.Now().Format(format)
	content := fmt.Sprintf("Current time: %s", currTime)
	if userID != "" {
		content += fmt.Sprintf(", Current User: %s", UserMention(userID))
	}

	return append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: content,
	})
}
