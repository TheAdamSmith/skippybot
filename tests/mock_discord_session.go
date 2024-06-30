package tests

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

// FOR TESTING
type MockDiscordSession struct {
	channelMessages     map[string][]string
	channelTypingCalled map[string]bool
	channelID           string
	content             string
	State               *discordgo.State
}

func (m *MockDiscordSession) ChannelMessageSend(
	channelID, content string,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	m.channelMessages[channelID] = append(m.channelMessages[channelID], content)
	m.channelID = channelID
	m.content = content
	return nil, nil
}

func (m *MockDiscordSession) ChannelTyping(
	channelID string,
	options ...discordgo.RequestOption,
) error {
	m.channelTypingCalled[channelID] = true
	return nil
}

func (m *MockDiscordSession) GuildMember(
	guildID string,
	userID string,
	options ...discordgo.RequestOption,
) (*discordgo.Member, error) {
	return nil, nil
}

func (m *MockDiscordSession) InteractionRespond(
	interaction *discordgo.Interaction,
	resp *discordgo.InteractionResponse,
	options ...discordgo.RequestOption,
) error {
	log.Println("InteractionRespond")
	return nil
}

func (m *MockDiscordSession) GetState() *discordgo.State {
	return m.State
}
