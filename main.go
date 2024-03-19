package main

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	openai "skippybot/openai"
	discord "skippybot/discord"
)

const DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
  Be nice and charming.
  Use responses of varying lengths.
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

	assistantId := "asst_YZ9utNnMlf1973bcH5ND7Tf1"
  client := openai.NewClient(openAIKey, assistantId)
  defer client.Close()
  client.UpdateAdditionalInstructions(DEFAULT_INSTRUCTIONS)

	discord.RunDiscord(token, client)
}
