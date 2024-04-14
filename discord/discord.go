package discord

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	openai "skippybot/openai"

	"github.com/bwmarrin/discordgo"
)

type context struct {
	channelID string
}

const COMMENTATE_INSTRUCTIONS = `
    Messages will be sent in this thread that will contain the json results of a rocket league game.
    Announce the overall score and commentate on the performance of the home team. Come up with creative insults on their performance, but praise high performers
  `

const DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
  Be nice and charming.
  Use responses of varying lengths.
`

func RunDiscord(token string, client *openai.Client) {
	var c *context
	c = new(context)
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unabel to get discord client")
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, client)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i, client, c)
		}
	})

	fileCh := make(chan string)
	// wsl to windows file path
	filePath := "/mnt/c/Users/12asm/AppData/Roaming/bakkesmod/bakkesmod/data/RLStatSaver/2024/"
	interval := 5 * time.Second
	go WatchFolder(filePath, fileCh, interval)

	go func() {
		for {
			select {
			case gameInfo := <-fileCh:
				if c.channelID == "" {
					continue
				}
				getAndSendResponse(dg, c.channelID, client, gameInfo)
			}
		}
	}()

	defer close(fileCh)

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
				Name:        "rl_sesh",
				Description: "Start or Stop a rocket league session",
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

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, client *openai.Client, c *context) {
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

	// TODO: find specific command
	// maybe add option to discord package?
	textOption := i.ApplicationCommandData().Options[0].Value // rl_sesh
	if textOption == "start" {
		log.Println("Handling newthread command. Attempting to reset thread")
		client.ThreadID = client.StartThread().ID

		// TODO: use method
		c.channelID = i.ChannelID
    client.UpdateAdditionalInstructions(COMMENTATE_INSTRUCTIONS)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Started rocket league session",
			},
		})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}

	if textOption == "stop" {
		log.Println("Handling newthread command. Attempting to reset thread")
		client.ThreadID = client.StartThread().ID

		// TODO: change
		c.channelID = ""

    client.UpdateAdditionalInstructions(DEFAULT_INSTRUCTIONS)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Stopped rocket league session",
			},
		})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
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
	if !isMentioned(m.Mentions, s.State.User.ID) && !strings.Contains(m.Author.Username, "njrage") {
		return
	}
	message := removeBotMention(m.Content, s.State.User.ID)
	for _, attachment := range m.Attachments {
		log.Println("Attachment url: ", attachment.URL)
		// downloadAttachment(attachment.URL, fmt.Sprint(rand.Int()) + ".jpg")
		message += " " + removeQuery(attachment.URL)

	}
	getAndSendResponse(s, m.ChannelID, client, message)
}

func getAndSendResponse(s *discordgo.Session, channelID string, client *openai.Client, message string) {
	s.ChannelTyping(channelID)

	log.Printf("Recieved message: %s\n", message)

	log.Println("Attempting to get response...")
	response := client.GetResponse(message)

	s.ChannelMessageSend(channelID, response)
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
