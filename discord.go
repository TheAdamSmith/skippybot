package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	openai "skippybot/openai"
)

func RunDiscord(token string, context *Context) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unabel to get discord client")
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
		log.Fatalln("Unabel to open discord client")
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
		log.Printf("Error creating application commands: %s\n", err)
		return
	}

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, context *Context) {
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

	// TODO: add flag handling
	textOption := i.ApplicationCommandData().Options[0].StringValue()
	if textOption == "newthread" {
		log.Println("Handling newthread command. Attempting to reset thread")
		context.UpdateThread(openai.StartThread(context.OpenAIKey))
		context.ResetTicker(THREAD_TIMEOUT)

	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Created new thread",
		},
	})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, context *Context) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	log.Printf("Recieved Message: %s\n", m.Content)
	log.Printf("Current User: %s\n", s.State.User.ID)
	// Check if the bot is mentioned
	if !isMentioned(m.Mentions, s.State.User.ID) {
		return
	}

	if context.CreateThread {
		log.Println("Generating new thread.")
		context.ResetTicker(THREAD_TIMEOUT)
		context.UpdateThread(openai.StartThread(context.OpenAIKey))
		context.UpdateCreateThread(false)
	}

	s.ChannelTyping(m.ChannelID)

	message := removeBotMention(m.Content, s.State.User.ID)
	log.Printf("Recieved message: %s\n", message)

	log.Println("Attempting to get response...")
	response :=openai.GetResponse(m.Content, context.Thread.ID, context.OpenAIKey)

	s.ChannelMessageSend(m.ChannelID, response)

}

func removeBotMention(content string, botID string) string {
	mentionPattern := fmt.Sprintf("<@%s>", botID)
	// remove nicknames
	mentionPatternNick := fmt.Sprintf("<@!%s>", botID)

	content = strings.Replace(content, mentionPattern, "", -1)
	content = strings.Replace(content, mentionPatternNick, "", -1)
	return content
}

func isMentioned(mentions []*discordgo.User, botId string) bool {
	for _, user := range mentions {
		if user.ID == botId {
			return true
		}
	}
	return false
}
