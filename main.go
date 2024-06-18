package main

import (
	"io"
	"log"
	"os"

	"github.com/joho/godotenv"

	skippy "skippybot/skippy"

	openai "github.com/sashabaranov/go-openai"
)

func main() {

	// Create or open a file for logging
	file, err := os.OpenFile("skippy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
	skippy.RunDiscord(token, assistantID, stockPriceAPIKey, weatherAPIKey, client, db)
}
