package tests

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
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
	BOT_ID  = "BOT"
	letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// these variable are shared between tests which is inentional
// they simulate a live discord environment
var client *openai.Client
var state *skippy.State
var dg *MockDiscordSession
var db skippy.Database

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	var err error
	dg, client, state, db, err = setup()
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

func setup() (dg *MockDiscordSession, client *openai.Client, state *skippy.State, db skippy.Database, err error) {
	dg = &MockDiscordSession{
		channelMessages:     make(map[string][]string),
		channelTypingCalled: make(map[string]bool),
	}
	dg.State = &discordgo.State{
		Ready: discordgo.Ready{
			User: &discordgo.User{
				ID: BOT_ID,
			},
		},
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
	// Auto migrate the schema
	return
}

func generateTestData(db skippy.Database, userID string, duration time.Duration, games []string) {
	now := time.Now()

	for j := 0; j < len(games); j++ {
		startTime := now.Add(time.Hour * time.Duration(10+j*2)) // 10 AM and 12 PM sessions
		session := skippy.GameSession{
			UserID:    userID,
			Game:      games[j],
			StartedAt: startTime,
			Duration:  duration,
		}
		db.CreateGameSession(&session)
	}
}
