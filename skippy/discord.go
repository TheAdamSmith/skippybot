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
	config *Config,
	db Database,
) {
	state := NewState(assistantID, token, stockAPIKey, weatherAPIKey)

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unable to get discord client")
	}
	session.Identify.Intents = discordgo.IntentsAll
	err = session.Open()
	if err != nil {
		log.Fatalln("Unable to open discord client", err.Error())
	}
	defer session.Close()

	session.State.TrackPresences = true
	session.State.TrackChannels = true
	session.State.TrackMembers = true

	dg := NewDiscordBot(session)
	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(dg, m, client, state, config)
	})

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			onCommand(dg, i, client, state, db, config)
		}
	})

	// discord presence update repeates calls rapidly
	// might be from multiple servers so debounce the calls
	debouncer := NewDebouncer(config.PresenceUpdateDebouncDelay)
	session.AddHandler(func(s *discordgo.Session, p *discordgo.PresenceUpdate) {
		debouncer.Debounce(p.User.ID, func() {
			onPresenceUpdate(dg, p, state, db, config)
		},
		)
	})

	// deleteSlashCommands(dg)
	initSlashCommands(session)

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc
}

func sendChunkedChannelMessage(dg DiscordSession, channelID string, message string) error {
	// Discord has a limit of 2000 characters for a single message
	// If the message is longer than that, we need to split it into chunks
	for len(message) > 0 {
		if len(message) > 2000 {
			_, err := dg.ChannelMessageSend(channelID, message[:2000])
			if err != nil {
				log.Printf("Could not send discord message on channel %s: %s\n", channelID, err)
				return err
			}
			message = message[2000:]
		} else {
			dg.ChannelMessageSend(channelID, message)
			break
		}
	}
	return nil
}

func messageCreate(
	dg DiscordSession,
	m *discordgo.MessageCreate,
	client *openai.Client,
	state *State,
	config *Config,
) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == dg.GetState().User.ID {
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
			config,
		)
		// value used by reminders to see if it needs to send another message to user
		state.SetAwaitsResponse(m.ChannelID, false, client)
	}

	role, roleMentioned := isRoleMentioned(dg, m)

	isMentioned := isMentioned(m.Mentions, dg.GetState().User) || roleMentioned
	if !isMentioned && !thread.alwaysRespond {
		return
	}

	message := removeBotMention(m.Content, dg.GetState().User.ID)
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
		config,
	)
}

func getAndSendResponse(
	ctx context.Context,
	dg DiscordSession,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	client *openai.Client,
	state *State,
	config *Config,
) error {
	dg.ChannelTyping(dgChannID)

	log.Printf("Using message: %s\n", messageReq.Content)

	log.Println("Attempting to get response...")

	thread := state.GetOrCreateThread(dgChannID, client)

	response, err := GetResponse(
		ctx,
		dg,
		thread.openAIThread.ID,
		dgChannID,
		messageReq,
		additionalInstructions,
		client,
		state,
		config,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = "Oh no! Something went wrong."
	}

	err = sendChunkedChannelMessage(dg, dgChannID, response)
	return err
}

func onPresenceUpdateDebounce(
	dg DiscordSession,
	p *discordgo.PresenceUpdate,
	s *State,
	db Database,
	debouncer *Debouncer,
	config *Config,
) {
	debouncer.Debounce(p.User.ID, func() {
		onPresenceUpdate(dg, p, s, db, config)
	})
}

func onPresenceUpdate(dg DiscordSession, p *discordgo.PresenceUpdate, s *State, db Database, config *Config) {

	game, isPlayingGame := getCurrentGame(p)
	isPlayingGame = isPlayingGame && game != ""

	userPresence, exists := s.GetPresence(p.User.ID)

	member, err := dg.GetState().Member(p.GuildID, p.User.ID)
	if err != nil {
		log.Println("Could not get user: ", err)
		return
	}

	startedPlaying := isPlayingGame && (!exists || exists && !userPresence.IsPlayingGame)
	stoppedPlaying := exists && userPresence.IsPlayingGame && !isPlayingGame

	if startedPlaying {
		log.Printf("User %s started playing %s\n", member.User.Username, game)
		s.UpdatePresence(p.User.ID, p.Status, isPlayingGame, game, time.Now())
	}

	if stoppedPlaying {

		duration := time.Since(userPresence.TimeStarted)

		log.Printf(
			"User %s stopped playing game %s, after %s\n",
			member.User.Username,
			userPresence.Game,
			duration,
		)

		// this state update must be done before the db call to avoid race conditions
		// later me: probably not required cuz debounced now
		s.UpdatePresence(p.User.ID, p.Status, isPlayingGame, "", time.Time{})

		if duration < config.MinGameSessionDuration {
			return
		}

		userSession := &GameSession{
			UserID:    p.User.ID,
			Game:      userPresence.Game,
			StartedAt: userPresence.TimeStarted,
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
