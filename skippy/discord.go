package skippy

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

const (
	ERROR_RESPONSE   = "Oh no! Something went wrong."
	EVERYONE_MENTION = "@everyone"
	POLL_INTERVAL    = 1 * time.Minute
)

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
	// TODO: fix
	state.openAIModel = config.OpenAIModel
	scheduler, err := NewScheduler()
	if err != nil {
		log.Fatal("could not create scheduler", err)
	}
	scheduler.Start()

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

	scheduler.AddDurationJob(POLL_INTERVAL, func() {
		pollPresenceStatus(context.Background(), dg, client, state, db, config)
	})

	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(dg, m, client, state, scheduler, config)
	})

	session.AddHandler(
		func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.Type == discordgo.InteractionApplicationCommand {
				onCommand(dg, i, client, state, db, config)
			}
		},
	)

	// discord presence update repeates calls rapidly
	// might be from multiple servers so debounce the calls
	debouncer := NewDebouncer(config.PresenceUpdateDebouncDelay)
	session.AddHandler(func(s *discordgo.Session, p *discordgo.PresenceUpdate) {
		onPresenceUpdateDebounce(dg, p, state, db, debouncer, config)
	})

	// deleteSlashCommands(dg)
	initSlashCommands(session)

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(
		sc,
		syscall.SIGINT,
		syscall.SIGTERM,
		os.Interrupt,
		syscall.SIGTERM,
	)
	<-sc
}

func sendChunkedChannelMessage(
	dg DiscordSession,
	channelID string,
	message string,
) error {
	log.Printf("Sending message on %s: %s\n", channelID, message)
	// Discord has a limit of 2000 characters for a single message
	// If the message is longer than that, we need to split it into chunks
	for len(message) > 0 {
		if len(message) > 2000 {
			_, err := dg.ChannelMessageSend(channelID, message[:2000])
			if err != nil {
				log.Printf(
					"Could not send discord message on channel %s: %s\n",
					channelID,
					err,
				)
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
	scheduler *Scheduler,
	config *Config,
) {

	log.Printf("Recieved Message: %s\n", m.Content)

	thread, threadExists := state.GetThread(m.ChannelID)

	// Ignore all messages created by the bot itself
	if m.Author.ID == dg.GetState().User.ID {
		return
	}

	if threadExists && thread.awaitsResponse {
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
			scheduler,
			config,
		)
		// value used by reminders to see if it needs to send another message to user
		state.SetAwaitsResponse(m.ChannelID, false, client)
		scheduler.CancelReminderJob(m.ChannelID)
	}

	role, roleMentioned := isRoleMentioned(dg, m)

	isMentioned := isMentioned(m.Mentions, dg.GetState().User) || roleMentioned
	alwaysRespond := threadExists && thread.alwaysRespond
	if !isMentioned && !alwaysRespond {
		return
	}

	message := removeBotMention(m.Content, dg.GetState().User.ID)
	message = removeRoleMention(message, role)
	message = replaceChannelIDs(message, m.MentionChannels)
	message += "\n current time: "

	// TODO: put in instructions
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
		scheduler,
		config,
	)
}

// Gets response from ai disables functions calls.
// Only capable of getting and sending a response
func getAndSendResponseWithoutTools(
	ctx context.Context,
	dg DiscordSession,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	client *openai.Client,
	state *State,
) error {
	dg.ChannelTyping(dgChannID)

	log.Printf("Using message: %s\n", messageReq.Content)

	log.Println("Attempting to get response...")

	thread, err := state.GetOrCreateThread(dgChannID, client)
	if err != nil {
		return fmt.Errorf("getAndSendResponseWithoutTools failed with channelID %s %w", dgChannID, err)
	}
	// lock the thread because we can't queue additional messages during a run
	state.LockThread(dgChannID)
	defer state.UnLockThread(dgChannID)

	response, err := GetResponse(
		ctx,
		dg,
		thread.openAIThread.ID,
		dgChannID,
		messageReq,
		additionalInstructions,
		true,
		client,
		state,
		// TODO: find a better way to pass this optionally
		nil,
		nil,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = ERROR_RESPONSE
	}

	err = sendChunkedChannelMessage(dg, dgChannID, response)
	return err
}

func getAndSendResponse(
	ctx context.Context,
	dg DiscordSession,
	dgChannID string,
	messageReq openai.MessageRequest,
	additionalInstructions string,
	client *openai.Client,
	state *State,
	scheduler *Scheduler,
	config *Config,
) error {
	dg.ChannelTyping(dgChannID)

	log.Printf("Using message: %s\n", messageReq.Content)

	log.Println("Attempting to get response...")

	thread, err := state.GetOrCreateThread(dgChannID, client)
	if err != nil {
		return fmt.Errorf("getAndSendResponse failed with channelID %s %w", dgChannID, err)
	}
	// lock the thread because we can't queue additional messages during a run
	state.LockThread(dgChannID)
	defer state.UnLockThread(dgChannID)

	response, err := GetResponse(
		ctx,
		dg,
		thread.openAIThread.ID,
		dgChannID,
		messageReq,
		additionalInstructions,
		false,
		client,
		state,
		scheduler,
		config,
	)
	if err != nil {
		log.Println("Unable to get response: ", err)
		response = ERROR_RESPONSE
	}

	err = sendChunkedChannelMessage(dg, dgChannID, response)
	return err
}
