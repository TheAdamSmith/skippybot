package skippy

import (
	"context"
	"log"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

type State struct {
	threadMap       map[string]*chatThread
	userPresenceMap map[string]*userPresence
	assistantID     string
}

type userPresence struct {
	status        discordgo.Status
	isPlayingGame bool
	game          string
	timeStarted   time.Time
}

type chatThread struct {
	openAIThread   openai.Thread
	awaitsResponse bool
	alwaysRespond  bool
	// TODO: this is can be used across multiple things ()
	// should update this to use separate params
	cancelFunc context.CancelFunc
	// messages []string
	// reponses []string
}

func NewState(assistantID string) *State {
	return &State{
		threadMap:       make(map[string]*chatThread),
		userPresenceMap: make(map[string]*userPresence),
		assistantID:     assistantID,
	}
}

func (s *State) updatePresence(
	userID string,
	status discordgo.Status,
	isPlayingGame bool,
	game string,
	timeStarted time.Time,
) {
	_, exists := s.userPresenceMap[userID]
	if !exists {
		s.userPresenceMap[userID] = &userPresence{}
	}
	s.userPresenceMap[userID].status = status
	s.userPresenceMap[userID].isPlayingGame = isPlayingGame
	s.userPresenceMap[userID].game = game
	s.userPresenceMap[userID].timeStarted = timeStarted
}

func (s *State) getPresence(userID string) (*userPresence, bool) {
	presence, exists := s.userPresenceMap[userID]
	if !exists {
		return nil, false
	}
	return presence, true
}

func (s *State) resetOpenAIThread(threadID string, client *openai.Client) error {
	log.Println("Resetting thread...")
	thread, err := client.CreateThread(context.Background(), openai.ThreadRequest{})
	if err != nil {
		return err
	}

	_, exists := s.threadMap[threadID]
	if !exists {
		s.threadMap[threadID] = &chatThread{}
	}

	s.threadMap[threadID].openAIThread = thread

	return nil
}

func (s *State) toggleAlwaysRespond(threadID string, client *openai.Client) bool {
	_, threadExists := s.threadMap[threadID]
	if !threadExists {
		s.resetOpenAIThread(threadID, client)
	}
	updateVal := !s.threadMap[threadID].alwaysRespond
	s.threadMap[threadID].alwaysRespond = updateVal
	return updateVal
}
