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
	OpenAIModel       string
}
