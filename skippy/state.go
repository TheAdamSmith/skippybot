package skippy

import (
	"context"
	"log"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

type State struct {
	threadMap       map[string]*chatThread
	userPresenceMap map[string]*userPresence
	assistantID     string
	mu              sync.RWMutex
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

func (s *State) GetThread(threadID string) (*chatThread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	thread, exists := s.threadMap[threadID]
	return thread, exists
}

func (s *State) SetThread(threadID string, thread *chatThread) {
	s.threadMap[threadID] = thread
}

func (s *State) AddCancelFunc(threadID string, cancelFunc context.CancelFunc, client *openai.Client) {
	_, exists := s.threadMap[threadID]
	if !exists {
		s.ResetOpenAIThread(threadID, client)
	}
	s.threadMap[threadID].cancelFunc = cancelFunc
}

func (s *State) UpdatePresence(
	userID string,
	status discordgo.Status,
	isPlayingGame bool,
	game string,
	timeStarted time.Time,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.userPresenceMap[userID]
	if !exists {
		s.userPresenceMap[userID] = &userPresence{}
	}
	s.userPresenceMap[userID].status = status
	s.userPresenceMap[userID].isPlayingGame = isPlayingGame
	s.userPresenceMap[userID].game = game
	s.userPresenceMap[userID].timeStarted = timeStarted
}

func (s *State) GetPresence(userID string) (*userPresence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	presence, exists := s.userPresenceMap[userID]
	if !exists {
		return nil, false
	}
	return presence, true
}

func (s *State) ResetOpenAIThread(threadID string, client *openai.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *State) ToggleAlwaysRespond(threadID string, client *openai.Client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, threadExists := s.threadMap[threadID]
	if !threadExists {
		s.ResetOpenAIThread(threadID, client)
	}
	updateVal := !s.threadMap[threadID].alwaysRespond
	s.threadMap[threadID].alwaysRespond = updateVal
	return updateVal
}

func (s *State) SetAwaitsResponse(threadID string, awaitsResponse bool, client *openai.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, threadExists := s.threadMap[threadID]
	if !threadExists {
		s.ResetOpenAIThread(threadID, client)
	}
	s.threadMap[threadID].awaitsResponse = awaitsResponse
}

func (s *State) GetAssistantID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.assistantID
}
