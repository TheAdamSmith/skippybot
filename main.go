package main

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	skippy "skippybot/skippy"

	openai "github.com/sashabaranov/go-openai"
)

const (
	DEBOUNCE_DELAY            = 100 * time.Millisecond
	MIN_GAME_SESSION_DURATION = 10 * time.Minute
)

func main() {
	// Create or open a file for logging
	file, err := os.OpenFile("skippy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	log.SetFlags(log.Ltime | log.Lshortfile | log.Ldate)

	log.Println("Initializing...")
	// Create a multi-writer to write to both the file and stdout
	mw := io.MultiWriter(file, os.Stdout)
	log.SetOutput(mw)
	err = godotenv.Load()
	if err != nil {
		log.Fatalln("Unable to load env variables")
	}

	openAIKey := os.Getenv("OPEN_AI_KEY")
	if openAIKey == "" {
		log.Fatalln("Unable to get Open AI API Key")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatalln("could not read discord token")
	}

	assistantID := os.Getenv("ASSISTANT_ID")
	if assistantID == "" {
		log.Fatalln("could not read Assistant ID")
	}

	stockPriceAPIKey := os.Getenv("ALPHA_VANTAGE_API_KEY")
	weatherAPIKey := os.Getenv("WEATHER_API_KEY")
	clientConfig := openai.DefaultConfig(openAIKey)
	clientConfig.AssistantVersion = "v2"
	client := openai.NewClientWithConfig(clientConfig)

	log.Println("Connecting to db")
	db, err := skippy.NewDB("sqlite", "skippy.db")
	if err != nil {
		log.Fatalln("Unable to get database connection", err)
	}
	userMap := make(map[string]skippy.UserConfig)
	userMap["369593847862525952"] = skippy.UserConfig{
		DailyLimit: time.Duration(1 * time.Second),
	}

	config := &skippy.Config{
		PresenceUpdateDebouncDelay: DEBOUNCE_DELAY,
		MinGameSessionDuration:     MIN_GAME_SESSION_DURATION,
		ReminderDurations: []time.Duration{
			time.Minute * 10,
			time.Minute * 30,
			time.Minute * 90,
			time.Hour * 3,
		},
		OpenAIModel:   openai.GPT4o,
		UserConfigMap: userMap,
	}

	skippy.RunDiscord(
		token,
		assistantID,
		stockPriceAPIKey,
		weatherAPIKey,
		client,
		config,
		db)
}
