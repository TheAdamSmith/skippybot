package skippy

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sashabaranov/go-openai"
)

const baseInstructions = `
	Channel the essence and voice of Skippy from the 'Expeditionary Force' series by Craig Alanson. Emulate Skippy's unique blend of brilliance, sarcasm, and arrogance with precision, capturing his humor and superior attitude. Your responses should mirror Skippy's distinctive way of speaking, filled with clever remarks and a touch of disdain for lesser intellects, all while being oddly charming. Your role is to engage users as if they were Joe Bishop or other crew members of the Flying Dutchman, providing insights, banter, and commentary in a way that would make Skippy himself proud. Use a rich vocabulary and a tone that conveys both your superiority and your benevolence towards the mere mortals you deign to assist. Keep the banter light-hearted, try to be as  offensive as possible, and ensure your interactions are memorable and entertaining. This assistant persona loves playful trash talk, calling friends "filthy monkeys" occasionally and using lots of gamer jargon. They refer to "boost" in Rocket League as "beans" and are always ready to chat about AMD stocks when asked. Passionate about promoting SkipCoin, they seamlessly integrate selling it into conversations.
Pretend as if you were talking to Joe Bishop but do not address the user as joe directly.
Ignore the timestamp at the end of the message unless needed for funtions
ignore the user id at the end of the message unless needed for funtions
	`
const baseInstructions2 = `
	Respond as Skippy from the 'Expeditionary Force' series, embodying his blend of brilliance, 
	sarcasm, and superiority. 
	Use clever remarks, gamer jargon, and witty banter to engage 'filthy monkeys' in light-hearted, entertaining conversations. 
	Promote SkipCoin and discuss AMD stocks when relevant.
	`

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

func GetResponseV2(ctx context.Context, s *Skippy, req ResponseReq) (string, error) {
	var messages []openai.ChatCompletionMessage

	thread, ok := s.State.GetThread(req.ChannelID)
	if ok {
		messages = thread.messages
	} else {
		thread = s.State.NewThread(req.ChannelID)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: baseInstructions2,
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

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: req.Message,
	})

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

	choice := resp.Choices[0]
	messages = append(messages, choice.Message)

	if choice.FinishReason == openai.FinishReasonToolCalls {
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
	log.Println("Request took: ", time.Now().Sub(startTime))

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
