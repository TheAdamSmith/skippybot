package skippy

import (
	"time"
)

type Config struct {
	// the minimum amount of time a user plays a game
	// to count it as a game session
	MinGameSessionDuration     time.Duration
	PresenceUpdateDebouncDelay time.Duration
	// The schedule set for WaitForReminderResponse
	ReminderDurations []time.Duration
	DefaultModel      string
	DailyGameLimit    time.Duration
	BaseInstructions string
	// discordgo.User.ID -> UserPresenceConfig
	UserConfigMap map[string]UserConfig
	WeatherAPIKey string
	StockAPIKey   string
	Name BotName
}

type UserConfig struct {
	DailyLimit             time.Duration
	WeeklyLimit            time.Duration
	Remind                 bool
	LimitReminderChannelID string
}
