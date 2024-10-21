package skippy

import (
	"context"
	"log"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

// TODO: rename file
const (
	ERROR_RESPONSE   = "Oh no! Something went wrong."
	EVERYONE_MENTION = "@everyone"
)

func sendChunkedChannelMessage(
	dg DiscordSession,
	channelID string,
	message string,
) error {
	log.Printf("Sending message on %s: %s\n", channelID, message)
	// Discord has a limit of 2000 characters for a single message
	// If the message is longer than that, we need to split it into chunks
	for len(message) > 0 {
		if len(message) > 2000 {
			_, err := dg.ChannelMessageSend(channelID, message[:2000])
			if err != nil {
				log.Printf(
					"Could not send discord message on channel %s: %s\n",
					channelID,
					err,
				)
				return err
			}
			message = message[2000:]
		} else {
			dg.ChannelMessageSend(channelID, message)
			break
		}
	}
	return nil
}

func messageCreate(m *discordgo.MessageCreate, s *Skippy) {
	log.Printf("Recieved Message: %s\n", m.Content)

	thread, threadExists := s.State.GetThread(m.ChannelID)

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.DiscordSession.GetState().User.ID {
		return
	}

	if threadExists && thread.awaitsResponse {
		messageReq := openai.MessageRequest{
			Role:    openai.ChatMessageRoleUser,
			Content: m.Content,
		}
		getAndSendResponse(
			context.Background(),
			m.ChannelID,
			messageReq,
			DEFAULT_INSTRUCTIONS,
			s,
		)
		// value used by reminders to see if it needs to send another message to user
		s.State.SetAwaitsResponse(m.ChannelID, false, s.AIClient)
		s.Scheduler.CancelReminderJob(m.ChannelID)
	}

	role, roleMentioned := isRoleMentioned(s.DiscordSession, m)

	isMentioned := isMentioned(m.Mentions, s.DiscordSession.GetState().User) || roleMentioned
	alwaysRespond := threadExists && thread.alwaysRespond
	if !isMentioned && !alwaysRespond {
		return
	}

	message := removeBotMention(m.Content, s.DiscordSession.GetState().User.ID)
	message = removeRoleMention(message, role)
	// TODO: remove add to system message
	message = replaceChannelIDs(message, m.MentionChannels)
	message += "\n current time: "

	// TODO: put in instructions
	format := "Monday, Jan 02 at 03:04 PM"
	message += time.Now().Format(format)
	message += "\n User ID: " + m.Author.Mention()

	log.Println("using message: ", message)

	log.Println("CHANELLID: ", m.ChannelID)
	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	}
	getAndSendResponse(
		context.Background(),
		m.ChannelID,
		messageReq,
		DEFAULT_INSTRUCTIONS,
		s,
	)
}

// Gets response from ai disables functions calls.
// Only capable of getting and sending a response
func getAndSendResponseWithoutTools(
	ctx context.Context,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	s *Skippy,
) error {
	log.Printf("Using message: %s\n", messageReq.Content)

	log.Println("Attempting to get response...")

	response, err := GetResponse(
		ctx,
		dgChannID,
		messageReq,
		additionalInstructions,
		false,
		s,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = ERROR_RESPONSE
	}

	err = sendChunkedChannelMessage(s.DiscordSession, dgChannID, response)
	return err
}

func getAndSendResponse(
	ctx context.Context,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	s *Skippy,
) error {
	log.Printf("Using message: %s\n", messageReq.Content)

	log.Println("Attempting to get response...")

	response, err := GetResponse(
		ctx,
		dgChannID,
		messageReq,
		additionalInstructions,
		false,
		s,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = ERROR_RESPONSE
	}

	err = sendChunkedChannelMessage(s.DiscordSession, dgChannID, response)
	return err
}
