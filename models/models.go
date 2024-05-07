package models

// used for sending a message on a specific discord channel
type ChannelMessage struct {
	Message     string `json:"message"`
	TimerLength int    `json:"timer_length,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
}
