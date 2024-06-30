package tests

import (
	"testing"
	"time"

	"skippybot/skippy"

	"github.com/bwmarrin/discordgo"
)

func TestOnPresenceUpdate(t *testing.T) {
	presenceUpdate := &discordgo.PresenceUpdate{
		GuildID: GUILD_ID,
		Presence: discordgo.Presence{
			User: &discordgo.User{
				ID: USER_ID,
			},
			Activities: []*discordgo.Activity{
				{
					Name: GAME,
					Type: discordgo.ActivityTypeGame,
				},
			},
		},
	}

	skippy.OnPresenceUpdate(dg, presenceUpdate, state, db, config)
	userPresence, exists := state.GetPresence(USER_ID)

	if !exists {
		t.Fatal("expected presence to exist")
	}

	if !(userPresence.IsPlayingGame && userPresence.Game == GAME) {
		t.Fatal("expected user presence to have correct state")
	}

	presenceUpdate = &discordgo.PresenceUpdate{
		GuildID: GUILD_ID,
		Presence: discordgo.Presence{
			User: &discordgo.User{
				ID: USER_ID,
			},
			Activities: []*discordgo.Activity{},
		},
	}
	skippy.OnPresenceUpdate(dg, presenceUpdate, state, db, config)
	userPresence, exists = state.GetPresence(USER_ID)

	if !exists {
		t.Fatal("expected presence to exist")
	}

	if userPresence.IsPlayingGame {
		t.Fatal("expected user presence to not be playing game")
	}

	gameSessions, err := db.GetGameSessionsByUser(USER_ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(gameSessions) != 1 {
		t.Error("expected there to be one game session")
	}

	if gameSessions[0].Game != GAME {
		t.Error("Expected game session to have correct game")
	}

	for _, gameSession := range gameSessions {
		db.DeleteGameSession(gameSession.ID)
	}
}

func TestOnPresenceUpdateDebounce(t *testing.T) {
	presenceUpdate := &discordgo.PresenceUpdate{
		GuildID: GUILD_ID,
		Presence: discordgo.Presence{
			User: &discordgo.User{
				ID: USER_ID,
			},
			Activities: []*discordgo.Activity{
				{
					Name: GAME,
					Type: discordgo.ActivityTypeGame,
				},
			},
		},
	}

	debouncer := skippy.NewDebouncer(100 * time.Millisecond)
	for i := 0; i < 3; i++ {
		go skippy.OnPresenceUpdateDebounce(
			dg,
			presenceUpdate,
			state,
			db,
			debouncer,
			config,
		)
		time.Sleep(time.Millisecond * 50)
	}

	time.Sleep(time.Millisecond * 100)
	userPresence, exists := state.GetPresence(USER_ID)
	if !exists {
		t.Fatal("expected presence to exist")
	}

	if !(userPresence.IsPlayingGame && userPresence.Game == GAME) {
		t.Fatal("expected user presence to have correct state")
	}

	presenceUpdate = &discordgo.PresenceUpdate{
		GuildID: GUILD_ID,
		Presence: discordgo.Presence{
			User: &discordgo.User{
				ID: USER_ID,
			},
			Activities: []*discordgo.Activity{},
		},
	}

	for i := 0; i < 3; i++ {
		go skippy.OnPresenceUpdateDebounce(
			dg,
			presenceUpdate,
			state,
			db,
			debouncer,
			config,
		)
		time.Sleep(time.Millisecond * 50)
	}
	time.Sleep(time.Millisecond * 100)

	userPresence, exists = state.GetPresence(USER_ID)
	if !exists {
		t.Fatal("expected presence to exist")
	}

	if userPresence.IsPlayingGame {
		t.Fatal("expected user presence to not be playing game")
	}

	gameSessions, err := db.GetGameSessionsByUser(USER_ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(gameSessions) != 1 {
		t.Error(
			"expected there to be one game session recieved: ",
			len(gameSessions),
		)
	}

	if gameSessions[0].Game != GAME {
		t.Error("Expected game session to have correct game")
	}

	for _, gameSession := range gameSessions {
		db.DeleteGameSession(gameSession.ID)
	}
}
