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

// TODO: add tools and additional system messages
// TODO: user id
func GetResponseV2(ctx context.Context, dgChannID string, userID string, message string, s *Skippy) (string, error) {
	var messages []openai.ChatCompletionMessage

	thread, ok := s.State.GetThread(dgChannID)
	if ok {
		messages = thread.messages
	} else {
		s.State.NewThread(dgChannID)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: baseInstructions,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})

	req := openai.ChatCompletionRequest{
		Model:    s.Config.DefaultModel,
		Messages: addTimeAndUserID(messages, userID),
	}

	startTime := time.Now()
	resp, err := s.AIClient.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Println("error getting response from ai", err)
		return "", err
	}
	log.Println("Request took: ", time.Now().Sub(startTime))

	respMessage := resp.Choices[0].Message
	messages = append(messages, respMessage)

	s.State.SetThreadMessages(dgChannID, messages)

	return respMessage.Content, nil
}

// adds the current user id and the timetamp to the message list
func addTimeAndUserID(messages []openai.ChatCompletionMessage, userID string) []openai.ChatCompletionMessage {
	format := "Monday, Jan 02 at 03:04 PM"
	currTime := time.Now().Format(format)

	return append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: fmt.Sprintf("Current User: %s, Current time: %s", UserMention(userID), currTime),
	})
}
