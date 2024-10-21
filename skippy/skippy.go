package skippy

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"skippybot/components"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

type Skippy struct {
	DiscordSession   DiscordSession
	AIClient         *openai.Client
	State            *State
	DB               Database
	ComponentHandler *components.ComponentHandler
	Config           *Config
	Scheduler        *Scheduler
}

const (
	DEBOUNCE_DELAY            = 100 * time.Millisecond
	MIN_GAME_SESSION_DURATION = 10 * time.Minute
	POLL_INTERVAL             = 1 * time.Minute
)

// TODO: need to create option funcs to pass in here and read from env as default?
func NewSkippy(aiClientKey string, discordToken string, assistantID string) *Skippy {
	session, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalln("Unable to get discord client")
	}

	// TODO: scope down intents
	session.Identify.Intents = discordgo.IntentsAll
	session.State.TrackPresences = true
	session.State.TrackChannels = true
	session.State.TrackMembers = true

	clientConfig := openai.DefaultConfig(aiClientKey)
	clientConfig.AssistantVersion = "v2"
	aiClient := openai.NewClientWithConfig(clientConfig)

	// TODO: should read this from the db first
	config := &Config{
		PresenceUpdateDebouncDelay: DEBOUNCE_DELAY,
		MinGameSessionDuration:     MIN_GAME_SESSION_DURATION,
		ReminderDurations: []time.Duration{
			time.Minute * 10,
			time.Minute * 30,
			time.Minute * 90,
			time.Hour * 3,
		},
		DefaultModel:  openai.GPT4o,
		UserConfigMap: make(map[string]UserConfig),
		StockAPIKey:   os.Getenv("ALPHA_VANTAGE_API_KEY"),
		WeatherAPIKey: os.Getenv("WEATHER_API_KEY"),
		AssistantID:   assistantID,
	}

	log.Println("Connecting to db")
	db, err := NewDB("sqlite", "skippy.db")
	if err != nil {
		log.Fatalln("Unable to get database connection", err)
	}

	scheduler, err := NewScheduler()
	if err != nil {
		log.Fatal("could not create scheduler", err)
	}

	return &Skippy{
		DiscordSession: NewDiscordBot(session),
		AIClient:       aiClient,
		// TODO: does this work
		ComponentHandler: components.NewComponentHandler(session),
		Config:           config,
		State:            NewState(),
		DB:               db,
		Scheduler:        scheduler,
	}
}

func (s *Skippy) Run() error {
	err := s.DiscordSession.Open()
	if err != nil {
		return fmt.Errorf("error unable to open discord session %w", err)
	}
	defer s.DiscordSession.Close()

	s.DiscordSession.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(m, s)
	})

	s.DiscordSession.AddHandler(
		func(_ *discordgo.Session, i *discordgo.InteractionCreate) {
			switch i.Type {
			case discordgo.InteractionApplicationCommand:
				onCommand(i, s)
			}
		},
	)

	// discord presence update repeates calls rapidly
	// might be from multiple servers so debounce the calls
	debouncer := NewDebouncer(s.Config.PresenceUpdateDebouncDelay)
	s.DiscordSession.AddHandler(func(_ *discordgo.Session, p *discordgo.PresenceUpdate) {
		onPresenceUpdateDebounce(p, debouncer, s)
	})

	initSlashCommands(s.DiscordSession)

	s.Scheduler.Start()

	s.Scheduler.AddDurationJob(POLL_INTERVAL, func() {
		pollPresenceStatus(context.Background(), s)
	})

	// deleteSlashCommands(dg)

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
	return nil
}
