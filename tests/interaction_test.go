package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"skippybot/skippy"

	"github.com/bwmarrin/discordgo"
)

func TestToggleAlwaysRespond(t *testing.T) {
	t.Parallel()
	channelID := GenerateRandomID(10)

	interaction := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			ChannelID: channelID,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: skippy.ALWAYS_RESPOND,
			},
		},
	}
	skippy.OnCommand(dg, interaction, client, state, db, config)

	if !state.GetAlwaysRespond(channelID) {
		t.Error("Always respond should be true")
	}

	content := "test"
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

	skippy.MessageCreate(dg, msg, client, state, scheduler, config)

	if len(dg.channelMessages[channelID]) != 1 {
		t.Error("Expected ChannelMessageSend to be called")
	}
	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}

	interaction = &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			ChannelID: channelID,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: skippy.ALWAYS_RESPOND,
			},
		},
	}
	skippy.OnCommand(dg, interaction, client, state, db, config)

	if state.GetAlwaysRespond(channelID) {
		t.Error("Always respond should be false")
	}
}

func TestSendChannelMessage(t *testing.T) {
	t.Parallel()
	channelID_1 := GenerateRandomID(10)
	channelID_2 := GenerateRandomID(10)
	mentionID := "00000001"

	interaction := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			GuildID:   GUILD_ID,
			Type:      discordgo.InteractionApplicationCommand,
			ChannelID: channelID_1,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: skippy.SEND_MESSAGE,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Type:  discordgo.ApplicationCommandOptionChannel,
						Name:  skippy.CHANNEL,
						Value: channelID_2,
					},
					{
						Type:  discordgo.ApplicationCommandOptionString,
						Name:  skippy.MESSAGE,
						Value: "test",
					},
					{
						Type:  discordgo.ApplicationCommandOptionString,
						Name:  skippy.MENTION,
						Value: mentionID,
					},
				},
			},
		},
	}
	skippy.OnCommand(dg, interaction, client, state, db, config)
	// wait for message
	timer := time.NewTimer(2 * time.Minute)
loop:
	for {
		select {
		case <-timer.C:
			t.Error("Expected message to be sent to channel 2")
			break loop
		default:
			if len(dg.channelMessages[channelID_2]) == 1 {
				timer.Stop()
				break loop
			}
			time.Sleep(1 * time.Second)
		}
	}
	if !strings.Contains(
		dg.channelMessages[channelID_2][0],
		skippy.UserMention(mentionID),
	) {
		t.Error("Expected message to contain mention")
	}
	if checkForErrorResponse(dg.channelMessages[channelID_1]) ||
		checkForErrorResponse(dg.channelMessages[channelID_2]) {
		t.Error("Expected message to not have error response")
	}
}

func TestGenerateGameStats(t *testing.T) {
	t.Parallel()
	channelID := GenerateRandomID(10)
	userID := "user1"
	// these need to be capitalized and spaced out because bot will automatically do that
	// needed for assertions below
	games := []string{"Valorant", "Rocket League"}
	generateTestData(db, userID, time.Hour, games)

	interaction := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			ChannelID: channelID,
			Member: &discordgo.Member{
				User: &discordgo.User{
					ID: userID,
				},
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: skippy.GAME_STATS,
			},
		},
	}
	skippy.OnCommand(dg, interaction, client, state, db, config)

	// wait for message
	timer := time.NewTimer(2 * time.Minute)
loop:
	for {
		select {
		case <-timer.C:
			t.Error("Expected message to be sent to channel 2")
			break loop
		default:
			if len(dg.channelMessages[channelID]) == 1 {
				timer.Stop()
				break loop
			}
			time.Sleep(1 * time.Second)
		}
	}
	for _, game := range games {
		if !strings.Contains(dg.channelMessages[channelID][0], game) {
			t.Error("Expected message to contain ", game)
		}
	}

	if !strings.Contains(
		dg.channelMessages[channelID][0],
		fmt.Sprint(len(games))+" hours",
	) {
		t.Error("Expected message to contain ", fmt.Sprint(len(games))+" hours")
	}
	if checkForErrorResponse(dg.channelMessages[channelID]) {
		t.Error("Expected message to not have error response")
	}
}
