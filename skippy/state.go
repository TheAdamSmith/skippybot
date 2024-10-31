package skippy

import (
	"context"
	"log"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// TODO: make channel id type
type State struct {
	threadMap       map[string]*ChatThread
	userPresenceMap map[string]UserPresence
	mu              sync.RWMutex
}

type ChatThread struct {
	openAIThread   openai.Thread
	awaitsResponse bool
	alwaysRespond  bool
	mu             sync.Mutex
	messages       []openai.ChatCompletionMessage
}

func (thread *ChatThread) Lock() {
	thread.mu.Lock()
}

func (thread *ChatThread) Unlock() {
	thread.mu.Unlock()
}

func NewState() *State {
	return &State{
		threadMap:       make(map[string]*ChatThread),
		userPresenceMap: make(map[string]UserPresence),
	}
}

func (s *State) GetThread(threadID string) (*ChatThread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	thread, exists := s.threadMap[threadID]
	return thread, exists
}

func (s *State) NewThread(threadID string) *ChatThread {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threadMap[threadID] = &ChatThread{}
	return s.threadMap[threadID]
}

func (s *State) SetThreadMessages(threadID string, messages []openai.ChatCompletionMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threadMap[threadID].messages = messages
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
