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

	// see discordgo.Session.ChannelTyping()
	ChannelTyping(channelID string, options ...discordgo.RequestOption) error

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
