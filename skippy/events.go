package skippy

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func scheduleEvent(dg DiscordSession, guildID string, name string, startTime time.Time) {
	channel, err := getTopVoiceChannel(dg, guildID)
	if err != nil {
		log.Println("could not get top voice channel: ", err)
		return
	}
	if name == "" {
		name = "Chillin"
	}
	_, err = dg.GuildScheduledEventCreate(guildID, &discordgo.GuildScheduledEventParams{
		Name:               name,
		ScheduledStartTime: &startTime,
		EntityType:         discordgo.GuildScheduledEventEntityTypeVoice,
		PrivacyLevel:       discordgo.GuildScheduledEventPrivacyLevelGuildOnly,
		ChannelID:          channel.ID,
	})
	if err != nil {
		log.Println(err)
	}
}

func getTopVoiceChannel(dg DiscordSession, guildID string) (*discordgo.Channel, error) {
	channels, err := dg.GuildChannels(guildID)
	if err != nil {
		return nil, err
	}
	// Find the top (highest positioned) voice channel
	var topVoiceChannel *discordgo.Channel
	for _, channel := range channels {
		// Check if the channel is a voice channel
		if channel.Type == discordgo.ChannelTypeGuildVoice {
			// If no top voice channel is set yet or the current channel has a higher position, set it
			if topVoiceChannel == nil || channel.Position > topVoiceChannel.Position {
				topVoiceChannel = channel
			}
		}
	}
	if topVoiceChannel == nil {
		return nil, fmt.Errorf("unable to find voice channel")
	}
	return topVoiceChannel, nil
}
