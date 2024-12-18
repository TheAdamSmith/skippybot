package tests

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"skippybot/skippy"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	BOT_ID                  = "BOT"
	letters                 = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	SKIPPY_INSTRUCTION_PATH = "../instructions/skippy.md"
	GLADOS_INSTRUCTION_PATH = "../instructions/glados.md"
	USERNAME                = "cap_lapse"
	GAME                    = "Outer Wilds"
	USER_ID                 = "USERID"
	GUILD_ID                = "GUILDID"
)

// these variables are shared between tests which is intentional
// they simulate a live discord environment
var (
	s             *skippy.Skippy
	dg            *MockDiscordSession
	enableLogging bool
	botName       *string
)

func init() {
	flag.BoolVar(&enableLogging, "log", false, "enable logging")
	botName = flag.String("bot", "glados", "skippy or glados")
}

func TestMain(m *testing.M) {
	flag.Parse()
	if !enableLogging {
		log.SetOutput(io.Discard)
	}
	var err error
	s, err = setup()
	defer teardown()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func GenerateRandomID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func setup() (
	s *skippy.Skippy, err error,
) {
	dg = &MockDiscordSession{
		channelMessages:     make(map[string][]string),
		channelTypingCalled: make(map[string]bool),
	}
	dg.State = discordgo.NewState()
	dg.State.Ready = discordgo.Ready{
		User: &discordgo.User{
			ID: BOT_ID,
		},
	}

	// need to intialize a member and guild
	// so the cache is populated and works for tests
	member := &discordgo.Member{
		User: &discordgo.User{
			ID:       USER_ID,
			Username: USERNAME,
		},
	}
	guild := discordgo.Guild{
		ID:      GUILD_ID,
		Members: []*discordgo.Member{member},
	}

	err = dg.State.GuildAdd(&guild)
	if err != nil {
		return
	}

	err = godotenv.Load("../.env")
	if err != nil {
		return
	}

	openAIKey := os.Getenv("OPEN_AI_KEY")
	// openAIKey := os.Getenv("GROQ_API_KEY")
	if openAIKey == "" {
		err = fmt.Errorf("unable to get Open AI API Key")
		return
	}

	assistantID := os.Getenv("ASSISTANT_ID")
	if assistantID == "" {
		err = fmt.Errorf("could not read Assistant ID")
		return
	}

	clientConfig := openai.DefaultConfig(openAIKey)
	// clientConfig.BaseURL = "https://api.groq.com/openai/v1/"
	client := openai.NewClientWithConfig(clientConfig)
	state := skippy.NewState()

	mrog, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		log.Println(err)
		return
	}

	err = mrog.AutoMigrate(&skippy.GameSession{})
	if err != nil {
		log.Println(err)
		return

	}
	db := &skippy.DB{DB: mrog}

	scheduler, err := skippy.NewScheduler()
	if err != nil {
		return
	}
	scheduler.Start()

	userConfigMap := make(map[string]skippy.UserConfig)
	userConfigMap[USER_ID] = skippy.UserConfig{
		Remind:      true,
		DailyLimit:  1 * time.Second,
		WeeklyLimit: 1 * time.Second,
	}

	var instructionsFilePath string
	switch *botName {
	case strings.ToLower(string(skippy.SKIPPY)):
		instructionsFilePath = SKIPPY_INSTRUCTION_PATH
	case strings.ToLower(string(skippy.GLADOS)):
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
	config := &skippy.Config{
		MinGameSessionDuration:     time.Nanosecond * 1,
		PresenceUpdateDebouncDelay: time.Millisecond * 100,
		ReminderDurations: []time.Duration{
			time.Millisecond * 50,
			time.Millisecond * 50,
			time.Hour,
		},
		// DefaultModel:  "llama-3.1-70b-versatile",
		BaseInstructions: instructions,
		DefaultModel:     openai.GPT4o,
		UserConfigMap:    userConfigMap,
		StockAPIKey:      os.Getenv("ALPHA_VANTAGE_API_KEY"),
		WeatherAPIKey:    os.Getenv("WEATHER_API_KEY"),
	}
	s = &skippy.Skippy{
		DiscordSession: dg,
		AIClient:       client,
		Config:         config,
		State:          state,
		DB:             db,
		Scheduler:      scheduler,
	}
	return
}

func teardown() {
	err := s.DB.Close()
	if err != nil {
		fmt.Println("unable to close db connection: ", err)
	}
}

func generateTestData(
	db skippy.Database,
	userID string,
	duration time.Duration,
	games []string,
) {
	now := time.Now()

	for j := 0; j < len(games); j++ {
		startTime := now.Add(
			time.Hour * time.Duration(10+j*2),
		) // 10 AM and 12 PM sessions
		session := skippy.GameSession{
			UserID:    userID,
			Game:      games[j],
			StartedAt: startTime,
			Duration:  duration,
		}
		log.Println(session)
		db.CreateGameSession(&session)
	}
}

func checkForErrorResponse(messages []string) bool {
	for _, message := range messages {
		if strings.Contains(message, skippy.ERROR_RESPONSE) {
			return true
		}
	}
	return false
}
