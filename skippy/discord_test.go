package skippy

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

const (
	BOT_ID  = "BOT"
	letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var client *openai.Client
var state *State
var dg *MockDiscordSession

func GenerateRandomID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func setup() (dg *MockDiscordSession, client *openai.Client, state *State, err error) {
	log.SetOutput(io.Discard)
	dg = &MockDiscordSession{
		channelMessages:     make(map[string][]string),
		channelTypingCalled: make(map[string]bool),
	}
	dg.State = &discordgo.State{
		Ready: discordgo.Ready{
			User: &discordgo.User{
				ID: BOT_ID,
			},
		},
	}

	err = godotenv.Load("../.env")
	if err != nil {
		return
	}

	openAIKey := os.Getenv("OPEN_AI_KEY")
	if openAIKey == "" {
		err = fmt.Errorf("unable to get Open AI API Key")
		return
	}

	assistantID := os.Getenv("ASSISTANT_ID")
	if assistantID == "" {
		fmt.Errorf("could not read Assistant ID")
	}

	clientConfig := openai.DefaultConfig(openAIKey)
	clientConfig.AssistantVersion = "v2"
	client = openai.NewClientWithConfig(clientConfig)
	state = &State{
		threadMap:       make(map[string]*chatThread),
		userPresenceMap: make(map[string]userPresence),
		assistantID:     assistantID,
	}

	return
}

func TestMain(m *testing.M) {
	var err error
	dg, client, state, err = setup()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestMessageCreateNoMention(t *testing.T) {
	content := "test"
	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
		},
	}

	messageCreate(dg, msg, client, state)
	if len(dg.channelMessages[channelID]) > 0 {
		t.Error("Expected ChannelMessageSend to not be called")
	}
	if dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to not be called")
	}
}

func TestMessageCreateWithMention(t *testing.T) {
	content := "test"
	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	messageCreate(dg, msg, client, state)
	if len(dg.channelMessages[channelID]) != 1 {
		t.Error("Expected ChannelMessageSend to be called")
	}
	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}
}

// TODO: need to test the reminder backoff
// but need to be able to set up testing config first
func TestCreateReminder(t *testing.T) {
	content := "Can you remind me 1 second to take out the trash"
	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	messageCreate(dg, msg, client, state)

	// wait for reminder
	time.Sleep(2 * time.Second)
	if len(dg.channelMessages[channelID]) != 2 {
		t.Error("Expected ChannelMessageSend to be called twice")
	}

	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}

	if !state.threadMap[channelID].awaitsResponse {
		t.Error("Expected thread to be awaiting response")
	}
	// check that bot responds after a reminder with no mention
	msg = &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   "thanks",
			Author: &discordgo.User{
				ID: "USER",
			},
		},
	}

	messageCreate(dg, msg, client, state)
	if len(dg.channelMessages[channelID]) != 3 {
		t.Error("Expected ChannelMessageSend to be called again")
	}

	if state.threadMap[channelID].awaitsResponse {
		t.Error("Expected thread to not be awaiting response")
	}
}

func TestToggleMorningMessage(t *testing.T) {
	// the time for this needs to be longer than it take to make the call to
	// set the morning message
	content := fmt.Sprintf(
		"can you set the morning message for %s",
		time.Now().
			Add(1*time.Minute).
			Format("15:04"))

	channelID := GenerateRandomID(10)
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	messageCreate(dg, msg, client, state)

	// wait for reminder
	timer := time.NewTimer(2 * time.Minute)
loop:
	for {
		select {
		case <-timer.C:
			t.Error("Expected ChannelMessageSend to be called")
			break loop
		default:
			if len(dg.channelMessages[channelID]) == 2 {
				timer.Stop()
				break loop
			}
			time.Sleep(1 * time.Second)
		}
	}

	if len(dg.channelMessages[channelID]) != 2 {
		t.Error("Expected morning message to be sent")
	}

	if state.threadMap[channelID].cancelFunc == nil {
		t.Error("Expected thread to have a cancelFunc")
	}

	if !dg.channelTypingCalled[channelID] {
		t.Error("Expected ChannelTyping to be called")
	}
	content = "cancel the morning message"

	msg = &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "1",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID: "USER",
			},
			Mentions: []*discordgo.User{
				&discordgo.User{
					ID: BOT_ID,
				},
			},
		},
	}

	messageCreate(dg, msg, client, state)

	if len(dg.channelMessages[channelID]) != 3 {
		t.Error("Expected morning message to be canceled")
	}
}
