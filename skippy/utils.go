package skippy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func removeBotMention(content string, botID string) string {
	mentionPattern := fmt.Sprintf("<@%s>", botID)
	// remove nicknames
	mentionPatternNick := fmt.Sprintf("<@!%s>", botID)

	content = strings.Replace(content, mentionPattern, "", -1)
	content = strings.Replace(content, mentionPatternNick, "", -1)
	return content
}

func removeRoleMention(content string, botID string) string {
	mentionPattern := fmt.Sprintf("<@&%s>", botID)

	content = strings.Replace(content, mentionPattern, "", -1)
	return content
}

func replaceChannelIDs(content string, channels []*discordgo.Channel) string {
	for _, channel := range channels {
		mentionPattern := fmt.Sprintf("<#%s>", channel.ID)
		content = strings.Replace(content, mentionPattern, "", -1)
	}
	return content
}

func isRoleMentioned(dg DiscordSession, m *discordgo.MessageCreate) (string, bool) {

	member, err := dg.GuildMember(m.GuildID, dg.GetState().User.ID)
	if err != nil {
		return "", false
	}

	for _, role := range m.MentionRoles {
		if slices.Contains(member.Roles, role) {
			return role, true
		}
	}
	return "", false
}

func isMentioned(mentions []*discordgo.User, currUser *discordgo.User) bool {
	for _, user := range mentions {
		if user.ID == currUser.ID {
			return true
		}
	}
	return false
}

//lint:ignore U1000 saving for later
func removeQuery(url string) string {
	// Find the index of the first occurrence of "?"
	index := strings.Index(url, "?")

	// If "?" is found, return the substring up to the "?"
	if index != -1 {
		return url[:index]
	}

	// If "?" is not found, return the original URL
	return url
}

//lint:ignore U1000 saving for later
func downloadAttachment(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Println("download successful attempting to write")
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
