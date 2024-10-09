package skippy

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func scheduleEvent(dg DiscordSession, guildID string, name string, description string, startTime time.Time) (*discordgo.GuildScheduledEvent, error) {
	channel, err := getTopVoiceChannel(dg, guildID)
	if err != nil {
		log.Println("could not get top voice channel: ", err)
		return nil, err
	}

	if name == "" {
		name = "Chillin"
	}

	return dg.GuildScheduledEventCreate(guildID, &discordgo.GuildScheduledEventParams{
		Name:               name,
		Description:        description,
		ScheduledStartTime: &startTime,
		EntityType:         discordgo.GuildScheduledEventEntityTypeVoice,
		PrivacyLevel:       discordgo.GuildScheduledEventPrivacyLevelGuildOnly,
		ChannelID:          channel.ID,
	})
}

func getTopVoiceChannel(dg DiscordSession, guildID string) (*discordgo.Channel, error) {
	channels, err := dg.GuildChannels(guildID)
	if err != nil {
		return nil, err
	}

	var topVoiceChannel *discordgo.Channel
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildVoice {
			if topVoiceChannel == nil || channel.Position < topVoiceChannel.Position && canAccessVoiceChannel(dg, channel.ID) {
				topVoiceChannel = channel
			}
		}
	}
	if topVoiceChannel == nil {
		return nil, fmt.Errorf("unable to find voice channel")
	}
	return topVoiceChannel, nil
}

func canAccessVoiceChannel(dg DiscordSession, channelID string) bool {
	permissions, err := dg.UserChannelPermissions(dg.GetState().User.ID, channelID)
	if err != nil {
		log.Println("error getting channel permissions", err)
		return false
	}

	if permissions&discordgo.PermissionVoiceConnect != 0 && permissions&discordgo.PermissionVoiceSpeak != 0 {
		return true
	}
	return false
}
