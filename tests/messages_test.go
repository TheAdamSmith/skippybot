package tests

import (
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

	skippy.MessageCreate(dg, msg, client, state, config)
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
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.MessageCreate(dg, msg, client, state, config)
	if len(dg.channelMessages[channelID]) != 1 {
		t.Error("Expected ChannelMessageSend to be called")
	}
	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}
}

// TODO: need to test the reminder backoff
// but need to be able to set up testing config first
func TestCreateReminder(t *testing.T) {
	t.Parallel()
	content := "Can you remind me 1 second to take out the trash"
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
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.MessageCreate(dg, msg, client, state, config)

	// wait for reminder
	time.Sleep(2 * time.Second)
	if len(dg.channelMessages[channelID]) != 4 {
		t.Error("Expected ChannelMessageSend to be called twice")
	}

	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}

	if !state.GetAwaitsResponse(channelID) {
		t.Error("Expected thread to be awaiting response")
	}
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

	skippy.MessageCreate(dg, msg, client, state, config)
	if len(dg.channelMessages[channelID]) != 5 {
		t.Error("Expected ChannelMessageSend to be called again")
	}

	if state.GetAwaitsResponse(channelID) {
		t.Error("Expected thread to not be awaiting response")
	}
}

func TestToggleMorningMessage(t *testing.T) {
	t.Parallel()
	// the time for this needs to be longer than it take to make the call to
	// set the morning message
	content :=
		"can you toggle the morning message for 1 minute from now"
	// fmt.Sprintf(
	// 	time.Now().
	// 		Add(1*time.Minute).
	// 		Format("15:04"))

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
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.MessageCreate(dg, msg, client, state, config)

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

	if _, exists := state.GetCancelFunc(channelID); !exists {
		t.Fatal("Expected thread to have a cancelFunc")
	}

	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
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
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	skippy.MessageCreate(dg, msg, client, state, config)

	if len(dg.channelMessages[channelID]) != 3 {
		t.Error("Expected morning message to be canceled")
	}
}
