package main

import (
	"github.com/joho/godotenv"
	"log"
	"os"

	discord "skippybot/discord"
	openai "skippybot/openai"
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

	assistantId := os.Getenv("ASSISTANT_ID")
	if assistantId == "" {
		log.Fatalln("could not read Assistant ID")
	}
	client := openai.NewClient(openAIKey, assistantId)

	discord.RunDiscord(token, client)
}
