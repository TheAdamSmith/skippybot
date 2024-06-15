package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	skippy "skippybot/skippy"

	openai "github.com/sashabaranov/go-openai"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Initializing...")

	err := godotenv.Load()
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

	clientConfig := openai.DefaultConfig(openAIKey)
	clientConfig.AssistantVersion = "v2"
	client := openai.NewClientWithConfig(clientConfig)

	skippy.RunDiscord(token, client, assistantID)
}
