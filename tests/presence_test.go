package tests

import (
	"skippybot/skippy"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestOnPresenceUpdate(t *testing.T) {
	t.Parallel()

	userID := GenerateRandomID(10)
	guildID := GenerateRandomID(10)
	username := "cap_lapse"
	game := "Outer Wilds"
	// TODO: move to setup
	dg.State = discordgo.NewState()
	member := &discordgo.Member{
		User: &discordgo.User{
			ID:       userID,
			Username: username,
		},
	}
	guild := discordgo.Guild{
		ID:      guildID,
		Members: []*discordgo.Member{member},
	}
	// err := dg.State.MemberAdd()
	err := dg.State.GuildAdd(&guild)
	if err != nil {
		t.Fatal(err)
	}

	presenceUpdate := &discordgo.PresenceUpdate{
		GuildID: guildID,
		Presence: discordgo.Presence{
			User: &discordgo.User{
				ID: userID,
			},
			Activities: []*discordgo.Activity{
				{
					Name: game,
					Type: discordgo.ActivityTypeGame,
				},
			},
		},
	}

	skippy.OnPresenceUpdate(dg, presenceUpdate, state, db)
	userPresence, exists := state.GetPresence(userID)
	if !exists {
		t.Fatal("expected presence to exist")
	}
	if !(userPresence.IsPlayingGame && userPresence.Game == game) {
		t.Fatal("expected user presence to have correct state")
	}
}
