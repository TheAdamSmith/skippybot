package tests

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"skippybot/skippy"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	BOT_ID   = "BOT"
	letters  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	USERNAME = "cap_lapse"
	GAME     = "Outer Wilds"
	USER_ID  = "USERID"
	GUILD_ID = "GUILDID"
)

// these variables are shared between tests which is intentional
// they simulate a live discord environment
var client *openai.Client
var state *skippy.State
var dg *MockDiscordSession
var db skippy.Database
var config *skippy.Config
var enableLogging bool

func init() {
	flag.BoolVar(&enableLogging, "log", false, "enable logging")
}

func TestMain(m *testing.M) {
	flag.Parse()
	if !enableLogging {
		log.SetOutput(io.Discard)
	}
	var err error
	dg, client, state, db, config, err = setup()
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

func setup() (dg *MockDiscordSession, client *openai.Client, state *skippy.State, db skippy.Database, config *skippy.Config, err error) {
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
	if openAIKey == "" {
		err = fmt.Errorf("unable to get Open AI API Key")
		return
	}

	assistantID := os.Getenv("ASSISTANT_ID")
	if assistantID == "" {
		fmt.Errorf("could not read Assistant ID")
	}

	clientConfig := openai.DefaultConfig(openAIKey)
	clientConfig.AssistantVersion = "v2"
	client = openai.NewClientWithConfig(clientConfig)
	state = skippy.NewState(assistantID, "", "", "")

	mrog, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return
	}

	mrog.AutoMigrate(&skippy.GameSession{})
	db = &skippy.DB{mrog}

	config = &skippy.Config{
		MinGameSessionDuration:     time.Nanosecond * 1,
		PresenceUpdateDebouncDelay: time.Millisecond * 100,
		ReminderDurations: []time.Duration{
			time.Millisecond * 50,
			time.Millisecond * 50,
			time.Hour,
		},
		OpenAIModel: openai.GPT3Dot5Turbo,
	}

	return
}
func teardown() {
	err := db.Close()
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
