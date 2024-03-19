package discord

import (
	"fmt"
  "net/http"
  "io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
  // "math/rand"

	"github.com/bwmarrin/discordgo"
	openai "skippybot/openai"
)

// TODO: pass in startNewThread func
func RunDiscord(token string, client *openai.Client) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unabel to get discord client")
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, client)
	})
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i, client)
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
	}

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, client *openai.Client) {
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

	// TODO: add flag handling
	textOption := i.ApplicationCommandData().Options[0].StringValue()
	if textOption == "newthread" {
		log.Println("Handling newthread command. Attempting to reset thread")
		client.ThreadID = client.StartThread().ID
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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, client *openai.Client) {
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
	message := removeBotMention(m.Content, s.State.User.ID)
  for _, attachment:= range m.Attachments{
    log.Println("Attachment url: ", attachment.URL)
    // downloadAttachment(attachment.URL, fmt.Sprint(rand.Int()) + ".jpg")    
    message += " " + removeQuery(attachment.URL)

  }
	s.ChannelTyping(m.ChannelID)

	log.Printf("Recieved message: %s\n", message)

	log.Println("Attempting to get response...")
	response := client.GetResponse(m.Content)

	s.ChannelMessageSend(m.ChannelID, response)

}

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

func downloadAttachment(url string, filename string) error {
    // Get the data
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    log.Println("download successful attempting to write")
    // Create the file
    out, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer out.Close()

    // Write the body to file
    _, err = io.Copy(out, resp.Body)
    return err
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
