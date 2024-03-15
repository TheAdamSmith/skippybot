package main

import (
	"encoding/json"
	"fmt"
	"time"
  "log"
  "os"
)

type Context struct {
	Thread Thread
  CreateThread bool
	Ticker *time.Ticker
  OpenAIKey string
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
  log.Println("Initializing...")
  openAIKey := os.Getenv("OPEN_AI_KEY")
  log.Printf("API_KEY: %s\n", openAIKey)

	if openAIKey == "" {
		log.Fatalf("could not read key")
		os.Exit(1)
	}

	context := &Context{
		Thread: StartThread(openAIKey),
    CreateThread: false,
		Ticker: time.NewTicker(30 * time.Minute),
    OpenAIKey: openAIKey,
	}

	defer context.Ticker.Stop()

	go func() {
		for range context.Ticker.C {
      log.Println("Recieved tick. Setting to create new thread with next message")
      context.UpdateCreateThread(true)
		}
	}()

	RunDiscord(context)
}

func printStruct(v any) {
	jsonV, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(jsonV))
}
