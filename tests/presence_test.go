package tests

import (
	"context"
	"strings"
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

	skippy.OnPresenceUpdate(presenceUpdate, s)
	userPresence, exists := s.State.GetPresence(USER_ID)

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
	skippy.OnPresenceUpdate(presenceUpdate, s)
	userPresence, exists = s.State.GetPresence(USER_ID)

	if !exists {
		t.Fatal("expected presence to exist")
	}

	if userPresence.IsPlayingGame {
		t.Fatal("expected user presence to not be playing game")
	}

	gameSessions, err := s.DB.GetGameSessionsByUser(USER_ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(gameSessions) != 1 {
		t.Error("expected there to be one game session")
	}

	if gameSessions[0].Game != GAME {
		t.Error("Expected game session to have correct game")
	}

	// return the channel id as the user id for
	for _, gameSession := range gameSessions {
		s.DB.DeleteGameSession(gameSession.ID)
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
			presenceUpdate, debouncer, s,
		)
		time.Sleep(time.Millisecond * 50)
	}

	time.Sleep(time.Millisecond * 100)
	userPresence, exists := s.State.GetPresence(USER_ID)
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
			presenceUpdate, debouncer, s,
		)
		time.Sleep(time.Millisecond * 50)
	}
	time.Sleep(time.Millisecond * 100)

	userPresence, exists = s.State.GetPresence(USER_ID)
	if !exists {
		t.Fatal("expected presence to exist")
	}

	if userPresence.IsPlayingGame {
		t.Fatal("expected user presence to not be playing game")
	}

	gameSessions, err := s.DB.GetGameSessionsByUser(USER_ID)
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
		s.DB.DeleteGameSession(gameSession.ID)
	}
}

func TestPollPresence(t *testing.T) {
	t.Parallel()

	skippy.PollPresenceStatus(context.Background(), s)
	if len(dg.channelMessages[USER_ID]) != 0 {
		t.Fatal("expected message to be sent not be on user channel")
	}

	s.State.UpdatePresence(
		USER_ID,
		skippy.WithGame("Rocket League"),
		skippy.WithTimeStarted(time.Now().Add(-2*time.Hour)),
		skippy.WithIsPlayingGame(true),
	)

	skippy.PollPresenceStatus(context.Background(), s)
	if len(dg.channelMessages[USER_ID]) != 1 {
		t.Fatal("expected message to be sent on user channel")
	}

	if !dg.channelTypingCalled[USER_ID] {
		t.Error("expected channel typing to be called on user channel")
	}

	skippy.PollPresenceStatus(context.Background(), s)
	if len(dg.channelMessages[USER_ID]) != 1 {
		t.Fatal("expected message to not be sent on user channel again")
	}

	// reset the state so we can test db
	s.State.UpdatePresence(
		USER_ID,
		skippy.WithGame(""),
		skippy.WithTimeStarted(time.Time{}),
		skippy.WithIsPlayingGame(false),
		skippy.WithLastLimitReminder(time.Time{}),
	)

	skippy.PollPresenceStatus(context.Background(), s)
	if len(dg.channelMessages[USER_ID]) != 1 {
		t.Fatal("expected message to not be sent on user channel again")
	}

	games := []string{"Valorant", "Rocket League"}
	generateTestData(s.DB, USER_ID, time.Hour, games)

	skippy.PollPresenceStatus(context.Background(), s)
	if len(dg.channelMessages[USER_ID]) != 2 {
		t.Fatal("expected message to be sent on user channel again")
	}

	if !strings.Contains(
		dg.channelMessages[USER_ID][0],
		skippy.UserMention(USER_ID),
	) {
		t.Error("Expected message to contain user mention")
	}

	if checkForErrorResponse(dg.channelMessages[USER_ID]) {
		t.Error("Expected message to not have error response")
	}
}
