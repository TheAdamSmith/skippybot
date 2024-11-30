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

func (m *MockDiscordSession) Open() error {
	return nil
}

func (m *MockDiscordSession) Close() error {
	return nil
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

func (m *MockDiscordSession) ChannelMessageSendComplex(
	channelID string, data *discordgo.MessageSend,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	return m.ChannelMessageSend(channelID, data.Content, options...)
}

func (m *MockDiscordSession) ChannelMessageSendEmbed(
	channelID string, embed *discordgo.MessageEmbed,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	return nil, nil
}

func (m *MockDiscordSession) ChannelTyping(
	channelID string,
	options ...discordgo.RequestOption,
) error {
	m.channelTypingCalled[channelID] = true
	return nil
}

func (m *MockDiscordSession) ChannelMessageEditEmbed(channelID string, messageID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	return nil, nil
}

func (m *MockDiscordSession) ChannelMessageEditComplex(_ *discordgo.MessageEdit, options ...discordgo.RequestOption) (st *discordgo.Message, err error) {
	return nil, nil
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

// returns a channel with USER_ID as the channel.ID
func (m *MockDiscordSession) UserChannelCreate(userID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	return &discordgo.Channel{
		ID: userID,
	}, nil
}

func (m *MockDiscordSession) GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	return nil, nil
}

func (m *MockDiscordSession) GuildScheduledEventCreate(guildID string, event *discordgo.GuildScheduledEventParams, options ...discordgo.RequestOption) (*discordgo.GuildScheduledEvent, error) {
	return nil, nil
}

func (m *MockDiscordSession) GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
	return nil, nil
}

func (m *MockDiscordSession) UserChannelPermissions(userID string, channelID string, fetchOptions ...discordgo.RequestOption) (int64, error) {
	return 0, nil
}

func (m *MockDiscordSession) ApplicationCommandCreate(appID string, guildID string, cmd *discordgo.ApplicationCommand, options ...discordgo.RequestOption) (*discordgo.ApplicationCommand, error) {
	return nil, nil
}

func (m *MockDiscordSession) AddHandler(handler interface{}) func() {
	return func() {}
}

func (m *MockDiscordSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	return nil, nil
}

func (m *MockDiscordSession) GetState() *discordgo.State {
	return m.State
}
