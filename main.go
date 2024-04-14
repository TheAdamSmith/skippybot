package main

import (
	"github.com/joho/godotenv"
	"log"
	"os"

	discord "skippybot/discord"
	openai "skippybot/openai"
)

const DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
  Be nice and charming.
  Use responses of varying lengths.
`

const COMMENTATE_INSTRUCTIONS = `
    Messages will be sent in this thread that will contain the csv results of a rocket league game.
    Look at the results of each game and respond as if you were a commentator summarizing the game.
  `

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

	defer client.Close()

	client.UpdateAdditionalInstructions(DEFAULT_INSTRUCTIONS)

	discord.RunDiscord(token, client)
}
