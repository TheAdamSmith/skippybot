package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func RunDiscord(context *Context) {
	err := godotenv.Load()
	token := os.Getenv("DISCORD_TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
			messageCreate(s, m, context)
	})
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i, context)
		}
	})

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	command := discordgo.ApplicationCommand{
		Name:        "skippy",
		Description: "Control Skippy the Magnificent",
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

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, context *Context) {
	fmt.Println(i.ApplicationCommandData().Name)
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

	// TODO: add flag handling
	textOption := i.ApplicationCommandData().Options[0].StringValue()
	if textOption == "newthread" {
		fmt.Println("atempting to reset thread")
		context.UpdateThread(StartThread(context.OpenAIKey))
		context.ResetTicker(THREAD_TIMEOUT)

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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, context *Context) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	fmt.Printf("Message: %s", m.Content)
	fmt.Printf("User: %s", s.State.User.ID)
	// Check if the bot is mentioned
	if !isMentioned(m.Mentions, s.State.User.ID) {
		return
	}

	if context.CreateThread {
		fmt.Println("creating new thread")
    context.ResetTicker(THREAD_TIMEOUT)
    context.UpdateThread(StartThread(context.OpenAIKey))
    context.UpdateCreateThread(false)
	}

	s.ChannelTyping(m.ChannelID)
	fmt.Println("attempting to get response")
	response := GetResponse(m.Content, context.Thread.ID, context.OpenAIKey)
	fmt.Println(response)
	s.ChannelMessageSend(m.ChannelID, response)

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
