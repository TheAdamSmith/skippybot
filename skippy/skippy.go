package skippy

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"skippybot/components"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

type BotName string

const (
	DEBOUNCE_DELAY                    = 100 * time.Millisecond
	MIN_GAME_SESSION_DURATION         = 10 * time.Minute
	POLL_INTERVAL                     = 1 * time.Minute
	SKIPPY_INSTRUCTION_PATH           = "./instructions/skippy.md"
	GLADOS_INSTRUCTION_PATH           = "./instructions/glados.md"
	GLADOS                    BotName = "GLaDOS"
	SKIPPY                    BotName = "Skippy"
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

// TODO: need to create option funcs to pass in here and read from env as default?
func NewSkippy(aiClientKey, discordToken string, botName BotName) *Skippy {
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
	// clientConfig.BaseURL = "https://api.groq.com/openai/v1/"
	aiClient := openai.NewClientWithConfig(clientConfig)

	var instructionsFilePath string
	switch botName {
	case SKIPPY:
		instructionsFilePath = SKIPPY_INSTRUCTION_PATH
	case GLADOS:
		instructionsFilePath = GLADOS_INSTRUCTION_PATH
	default:
		log.Fatal("invalid bot type")
	}

	file, err := os.Open(instructionsFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	instructions := string(content)

	log.Println("using instructions: ", instructions)

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
		// DefaultModel: "llama3-groq-70b-8192-tool-use-preview",
		// DefaultModel: "llama-3.1-70b-versatile",
		BaseInstructions: instructions,
		DefaultModel:     openai.GPT4o,
		UserConfigMap:    make(map[string]UserConfig),
		StockAPIKey:      os.Getenv("ALPHA_VANTAGE_API_KEY"),
		WeatherAPIKey:    os.Getenv("WEATHER_API_KEY"),
		Name:             botName,
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

	s.DiscordSession.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		OnMessageCreate(m, s)
	})

	s.DiscordSession.AddHandler(
		func(_ *discordgo.Session, i *discordgo.InteractionCreate) {
			switch i.Type {
			case discordgo.InteractionApplicationCommand:
				OnInteraction(i, s)
			}
		},
	)

	// discord presence update repeates calls rapidly
	// might be from multiple servers so debounce the calls
	debouncer := NewDebouncer(s.Config.PresenceUpdateDebouncDelay)
	s.DiscordSession.AddHandler(func(_ *discordgo.Session, p *discordgo.PresenceUpdate) {
		OnPresenceUpdateDebounce(p, debouncer, s)
	})

	initSlashCommands(s)

	s.Scheduler.Start()

	s.Scheduler.AddDurationJob(POLL_INTERVAL, func() {
		PollPresenceStatus(context.Background(), s)
	})

	// deleteSlashCommands(dg)

	log.Println("Bot is now running. Press CTRL+C to exit.")

	return nil
}

func (s *Skippy) Close() error {
	s.ComponentHandler.Close()
	if err := s.DiscordSession.Close(); err != nil {
		return err
	}
	if err := s.DB.Close(); err != nil {
		return err
	}
	return nil
}
