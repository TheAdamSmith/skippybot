package tests

import (
	"fmt"
	"testing"
	"time"

	"skippybot/skippy"

	"github.com/bwmarrin/discordgo"
)

// TODO test large message
func TestMessageCreateNoMention(t *testing.T) {
	t.Parallel()
	content := "test"
	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
		},
	}

	skippy.OnMessageCreate(msg, s)
	if len(dg.channelMessages[channelID]) > 0 {
		t.Error("Expected ChannelMessageSend to not be called")
	}
	if dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to not be called")
	}
}

func TestMessageCreateWithMention(t *testing.T) {
	t.Parallel()
	content := "test"
	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.OnMessageCreate(msg, s)
	if len(dg.channelMessages[channelID]) != 1 {
		t.Error("Expected ChannelMessageSend to be called")
	}
	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}
	if checkForErrorResponse(dg.channelMessages[channelID]) {
		t.Error("Expected message to not have error response")
	}
}

func TestSendMultipleMessages(t *testing.T) {
	t.Parallel()
	content := "test"
	channelID_1 := GenerateRandomID(10)
	channelID_2 := GenerateRandomID(10)
	go func() {
		for i := 0; i < 3; i++ {
			msg_1 := &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "1",
					ChannelID: channelID_1,
					Content:   content,
					Author: &discordgo.User{
						ID: "USER",
					},
					Mentions: []*discordgo.User{
						{
							ID: BOT_ID,
						},
					},
				},
			}

			msg_2 := &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "2",
					ChannelID: channelID_2,
					Content:   content,
					Author: &discordgo.User{
						ID: "USER",
					},
					Mentions: []*discordgo.User{
						{
							ID: BOT_ID,
						},
					},
				},
			}
			skippy.OnMessageCreate(msg_1, s)
			skippy.OnMessageCreate(msg_2, s)

		}
	}()
	timer := time.NewTimer(1 * time.Minute)
loop:
	for {
		select {
		case <-timer.C:
			t.Error("Expected ChannelMessageSend to be called 3 times")
			break loop
		default:
			if len(dg.channelMessages[channelID_1]) == 3 && len(dg.channelMessages[channelID_2]) == 3 {
				timer.Stop()
				break loop
			}
			time.Sleep(1 * time.Second)
		}
	}

	if !dg.channelTypingCalled[channelID_1] || !dg.channelTypingCalled[channelID_2] {
		t.Error("Expected ChannelTyping to be called")
	}

	if checkForErrorResponse(dg.channelMessages[channelID_1]) || checkForErrorResponse(dg.channelMessages[channelID_2]) {
		t.Error("Expected message to not have error response")
	}
}

func TestCreateReminder(t *testing.T) {
	t.Parallel()
	content := "Can you remind me 20 second to take out the trash"
	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.OnMessageCreate(msg, s)

	// wait for reminder
	timer := time.NewTimer(1 * time.Minute)
loop:
	for {
		select {
		case <-timer.C:
			t.Error("Expected ChannelMessageSend to be called 4 times")
			break loop
		default:
			// one for the response to the reminder, one for the reminder,
			// two for the follow up reminders
			if len(dg.channelMessages[channelID]) == 4 {
				timer.Stop()
				break loop
			}
			time.Sleep(1 * time.Second)
		}
	}

	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}

	// TODO: is this check needed
	// if !state.GetAwaitsResponse(channelID) {
	// 	t.Error("Expected thread to be awaiting response")
	// }
	// check that bot responds after a reminder with no mention
	msg = &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   "thanks",
			Author: &discordgo.User{
				ID: "USER",
			},
		},
	}

	skippy.OnMessageCreate(msg, s)
	if len(dg.channelMessages[channelID]) != 5 {
		t.Error("Expected ChannelMessageSend to be called again")
	}

	// TODO: check needed?
	// if state.GetAwaitsResponse(channelID) {
	// 	t.Error("Expected thread to not be awaiting response")
	// }

	if checkForErrorResponse(dg.channelMessages[channelID]) {
		t.Error("Expected message to not have error response")
	}
}

func TestToggleMorningMessage(t *testing.T) {
	t.Parallel()
	// the time for this needs to be longer than it take to make the call to
	// set the morning message
	content := fmt.Sprintf("can you set the morning message for %s. No stocks or weather", time.Now().Add(1*time.Minute).Format("03:04 PM"))

	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.OnMessageCreate(msg, s)

	// wait for reminder
	timer := time.NewTimer(2 * time.Minute)
loop:
	for {
		select {
		case <-timer.C:
			t.Error("Expected ChannelMessageSend to be called")
			break loop
		default:
			if len(dg.channelMessages[channelID]) == 2 {
				timer.Stop()
				break loop
			}
			time.Sleep(1 * time.Second)
		}
	}

	if len(dg.channelMessages[channelID]) != 2 {
		t.Fatal("Expected morning message to be sent")
	}

	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}

	if !scheduler.HasMorningMsgJob(channelID) {
		t.Error("Expected job to be scheduled")
	}

	content = "cancel the morning message"

	msg = &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.OnMessageCreate(msg, s)

	if len(dg.channelMessages[channelID]) != 3 {
		t.Error("Expected morning message to be canceled")
	}

	if scheduler.HasMorningMsgJob(channelID) {
		t.Error("Expected job to be canceled")
	}

	if checkForErrorResponse(dg.channelMessages[channelID]) {
		t.Error("Expected message to not have error response")
	}
}
