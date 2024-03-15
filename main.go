package main

import (
	"encoding/json"
	"fmt"
	"time"
  "log"

)

type Context struct {
	Thread Thread
	Ticker *time.Ticker
}

func (c *Context) UpdateThread(thread Thread) {
	c.Thread = thread
}
func (c *Context) ResetTicker(min int) {
	c.Ticker.Reset(time.Duration(min) * time.Minute)
}

var THREAD_TIMEOUT = 30
var thread Thread
var createThread bool = true
var timer *time.Timer

func main() {
  log.Println("Initializing...")
	context := &Context{
		Thread: StartThread(),
		Ticker: time.NewTicker(30 * time.Minute),
	}
	defer context.Ticker.Stop()

	go func() {
		for range context.Ticker.C {
      log.Println("Recieved tick. Updating thread")
			context.UpdateThread((StartThread()))
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
