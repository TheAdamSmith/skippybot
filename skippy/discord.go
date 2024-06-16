package skippy

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

const COMMENTATE_INSTRUCTIONS = `
    Messages will be sent in this thread that will contain the json results of a rocket league game.
    Announce the overall score and commentate on the performance of the home team. Come up with creative insults on their performance, but praise high performers
  `

const DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
  Use responses of varying lengths.
`
const MORNING_MESSAGE_INSTRUCTIONS = `
You are making creating morning wake up message for the users of a discord server. Make sure to mention @here in your message. 
Be creative in the message you create in wishing everyone good morning. If there is weather data included in the message please give a brief overview of the weather for each location.
if there is stock price information included in the message include that information in the message.
	`

const SEND_CHANNEL_MSG_INSTRUCTIONS = `You are generating a message to send in a discord channel. Generate a message based on the prompt.
	If a user id is provided use it in your message.
`

const (
	RL_SESH        = "rl_sesh"
	ALWAYS_RESPOND = "always_respond"
	SEND_MESSAGE   = "send_message"
)

func RunDiscord(token string, client *openai.Client, assistantID string) {
	state := NewState(assistantID)

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unable to get discord client")
	}

	dg.Identify.Intents = discordgo.IntentGuildPresences | discordgo.IntentGuilds

	err = dg.Open()
	if err != nil {
		log.Fatalln("Unable to open discord client", err.Error())
	}
	defer dg.Close()

	dg.State.TrackPresences = true
	dg.State.TrackChannels = true
	dg.State.TrackMembers = true

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, client, state)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			onCommand(s, i, client, state)
		}
	})

	dg.AddHandler(func(dg *discordgo.Session, p *discordgo.PresenceUpdate) {
		onPresenceUpdate(dg, p, state)
	})

	// deleteSlashCommands(dg)
	initSlashCommands(dg)

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc
}

func onPresenceUpdate(dg *discordgo.Session, p *discordgo.PresenceUpdate, s *State) {
	game, isPlayingGame := getCurrentGame(p)
	userPresence, exists := s.getPresence(p.User.ID)

	member, err := dg.State.Member(p.GuildID, p.User.ID)
	if err != nil {
		log.Println("Could not get user: ", err)
	}
	startedPlaying := isPlayingGame && (!exists || exists && !userPresence.isPlayingGame)
	stoppedPlaying := exists && userPresence.isPlayingGame && !isPlayingGame

	if startedPlaying {
		log.Printf("User %s started playing %s\n", member.User.Username, game)
		s.updatePresence(p.User.ID, p.Status, isPlayingGame, game, time.Now())
	}

	if stoppedPlaying {
		duration := time.Since(userPresence.timeStarted)
		log.Printf("User %s stopped playing game %s, after %s\n", member.User.Username, userPresence.game, duration)
		s.updatePresence(p.User.ID, p.Status, isPlayingGame, "", time.Time{})
	}

}

func getCurrentGame(p *discordgo.PresenceUpdate) (string, bool) {
	for _, activity := range p.Activities {
		if activity.Type == discordgo.ActivityTypeGame {
			return activity.Name, true
		}
	}
	return "", false
}

func deleteSlashCommands(dg *discordgo.Session) error {
	appCommands, err := dg.ApplicationCommands(dg.State.Application.ID, "")
	if err != nil {
		log.Println("Could not get application commands: ", err)
		return fmt.Errorf(
			"could not get application commands:%s",
			err,
		)
	}

	for _, appCommand := range appCommands {
		err = dg.ApplicationCommandDelete(dg.State.Application.ID, "", appCommand.ID)
		if err != nil {
			log.Println("Could not delete command", err)
			return err
		}
	}
	return nil
}

func initSlashCommands(dg *discordgo.Session) ([]*discordgo.ApplicationCommand, error) {
	var commands []*discordgo.ApplicationCommand
	command := discordgo.ApplicationCommand{
		Name:        "send_message",
		Description: "Have the bot send a message",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "channel to send message on",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "message to send",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionMentionable,
				Name:        "mention",
				Description: "anyone to mention",
				Required:    false,
			},
		},
	}

	applicationCommand, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)

	if err != nil {
		log.Println("error creating application command: ", err)
	}

	command = discordgo.ApplicationCommand{
		Name:        "always_respond",
		Description: "Toggle auto respond when on Skippy will always respond to messages in this channel",
	}

	applicationCommand, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)

	if err != nil {
		log.Println("error creating application command: ", err)
	}

	command = discordgo.ApplicationCommand{
		Name:        "rl_sesh",
		Description: "start/stop rl sesh",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "startorstop",
				Description: "should be either start or stop",
				Required:    false,
			},
		},
	}

	applicationCommand, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)
	if err != nil {
		log.Println("error creating application command: ", err)
	}

	return commands, err
}

func onCommand(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {
	log.Println(i.ApplicationCommandData().Name)
	switch i.ApplicationCommandData().Name {
	case RL_SESH:
		handleRLSesh(dg, i, client, state)
	case ALWAYS_RESPOND:
		handleAlwaysRespond(dg, i, client, state)
	case SEND_MESSAGE:
		sendChannelMessage(dg, i, client, state)
	default:
		log.Println("recieved unrecognized command")
		err := dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Recieved unrecognized command",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}

}

func sendChannelMessage(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 2 {
		return fmt.Errorf("recieved an incorrect amount of options")
	}

	channel := options[0].ChannelValue(nil)
	channelID := channel.ID

	prompt := options[1].StringValue()
	var mentionString string

	if len(options) > 2 {
		mention := options[2].UserValue(nil)
		mentionString = mention.Mention()
		log.Println(mentionString)
	}

	message := "prompt: " + prompt + "\n"
	if mentionString != "" {
		message += "User ID :" + mentionString
	}

	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	}

	ctx := context.WithValue(context.Background(), DisableFunctions, true)

	go getAndSendResponse(
		ctx,
		dg,
		channelID,
		messageReq,
		SEND_CHANNEL_MSG_INSTRUCTIONS,
		client,
		state,
	)

	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sent the message",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
		return err
	}

	return nil
}

func handleAlwaysRespond(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {
	enabled := state.toggleAlwaysRespond(i.ChannelID, client)
	var message string
	if enabled {
		message = "Turned on always respond"
	} else {
		message = "Turned off always respond"
	}
	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}
}

func handleRLSesh(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {

	if i.ApplicationCommandData().Name != "rl_sesh" {
		return
	}

	// TODO: find specific command
	// maybe add option to discord package?
	textOption := i.ApplicationCommandData().Options[0].Value // rl_sesh
	if textOption == "start" {
		log.Println("Handling rl_sesh start command. creating new thread")

		err := state.resetOpenAIThread(i.ChannelID, client)
		if err != nil {
			log.Println("Unable to create thread: ", err)
			err = dg.InteractionRespond(i.Interaction,
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

		ctx, cancelFunc := context.WithCancel(context.Background())
		state.threadMap[i.ChannelID].cancelFunc = cancelFunc

		message := "Started rocket league session"
		filePath := os.Getenv("RL_DIR")
		err = StartRocketLeagueSession(ctx, filePath, i.ChannelID, dg, state, client)
		if err != nil {
			message = "unable to start rocket leage session"
		}
		err = dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}

	if textOption == "stop" {

		var message string
		cancelFunc := state.threadMap[i.ChannelID].cancelFunc
		if cancelFunc == nil {
			message = "Unable to stop session no cancel function"
		} else {
			cancelFunc()
			message = "Stopped rocket league session"
		}

		err := dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
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
			messageReq := openai.MessageRequest{
				Role:    openai.ChatMessageRoleUser,
				Content: m.Content,
			}
			getAndSendResponse(
				context.Background(),
				s,
				m.ChannelID,
				messageReq,
				DEFAULT_INSTRUCTIONS,
				client,
				state,
			)
		}
		// value used by reminders to see if it needs to send another message to user
		state.threadMap[m.ChannelID].awaitsResponse = false
	}

	role, roleMentioned := isRoleMentioned(s, m)

	isMentioned := isMentioned(m.Mentions, s.State.User) || roleMentioned
	alwaysRespond := threadExists && state.threadMap[m.ChannelID].alwaysRespond
	// Check if the bot is mentioned
	if !isMentioned && !alwaysRespond {
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

	if !threadExists {
		err := state.resetOpenAIThread(m.ChannelID, client)
		if err != nil {
			log.Println("Unable to reset thread: ", err)
		}
	}

	log.Println("CHANELLID: ", m.ChannelID)
	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	}
	getAndSendResponse(
		context.Background(),
		s,
		m.ChannelID,
		messageReq,
		DEFAULT_INSTRUCTIONS,
		client,
		state,
	)
}

func getAndSendResponse(
	ctx context.Context,
	dg *discordgo.Session,
	channelID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	client *openai.Client,
	state *State) {
	dg.ChannelTyping(channelID)

	log.Printf("Recieved message: %s with role: %s\n", messageReq.Content, messageReq.Role)

	log.Println("Attempting to get response...")

	ctx = context.WithValue(ctx, DGChannelID, channelID)
	_, exists := state.threadMap[channelID]
	if !exists {
		state.resetOpenAIThread(channelID, client)
	}
	ctx = context.WithValue(ctx, ThreadID, state.threadMap[channelID].openAIThread.ID)
	ctx = context.WithValue(ctx, AssistantID, state.assistantID)

	response, err := GetResponse(
		ctx,
		dg,
		messageReq,
		state,
		client,
		additionalInstructions,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = "Oh no! Something went wrong."
	}

	_, err = dg.ChannelMessageSend(channelID, response)
	if err != nil {
		log.Printf("Could not send discord message on channel %s: %s\n", channelID, err)
	}
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
