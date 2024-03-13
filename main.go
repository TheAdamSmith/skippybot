package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func printStruct(v any) {
	jsonV, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(jsonV))
}

func runDiscord() {
	err := godotenv.Load()
	token := os.Getenv("DISCORD_TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}
	dg.AddHandler(messageCreate)
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i)
		}
	})

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	command := discordgo.ApplicationCommand{
		Name:        "skippy",
		Description: "Talk to Skippy the Magnificent",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "newthread",
				Description: "Reset Skippy's thread",
				Required:    false,
			},
		},
	}


	_, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	if err != nil {
		fmt.Println("Error creating application commands", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

var thread Thread
var createThread bool = true
var timer *time.Timer

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
  fmt.Println(i.ApplicationCommandData().Name)
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

  // TODO: add flag handling
  textOption := i.ApplicationCommandData().Options[0].StringValue()
  if textOption == "newthread" {
    fmt.Println("resetting thread")
    createThread = true
  }
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: textOption,
		},
	})
	if err != nil {
		fmt.Println("Error responding to slash command: ", err)
	}
}
func removeBotMention(content string, botID string) string {
	// Prepare the mention patterns for the bot with and without a nickname
	mentionPattern := fmt.Sprintf("<@%s>", botID)
	mentionPatternNick := fmt.Sprintf("<@!%s>", botID)

	// Replace the bot's mentions with an empty string
	content = strings.Replace(content, mentionPattern, "", -1)
	content = strings.Replace(content, mentionPatternNick, "", -1)
	return content
}

func isMentioned(mentions []*discordgo.User, botId string) bool {
	for _, user := range mentions {
		if user.ID == botId {
			fmt.Println(user.ID)
			fmt.Println(botId)
			return true
		}
	}
	return false
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	// Check if the bot is mentioned
	if !isMentioned(m.Mentions, s.State.User.ID) {
		return
	}

	if createThread {
		fmt.Println("creating new thread")
		timer = time.NewTimer(30 * time.Minute)
		thread = StartThread()
		createThread = true
	}

	s.ChannelTyping(m.ChannelID)
	fmt.Println("attempting to get response")
	response := GetResponse(m.Content, thread.ID)
	fmt.Println(response)
	s.ChannelMessageSend(m.ChannelID, response)

	select {
	case <-timer.C:
		createThread = true
	default:
		// do nothing
	}

}

func main() {
	runDiscord()
}
