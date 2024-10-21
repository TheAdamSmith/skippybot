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

	Skippy := skippy.NewSkippy(openAIKey, token, assistantID)

	if err = Skippy.Run(); err != nil {
		log.Fatalf("unable to start skippy %s", err)
	}

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
