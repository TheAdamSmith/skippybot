package discord

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	openai "skippybot/openai"

	"github.com/bwmarrin/discordgo"
)

type context struct {
	channelID string
}

func RunDiscord(token string, client *openai.Client) {
	var c *context
	c = new(context)
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Unabel to get discord client")
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, client)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i, client, c)
		}
	})

	fileCh := make(chan string)
	filePath := "/mnt/c/Users/12asm/AppData/Roaming/bakkesmod/bakkesmod/data/RLStatSaver/2024/"
	// filePath := "/mnt/c/Users/12asm/AppData/Roaming/bakkesmod/bakkesmod/data/RLStatSaver/2024/test.txt"
	interval := 5 * time.Second
	go watchFolder(filePath, fileCh, interval)

	go func() {
		for {
			select {
			case gameInfo := <-fileCh:
				if c.channelID == "" {
					continue
				}
				log.Println(gameInfo)
				getAndSendResponse(dg, c.channelID, client, gameInfo)
			}
		}
	}()

	defer close(fileCh)

	err = dg.Open()
	if err != nil {
		log.Fatalln("Unabel to open discord client")
	}

	command := discordgo.ApplicationCommand{
		Name:        "skippy",
		Description: "Control Skippy the Magnificent",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "rl_sesh",
				Description: "Start or Stop a rocket league session",
				Required:    false,
			},
		},
	}

	_, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	if err != nil {
		log.Printf("Error creating application commands: %s\n", err)
	}

	log.Println("Bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, client *openai.Client, c *context) {
	if i.ApplicationCommandData().Name != "skippy" {
		return
	}

	// TODO: find specific command
	// maybe add option to discord package?
	textOption := i.ApplicationCommandData().Options[0].Value // rl_sesh
	if textOption == "start" {
		log.Println("Handling newthread command. Attempting to reset thread")
		client.ThreadID = client.StartThread().ID

		// TODO: use method
		c.channelID = i.ChannelID

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Started new session",
			},
		})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, client *openai.Client) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	log.Printf("Recieved Message: %s\n", m.Content)
	log.Printf("Current User: %s\n", s.State.User.ID)
	// Check if the bot is mentioned
	if !isMentioned(m.Mentions, s.State.User.ID) {
		return
	}
	message := removeBotMention(m.Content, s.State.User.ID)
	for _, attachment := range m.Attachments {
		log.Println("Attachment url: ", attachment.URL)
		// downloadAttachment(attachment.URL, fmt.Sprint(rand.Int()) + ".jpg")
		message += " " + removeQuery(attachment.URL)

	}
	getAndSendResponse(s, m.ChannelID, client, message)
}

func getAndSendResponse(s *discordgo.Session, channelID string, client *openai.Client, message string) {
	s.ChannelTyping(channelID)

	log.Printf("Recieved message: %s\n", message)

	log.Println("Attempting to get response...")
	response := client.GetResponse(message)

	s.ChannelMessageSend(channelID, response)
}

func watchFolder(filePath string, ch chan<- string, interval time.Duration) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Println("Error reading folder:", err)
		return
	}

	if !fileInfo.IsDir() {
		watchFile(filePath, ch, interval)
	}

	dir, err := os.Open(filePath)
	if err != nil {
		log.Println("Could not epen directory: ", filePath, err)
	}
	files, err := dir.ReadDir(0)
	for _, file := range files {
		go watchFile(filePath+file.Name(), ch, interval)
	}
}

func watchFile(filePath string, ch chan<- string, interval time.Duration) {
	var lastModTime time.Time

	for {
		// Retrieve file info
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Println("Error checking file:", err)
			continue
		}

		modTime := fileInfo.ModTime()
		if lastModTime == (time.Time{}) {
			lastModTime = modTime
		}
		// Check if the modification time has changed
		if modTime.After(lastModTime) {
			log.Println("File has been modified at", modTime)
			lastModTime = modTime
			gameInfo := getGameData(filePath)
			if gameInfo != "" {
				ch <- gameInfo
			}
		}

		time.Sleep(interval)
	}
}

/*
assumes using RL Stat saver https://bakkesplugins.com/plugins/view/390
reads most recent game that is from the bottom up
*/
func getGameData(filePath string) string {
	if filepath.Ext(filePath) != ".csv" {
		log.Println("Attempted to read non csv file: ", filePath)
		return ""
	}
	file, err := os.Open(filePath)
	if err != nil {
		log.Println("Error could not open file", filePath)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)

	records, err := csvReader.ReadAll()
	if err != nil {
		log.Println("Could not read csv: ", filePath, err)
	}

	index := -1
	for i := len(records) - 1; i >= 0; i-- {
		if records[i][0] == "TEAM COLOR" {
			index = i
			break
		}
	}

	if index == -1 {
		log.Println("Could not find team data in csv: ", filePath)
		return ""
	}

	// only get last game and remove the unecessary cols
	return toCsv(records[index:], 0, 8)
}

func toCsv(records [][]string, startCol int, endCol int) string {
	var retVal string
	for _, record := range records {
		for i := startCol; i < endCol; i++ {
			retVal += record[i] + ","
		}
		retVal += "\n"
	}
	return retVal
}

func removeQuery(url string) string {
	// Find the index of the first occurrence of "?"
	index := strings.Index(url, "?")

	// If "?" is found, return the substring up to the "?"
	if index != -1 {
		return url[:index]
	}

	// If "?" is not found, return the original URL
	return url
}

func downloadAttachment(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Println("download successful attempting to write")
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func removeBotMention(content string, botID string) string {
	mentionPattern := fmt.Sprintf("<@%s>", botID)
	// remove nicknames
	mentionPatternNick := fmt.Sprintf("<@!%s>", botID)

	content = strings.Replace(content, mentionPattern, "", -1)
	content = strings.Replace(content, mentionPatternNick, "", -1)
	return content
}

func isMentioned(mentions []*discordgo.User, botId string) bool {
	for _, user := range mentions {
		if user.ID == botId {
			return true
		}
	}
	return false
}
