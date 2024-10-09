package skippy

import "github.com/bwmarrin/discordgo"

// discordgo.Session interface wrapping for modularity and testing
// implements methods used in this project
type DiscordSession interface {
	// see discordgo.Session.ChannelMessageSend
	ChannelMessageSend(
		channelID, content string,
		options ...discordgo.RequestOption,
	) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (st *discordgo.Message, err error)
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)

	// see discordgo.Session.ChannelTyping()
	ChannelTyping(channelID string, options ...discordgo.RequestOption) error
	ChannelMessageEditEmbed(channelID string, messageID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit, options ...discordgo.RequestOption) (st *discordgo.Message, err error)

	// see discordgo.Session.GuildMember()
	GuildMember(
		guildID string,
		userID string,
		options ...discordgo.RequestOption,
	) (st *discordgo.Member, err error)

	// see discordgo.Session.InteractionRespond()
	InteractionRespond(
		interaction *discordgo.Interaction,
		resp *discordgo.InteractionResponse,
		options ...discordgo.RequestOption,
	) error

	UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error)
	GuildScheduledEventCreate(guildID string, event *discordgo.GuildScheduledEventParams, options ...discordgo.RequestOption) (st *discordgo.GuildScheduledEvent, err error)
	GuildChannels(guildID string, options ...discordgo.RequestOption) (st []*discordgo.Channel, err error)
	UserChannelPermissions(userID string, channelID string, fetchOptions ...discordgo.RequestOption) (apermissions int64, err error)

	AddHandler(handler interface{}) func()
	InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	// wraps discordgo.Session.State
	GetState() *discordgo.State
}

// wrapper
type DiscordBot struct {
	*discordgo.Session
}

func NewDiscordBot(session *discordgo.Session) *DiscordBot {
	return &DiscordBot{Session: session}
}

func (bot *DiscordBot) GetState() *discordgo.State {
	return bot.State
}
