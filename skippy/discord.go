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

func RunDiscord(
	token string,
	assistantID string,
	stockAPIKey string,
	weatherAPIKey string,
	client *openai.Client,
	db Database,
) {
	state := NewState(assistantID, token, stockAPIKey, weatherAPIKey)

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
	dg *discordgo.Session,
	m *discordgo.MessageCreate,
	client *openai.Client,
	state *State,
) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == dg.State.User.ID {
		return
	}

	log.Printf("Recieved Message: %s\n", m.Content)

	thread := state.GetOrCreateThread(m.ChannelID, client)

	if thread.awaitsResponse {
		messageReq := openai.MessageRequest{
			Role:    openai.ChatMessageRoleUser,
			Content: m.Content,
		}
		getAndSendResponse(
			context.Background(),
			dg,
			m.ChannelID,
			messageReq,
			DEFAULT_INSTRUCTIONS,
			client,
			state,
		)
		// value used by reminders to see if it needs to send another message to user
		state.SetAwaitsResponse(m.ChannelID, false, client)
	}

	role, roleMentioned := isRoleMentioned(dg, m)

	isMentioned := isMentioned(m.Mentions, dg.State.User) || roleMentioned
	if !isMentioned && !thread.alwaysRespond {
		return
	}

	message := removeBotMention(m.Content, dg.State.User.ID)
	message = removeRoleMention(message, role)
	message = replaceChannelIDs(message, m.MentionChannels)
	message += "\n current time: "

	format := "Monday, Jan 02 at 03:04 PM"
	message += time.Now().Format(format)
	message += "\n User ID: " + m.Author.Mention()

	log.Println("using message: ", message)

	log.Println("CHANELLID: ", m.ChannelID)
	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	}
	getAndSendResponse(
		context.Background(),
		dg,
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
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	client *openai.Client,
	state *State) {
	dg.ChannelTyping(dgChannID)

	log.Printf("Recieved message: %s with role: %s\n", messageReq.Content, messageReq.Role)

	log.Println("Attempting to get response...")

	thread := state.GetOrCreateThread(dgChannID, client)

	response, err := GetResponse(
		ctx,
		dg,
		thread.openAIThread.ID,
		dgChannID,
		messageReq,
		state,
		client,
		additionalInstructions,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = "Oh no! Something went wrong."
	}

	_, err = dg.ChannelMessageSend(dgChannID, response)
	if err != nil {
		log.Printf("Could not send discord message on channel %s: %s\n", dgChannID, err)
	}
}

func onPresenceUpdate(dg *discordgo.Session, p *discordgo.PresenceUpdate, s *State, db Database) {
	game, isPlayingGame := getCurrentGame(p)
	isPlayingGame = isPlayingGame && game != ""

	userPresence, exists := s.GetPresence(p.User.ID)

	member, err := dg.State.Member(p.GuildID, p.User.ID)
	if err != nil {
		log.Println("Could not get user: ", err)
	}
	startedPlaying := isPlayingGame && (!exists || exists && !userPresence.isPlayingGame)
	stoppedPlaying := exists && userPresence.isPlayingGame && !isPlayingGame

	if startedPlaying {
		log.Printf("User %s started playing %s\n", member.User.Username, game)
		s.UpdatePresence(p.User.ID, p.Status, isPlayingGame, game, time.Now())
	}

	if stoppedPlaying {

		duration := time.Since(userPresence.timeStarted)

		log.Printf(
			"User %s stopped playing game %s, after %s\n",
			member.User.Username,
			userPresence.game,
			duration,
		)

		// this state update must be done before the db call to avoid race conditions
		s.UpdatePresence(p.User.ID, p.Status, isPlayingGame, "", time.Time{})

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
