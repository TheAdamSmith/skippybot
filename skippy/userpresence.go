package skippy

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

type UserPresence struct {
	Status            discordgo.Status
	IsPlayingGame     bool
	Game              string
	TimeStarted       time.Time
	LastLimitReminder time.Time
}

type UserPresenceOption func(*UserPresence)

func WithStatus(status discordgo.Status) UserPresenceOption {
	return func(up *UserPresence) {
		up.Status = status
	}
}

func WithIsPlayingGame(isPlayingGame bool) UserPresenceOption {
	return func(up *UserPresence) {
		up.IsPlayingGame = isPlayingGame
	}
}

func WithGame(game string) UserPresenceOption {
	return func(up *UserPresence) {
		up.Game = game
	}
}

func WithTimeStarted(timeStarted time.Time) UserPresenceOption {
	return func(up *UserPresence) {
		up.TimeStarted = timeStarted
	}
}

func WithLastLimitReminder(lastLimitReminder time.Time) UserPresenceOption {
	return func(up *UserPresence) {
		up.LastLimitReminder = lastLimitReminder
	}
}
