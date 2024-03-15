package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"time"
)

type Context struct {
	Thread       Thread
	CreateThread bool
	Ticker       *time.Ticker
	OpenAIKey    string
}

func (c *Context) UpdateThread(thread Thread) {
	c.Thread = thread
}

func (c *Context) UpdateCreateThread(val bool) {
	c.CreateThread = val
}

func (c *Context) ResetTicker(min int) {
	c.Ticker.Reset(time.Duration(min) * time.Minute)
}

var THREAD_TIMEOUT = 30

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

	context := &Context{
		Thread:       StartThread(openAIKey),
		CreateThread: false,
		Ticker:       time.NewTicker(30 * time.Minute),
		OpenAIKey:    openAIKey,
	}

	defer context.Ticker.Stop()

	go func() {
		for range context.Ticker.C {
			log.Println("Recieved tick. Setting to create new thread with next message")
			context.UpdateCreateThread(true)
		}
	}()

	RunDiscord(token, context)
}

func printStruct(v any) {
	jsonV, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(jsonV))
}
