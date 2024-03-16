package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	openai "skippybot/openai"
	discord "skippybot/discord"
	"time"
)

type Context struct {
	Thread       openai.Thread
	CreateThread bool
	Ticker       *time.Ticker
	OpenAIKey    string
}

func (c *Context) UpdateThread(thread openai.Thread) {
	c.Thread = thread
}

func (c *Context) UpdateCreateThread(val bool) {
	c.CreateThread = val
}

func (c *Context) ResetTicker(min int) {
	c.Ticker.Reset(time.Duration(min) * time.Minute)
}

const THREAD_TIMEOUT = 30

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

	discord.RunDiscord(token, client)
}

func printStruct(v any) {
	jsonV, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(jsonV))
}
