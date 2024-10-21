package skippy

import (
	"context"
	"fmt"
	"log"
	"skippybot/components"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// TODO: make channel id type
type State struct {
	threadMap        map[string]*ChatThread
	userPresenceMap  map[string]UserPresence
	componentHandler *components.ComponentHandler
	mu               sync.RWMutex
	stockApiKey      string
	weatherApiKey    string
	// TODO
	// this is a temporary fix that removes some dependency on
	// the Config struct. The config struct should be moved into here
	// and made a map for config per server
	openAIModel string
}

type ChatThread struct {
	openAIThread   openai.Thread
	awaitsResponse bool
	alwaysRespond  bool
	// TODO: this is can be used across multiple things ()
	// should update this to use separate params
	cancelFunc context.CancelFunc
	mu         sync.Mutex
	// messages []string
	// reponses []string
}

func NewState() *State {
	return &State{
		threadMap:       make(map[string]*ChatThread),
		userPresenceMap: make(map[string]UserPresence),
	}
}

func (s *State) Close() {
	s.componentHandler.Close()
}

func (s *State) GetThread(threadID string) (*ChatThread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	thread, exists := s.threadMap[threadID]
	return thread, exists
}

func (s *State) GetOrCreateThread(threadID string, client *openai.Client) (*ChatThread, error) {
	s.mu.RLock()
	thread, exists := s.threadMap[threadID]
	if exists {
		s.mu.RUnlock()
		return thread, nil
	}

	s.mu.RUnlock()
	err := s.ResetOpenAIThread(threadID, client)
	if err != nil {
		return nil, fmt.Errorf("GetOrCreateThread failed with threadID %s %w", threadID, err)
	}

	s.mu.RLock()
	thread = s.threadMap[threadID]
	s.mu.RUnlock()

	return thread, nil
}

func (s *State) SetThread(threadID string, thread *ChatThread) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.threadMap[threadID] = thread
}

// used for locking a specific chat Thread
// does not lock state
// CALLER MUST CALL UnLockThread
func (s *State) LockThread(threadID string) {
	s.threadMap[threadID].mu.Lock()
}

// unlocks a specific chat thread
// does not unlock state
func (s *State) UnLockThread(threadID string) {
	s.threadMap[threadID].mu.Unlock()
}

func (s *State) AddCancelFunc(
	threadID string,
	cancelFunc context.CancelFunc,
	client *openai.Client,
) {
	// TODO: not completely thread safe
	_, exists := s.threadMap[threadID]
	if !exists {
		s.ResetOpenAIThread(threadID, client)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.threadMap[threadID].cancelFunc = cancelFunc
}

func (s *State) GetCancelFunc(threadID string) (context.CancelFunc, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cancelFunc := s.threadMap[threadID].cancelFunc
	return cancelFunc, cancelFunc != nil
}

func (s *State) UpdatePresence(userID string, opts ...UserPresenceOption) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, exists := s.userPresenceMap[userID]
	if !exists {
		original = UserPresence{}
	}

	for _, opt := range opts {
		opt(&original)
	}

	s.userPresenceMap[userID] = original
}

func (s *State) GetPresence(userID string) (UserPresence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	presence, exists := s.userPresenceMap[userID]
	if !exists {
		return UserPresence{}, false
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
		s.threadMap[threadID] = &ChatThread{}
	}

	s.threadMap[threadID].openAIThread = thread

	return nil
}

func (s *State) ToggleAlwaysRespond(threadID string, client *openai.Client) bool {
	s.mu.Lock()
	_, threadExists := s.threadMap[threadID]
	if !threadExists {
		s.mu.Unlock()
		s.ResetOpenAIThread(threadID, client)
		s.mu.Lock()
	}
	updateVal := !s.threadMap[threadID].alwaysRespond
	s.threadMap[threadID].alwaysRespond = updateVal
	s.mu.Unlock()
	return updateVal
}

func (s *State) GetAlwaysRespond(threadID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	thread, exists := s.threadMap[threadID]
	if !exists {
		return false
	}
	return thread.alwaysRespond
}

// TODO: move to config
func (s *State) SetAwaitsResponse(threadID string, awaitsResponse bool, client *openai.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, threadExists := s.threadMap[threadID]
	if !threadExists {
		s.ResetOpenAIThread(threadID, client)
	}
	s.threadMap[threadID].awaitsResponse = awaitsResponse
}

func (s *State) GetAwaitsResponse(threadID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	thread, exists := s.threadMap[threadID]
	if !exists {
		return false
	}
	return thread.awaitsResponse
}

func (s *State) GetComponentHandler() *components.ComponentHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.componentHandler
}

func (s *State) GetStockAPIKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stockApiKey
}

func (s *State) GetWeatherAPIKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.weatherApiKey
}
