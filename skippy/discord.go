package skippy

import (
	"context"
	"log"

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

func OnMessageCreate(m *discordgo.MessageCreate, s *Skippy) {
	log.Printf("Recieved Message: %s\n", m.Content)

	thread, threadExists := s.State.GetThread(m.ChannelID)

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.DiscordSession.GetState().User.ID {
		return
	}

	if threadExists && thread.awaitsResponse {
		getAndSendResponse(
			context.Background(),
			s,
			ResponseReq{
				ChannelID: m.ChannelID,
				UserID:    m.Author.ID,
				Message:   m.Content,
			},
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
	// TODO: why did I have this here?
	message = replaceChannelIDs(message, m.MentionChannels)

	log.Println("using message: ", message)

	log.Println("CHANELLID: ", m.ChannelID)
	err := getAndSendResponse(context.Background(), s, ResponseReq{
		ChannelID: m.ChannelID,
		UserID:    m.Author.ID,
		Message:   message,
	})
	if err != nil {
		log.Println(err)
		return
	}
}

// Gets response from ai disables functions calls.
// Only capable of getting and sending a response
func getAndSendResponse(
	ctx context.Context,
	s *Skippy,
	req ResponseReq,
) error {
	log.Printf("Using message: %s\n Attempting to get response...", req.Message)

	s.DiscordSession.ChannelTyping(req.ChannelID)

	response, err := GetResponse(
		ctx,
		s,
		req,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = ERROR_RESPONSE
	}

	err = sendChunkedChannelMessage(s.DiscordSession, req.ChannelID, response)
	return err
}
