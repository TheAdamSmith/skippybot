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
  "slices"


	models "skippybot/models"
	openai "skippybot/openai"

	"github.com/bwmarrin/discordgo"
)

type context struct {
	rlChannelID string
	threadMap   map[string]chatThread
}

type chatThread struct {
	openAIThread           openai.Thread
	additionalInstructions string
	// messages []string
	// reponses []string
}

const COMMENTATE_INSTRUCTIONS = `
    Messages will be sent in this thread that will contain the json results of a rocket league game.
    Announce the overall score and commentate on the performance of the home team. Come up with creative insults on their performance, but praise high performers
  `

const DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
  Use responses of varying lengths.
`

func RunDiscord(token string, client *openai.Client) {
	var c *context = &context{
		threadMap: make(map[string]chatThread),
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unabel to get discord client")
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, client, c)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i, client, c)
		}
	})

	fileCh := make(chan string)

	filePath := os.Getenv("RL_DIR")
	if filePath != "" {
		log.Println(filePath)
		interval := 5 * time.Second
		go WatchFolder(filePath, fileCh, interval)
	} else {
		log.Println("Could not read rocket league folder")
	}

	defer close(fileCh)

	messageCh := make(chan models.ChannelMessage)
	client.SetMessageCH(messageCh)
	go func() {
		for {
			select {
			case gameInfo := <-fileCh:
				if c.rlChannelID == "" {
					continue
				}
				getAndSendResponse(dg, c.rlChannelID, gameInfo, client, c)
			case channelMsg := <-messageCh:
				log.Println("Recieved channel message")
				time.AfterFunc(time.Duration(channelMsg.TimerLength)*time.Second, func() {
					log.Println("attempting to send message on: ", channelMsg.ChannelID)
					dg.ChannelMessageSend(channelMsg.ChannelID, channelMsg.Message)
				})
			}
		}
	}()

  defer close(messageCh)

	err = dg.Open()
	if err != nil {
		log.Fatalln("Unable to open discord client")
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
		log.Println("Handling rl_sesh start command. creating new thread")
		c.threadMap[i.ChannelID] = chatThread{
			openAIThread:           client.StartThread(),
			additionalInstructions: COMMENTATE_INSTRUCTIONS,
		}

		// TODO: use method
		c.rlChannelID = i.ChannelID
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

		// TODO: change
		c.rlChannelID = ""

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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, client *openai.Client, c *context) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("Recieved Message: %s\n", m.Content)

  role, roleMentioned := isRoleMentioned(s, m)
	// Check if the bot is mentioned
	if !(isMentioned(m.Mentions, s.State.User) || roleMentioned){
		return
	}

	message := removeBotMention(m.Content, s.State.User.ID)
	message = removeRoleMention(m.Content, role)
	message = replaceChannelIDs(message, m.MentionChannels)
  log.Println("using message: ", message)

	thread, exists := c.threadMap[m.ChannelID]
	if !exists {
		thread = chatThread{
			openAIThread:           client.StartThread(),
			additionalInstructions: DEFAULT_INSTRUCTIONS,
		}
		c.threadMap[m.ChannelID] = thread
	}

	for _, attachment := range m.Attachments {
		log.Println("Attachment url: ", attachment.URL)
		// downloadAttachment(attachment.URL, fmt.Sprint(rand.Int()) + ".jpg")
		message += " " + removeQuery(attachment.URL)

	}
	log.Println("CHANELLID: ", m.ChannelID)
	getAndSendResponse(s, m.ChannelID, message, client, c)
}

func getAndSendResponse(
	s *discordgo.Session,
	channelID string,
	message string,
	client *openai.Client,
	c *context) {
	s.ChannelTyping(channelID)

	log.Printf("Recieved message: %s\n", message)

	log.Println("Attempting to get response...")
	response := client.GetResponse(message, channelID, c.threadMap[channelID].openAIThread.ID, c.threadMap[channelID].additionalInstructions)

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

func isRoleMentioned( s *discordgo.Session, m *discordgo.MessageCreate) (string, bool) {

  member, err := s.GuildMember(m.GuildID, s.State.User.ID);
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
	log.Println("mentions length", len(mentions))
	for _, user := range mentions {
		log.Printf("comparing %s to %s", user.Username, currUser.Username)
		if user.Username == currUser.Username {
			return true
		}
	}
	return false
}
