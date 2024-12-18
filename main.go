package main

import (
	"io"
	"log"
	"os"
	"os/signal"
	"skippybot/skippy"
	"syscall"

	"github.com/joho/godotenv"
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
	// openAIKey := os.Getenv("GROQ_API_KEY")
	if openAIKey == "" {
		log.Fatalln("Unable to get Open AI API Key")
	}

	var token string
	var botName skippy.BotName
	botType := os.Args[1]
	log.Println(botType)
	switch botType {
	case "skippy":
		token = os.Getenv("SKIPPY_DISCORD_TOKEN")
		if token == "" {
			log.Fatalln("could not read discord token")
		}
		botName = skippy.SKIPPY
	case "glados": 
		token = os.Getenv("GLADOS_DISCORD_TOKEN")
		if token == "" {
			log.Fatalln("could not read discord token")
		}
		botName = skippy.GLADOS
	}

	bot := skippy.NewSkippy(openAIKey, token, botName)

	if err = bot.Run(); err != nil {
		log.Fatalf("unable to start skippy %s", err)
	}
	defer bot.Close()

	sc := make(chan os.Signal, 1)
	signal.Notify(
		sc,
		syscall.SIGINT,
		syscall.SIGTERM,
		os.Interrupt,
		syscall.SIGTERM,
	)
	<-sc
}
