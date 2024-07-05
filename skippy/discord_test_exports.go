//go:build test
// +build test

package skippy

var (
	MessageCreate            = messageCreate
	OnCommand                = onCommand
	OnPresenceUpdate         = onPresenceUpdate
	OnPresenceUpdateDebounce = onPresenceUpdateDebounce
	PollPresenceStatus       = pollPresenceStatus
)
