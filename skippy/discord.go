package skippy

import (
	"context"
	"log"
	"os"
	"os/signal"
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
const GENERATE_GAME_STAT_INSTRUCTIONS = `You are summarizing a users game sessions. 
	The message will be a a json formatted list of game sessions. 
	Please summarise the results of the sessions including total hours played and the most played game.
	This is the user mention (%s) of the user you are summarizing. Please include it in your message.
	`

func RunDiscord(token string, assistantID string, client *openai.Client, db Database) {
	state := NewState(assistantID)

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unable to get discord client")
	}

	dg.Identify.Intents = discordgo.IntentsAll

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
			onCommand(s, i, client, state, db)
		}
	})

	dg.AddHandler(func(dg *discordgo.Session, p *discordgo.PresenceUpdate) {
		onPresenceUpdate(dg, p, state, db)
	})

	// deleteSlashCommands(dg)
	initSlashCommands(dg)

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc
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

func onPresenceUpdate(dg *discordgo.Session, p *discordgo.PresenceUpdate, s *State, db Database) {
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
		userSession := &GameSession{
			UserID:    p.User.ID,
			Game:      userPresence.game,
			StartedAt: userPresence.timeStarted,
			Duration:  duration,
		}

		err = db.CreateGameSession(userSession)
		if err != nil {
			log.Println("Unable to create game session: ", err)
		}

		log.Printf(
			"User %s stopped playing game %s, after %s\n",
			member.User.Username,
			userPresence.game,
			duration,
		)
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
