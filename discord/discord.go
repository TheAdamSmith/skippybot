package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

type State struct {
	rlChannelID string
	threadMap   map[string]*chatThread
	assistantID string
	messageCH   chan ChannelMessage
}

type chatThread struct {
	openAIThread           openai.Thread
	additionalInstructions string
	awaitsResponse         bool
	// messages []string
	// reponses []string
}

// used for sending a message on a specific discord channel
type ChannelMessage struct {
	Message     string `json:"message"`
	TimerLength int    `json:"timer_length,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	IsReminder  bool   `json:"is_reminder,omitempty"`
}

const COMMENTATE_INSTRUCTIONS = `
    Messages will be sent in this thread that will contain the json results of a rocket league game.
    Announce the overall score and commentate on the performance of the home team. Come up with creative insults on their performance, but praise high performers
  `

const DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
  Use responses of varying lengths.
`

func RunDiscord(token string, client *openai.Client, assistantID string) {
	var state *State = &State{
		threadMap:   make(map[string]*chatThread),
		assistantID: assistantID,
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unable to get discord client")
	}

	err = dg.Open()
	if err != nil {
		log.Fatalln("Unable to open discord client", err.Error())
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, client, state)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i, client, state)
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

	messageCh := make(chan ChannelMessage)
	state.messageCH = messageCh
	go func() {
		for {
			select {
			case gameInfo := <-fileCh:
				if state.rlChannelID == "" {
					continue
				}
				getAndSendResponse(dg, state.rlChannelID, gameInfo, client, state)
			case channelMsg := <-messageCh:
				log.Println("Recieved channel message. sending after desired timeout")

				time.AfterFunc(
					time.Duration(channelMsg.TimerLength)*time.Second,
					func() {

						log.Println("attempting to send message on: ", channelMsg.ChannelID)
						dg.ChannelMessageSend(channelMsg.ChannelID, channelMsg.Message)
						if channelMsg.IsReminder {
							state.threadMap[channelMsg.ChannelID].awaitsResponse = true
							go waitForReminderResponse(
								dg,
								channelMsg.ChannelID,
								channelMsg.UserID,
								client,
								state,
							)

						}
					})
			}
		}
	}()

	defer close(messageCh)

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

func waitForReminderResponse(
	s *discordgo.Session,
	channelID string,
	userID string,
	client *openai.Client,
	c *State,
) {

	maxRemind := 5
	timeoutMin := 2 * time.Minute
	timer := time.NewTimer(timeoutMin)

	reminds := 0
	for {

		<-timer.C

		// this value is reset on messageCreate
		if !c.threadMap[channelID].awaitsResponse || reminds == maxRemind {
			timer.Stop()
			return
		}

		reminds++
		timeoutMin = timeoutMin * 2
		timer.Reset(timeoutMin)

		log.Println("sending another reminder")

		if userID == "" {
			userID = "they"
		}
		message := "It looks " + userID + " haven't responsed to this reminder can you generate a response nagging them about it. This is not a tool request."
		getAndSendResponse(s, channelID, message, client, c)
	}

}

func handleSlashCommand(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

	// TODO: find specific command
	// maybe add option to discord package?
	textOption := i.ApplicationCommandData().Options[0].Value // rl_sesh
	if textOption == "start" {
		log.Println("Handling rl_sesh start command. creating new thread")
		thread, err := client.CreateThread(context.Background(), openai.ThreadRequest{})

		if err != nil {
			log.Println("Unable to create thread: ", err)
			err = s.InteractionRespond(i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Unable to start chat thread",
					},
				})
			if err != nil {
				log.Printf("Error responding to slash command: %s\n", err)
			}
			return
		}
		state.threadMap[i.ChannelID] = &chatThread{
			openAIThread:           thread,
			additionalInstructions: COMMENTATE_INSTRUCTIONS,
		}

		// TODO: use method
		state.rlChannelID = i.ChannelID
		err = s.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
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
		state.rlChannelID = ""

		err := s.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
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

func messageCreate(
	s *discordgo.Session,
	m *discordgo.MessageCreate,
	client *openai.Client,
	state *State,
) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	log.Printf("Recieved Message: %s\n", m.Content)

	_, threadExists := state.threadMap[m.ChannelID]

	if threadExists {
		if state.threadMap[m.ChannelID].awaitsResponse {
			getAndSendResponse(s, m.ChannelID, m.Content, client, state)
		}
		// value used by reminders to see if it needs to send another message to user
		state.threadMap[m.ChannelID].awaitsResponse = false
	}

	role, roleMentioned := isRoleMentioned(s, m)
	// Check if the bot is mentioned
	if !(isMentioned(m.Mentions, s.State.User) || roleMentioned) {
		return
	}

	message := removeBotMention(m.Content, s.State.User.ID)
	message = removeRoleMention(message, role)
	message = replaceChannelIDs(message, m.MentionChannels)
	message += "\n current time: "

	format := "Monday, Jan 02 at 03:04 PM"
	message += time.Now().Format(format)
	message += "\n User ID: " + m.Author.Mention()

	log.Println("using message: ", message)

	thread, err := client.CreateThread(context.Background(), openai.ThreadRequest{})

	if err != nil {
		log.Println("Unable to create thread: ", err)
	}

	if !threadExists {
		state.threadMap[m.ChannelID] = &chatThread{
			openAIThread:           thread,
			additionalInstructions: DEFAULT_INSTRUCTIONS,
		}
	}

	for _, attachment := range m.Attachments {
		log.Println("Attachment url: ", attachment.URL)
		// downloadAttachment(attachment.URL, fmt.Sprint(rand.Int()) + ".jpg")
		message += " " + removeQuery(attachment.URL)

	}
	log.Println("CHANELLID: ", m.ChannelID)
	getAndSendResponse(s, m.ChannelID, message, client, state)
}

func getAndSendResponse(
	s *discordgo.Session,
	channelID string,
	message string,
	client *openai.Client,
	state *State) {
	s.ChannelTyping(channelID)

	log.Printf("Recieved message: %s\n", message)

	log.Println("Attempting to get response...")
	response := GetResponse(
		message,
		channelID,
		state.threadMap[channelID].openAIThread.ID,
		state.assistantID,
		state.messageCH,
		client,
		state.threadMap[channelID].additionalInstructions,
	)

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

func isRoleMentioned(s *discordgo.Session, m *discordgo.MessageCreate) (string, bool) {

	member, err := s.GuildMember(m.GuildID, s.State.User.ID)
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
		if user.Username == currUser.Username {
			return true
		}
	}
	return false
}
