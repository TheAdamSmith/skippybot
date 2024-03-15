package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
  "log"
)

type Thread struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"`
	CreatedAt int64             `json:"created_at"`
	Metadata  map[string]string `json:"metadata"`
}

type MessageReq struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RunReq struct {
	AssistantId  string `json:"assistant_id"`
	Instructions string `json:"instructions"`
}

type MessageList struct {
	Object  string    `json:"object"`
	Data    []Message `json:"data"`
	FirstID string    `json:"first_id"`
	LastID  string    `json:"last_id"`
	HasMore bool      `json:"has_more"`
}

type Message struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	CreatedAt   int64                  `json:"created_at"`
	AssistantID *string                `json:"assistant_id"` // Using *string to handle null values
	ThreadID    string                 `json:"thread_id"`
	RunID       *string                `json:"run_id"` // Using *string to handle null values
	Role        string                 `json:"role"`
	Content     []Content              `json:"content"`
	FileIDs     []string               `json:"file_ids"`
	Metadata    map[string]interface{} `json:"metadata"` // Using interface{} for flexibility
}

type Content struct {
	Type string `json:"type"`
	Text Text   `json:"text"`
}

type Text struct {
	Value       string        `json:"value"`
	Annotations []interface{} `json:"annotations"` // Using interface{} for flexible data types
}

type Run struct {
	ID           string                 `json:"id"`
	Object       string                 `json:"object"`
	CreatedAt    int64                  `json:"created_at"`
	AssistantID  string                 `json:"assistant_id"`
	ThreadID     string                 `json:"thread_id"`
	Status       string                 `json:"status"`
	StartedAt    int64                  `json:"started_at"`
	ExpiresAt    *int64                 `json:"expires_at"`   // Nullable field
	CancelledAt  *int64                 `json:"cancelled_at"` // Nullable field
	FailedAt     *int64                 `json:"failed_at"`    // Nullable field
	CompletedAt  int64                  `json:"completed_at"`
	LastError    *string                `json:"last_error"` // Nullable field
	Model        string                 `json:"model"`
	Instructions *string                `json:"instructions"` // Nullable field
	Tools        []Tool                 `json:"tools"`
	FileIDs      []string               `json:"file_ids"`
	Metadata     map[string]interface{} `json:"metadata"`
	Usage        *interface{}           `json:"usage"` // Nullable field, assuming usage could be of any type
}

type Tool struct {
	Type string `json:"type"`
}

var apiKey string

func GetResponse(messageString string, threadId string, apiKey string) string {
	apiKey = os.Getenv("OPEN_AI_KEY")
	if apiKey == "" {
		log.Fatalf("could not read key")
		os.Exit(1)
	}
	// TODO move out
	assistantId := "asst_YZ9utNnMlf1973bcH5ND7Tf1"
	// listAssistants()
	// thread := startThread()
	fmt.Printf("thread id: %s\n", threadId)

	sendMessage(messageString, threadId, apiKey)
	initialRun := run(assistantId, threadId, apiKey)
	runId := initialRun.ID
	fmt.Printf("Run id: %s\n", initialRun.ID)
	fmt.Printf("Run status: %s\n", initialRun.Status)
	runStatus := ""
	runDelay := 1
	for runStatus != "completed" {
		run := getRun(threadId, runId, apiKey)
		// TODO: LOG
		fmt.Printf("Run status: %s\n", run.Status)
		runStatus = run.Status
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
	messageList := listMessages(threadId, apiKey)
	fmt.Println(threadId)
	fmt.Println(getFirstMessage(messageList))
	return getFirstMessage(messageList)
}

func StartThread(apiKey string) Thread {
  log.Printf("API_KEY: %s\n", apiKey)
	url := "https://api.openai.com/v1/threads"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("POST error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
	}
	fmt.Println(string(responseBody))

	var thread Thread
	if err := json.Unmarshal(responseBody, &thread); err != nil {
		fmt.Printf("Error unmarshl  %s\n", err)
	}

	return thread
}

func run(assistantId string, threadId string, apiKey string) Run {
	url := "https://api.openai.com/v1/threads/" + threadId + "/runs"

	reqData := RunReq{AssistantId: assistantId, Instructions: ""}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		fmt.Printf("Error Marshal %s\n", err)
	}
	fmt.Println(string(jsonData))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("POST error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
	}
	// fmt.Println(string(responseBody))
	var run Run
	if err := json.Unmarshal(responseBody, &run); err != nil {
		fmt.Printf("Error unmarshl  %s\n", err)
	}

	return run
}

func sendMessage(messageString string, threadId string, apiKey string) {
	url := "https://api.openai.com/v1/threads/" + threadId + "/messages"

	message := MessageReq{Role: "user", Content: messageString}
	jsonData, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("Error Marshal %s\n", err)
	}
	fmt.Println(string(jsonData))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("POST error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
	}
	fmt.Println(string(responseBody))
}

func getRun(threadId string, runId string, apiKey string) Run {
	url := "https://api.openai.com/v1/threads/" + threadId + "/runs/" + runId
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("GET error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
	}

	var run Run
	if err := json.Unmarshal(responseBody, &run); err != nil {
		fmt.Printf("Error unmarshl  %s\n", err)
	}

	return run
}

func ListAssistants(apiKey string) {
	url := "https://api.openai.com/v1/assistants"
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("GET error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
		return
	}
	fmt.Println("Response:", string(responseBody))
}

func listMessages(threadId string, apiKey string) MessageList {
	url := "https://api.openai.com/v1/threads/" + threadId + "/messages"
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("GET error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading resp %s", err)
	}
	var messageList MessageList
	if err := json.Unmarshal(responseBody, &messageList); err != nil {
		fmt.Printf("Error unmarshl  %s\n", err)
	}

	return messageList
}

func getFirstMessage(messageList MessageList) string {
	firstId := messageList.FirstID
	for _, message := range messageList.Data {
		fmt.Printf("Message ID: %s\n", message.ID)
		if message.ID == firstId {
			return message.Content[0].Text.Value
		}
	}
	return ""
}

func addHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("OpenAI-Beta", "assistants=v1")
}
