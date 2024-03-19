package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Client struct {
	OpenAIApiKey  string
	AssistantID   string
	ThreadID      string
  AdditionalInstructions string
	Ticker        *time.Ticker
	ThreadTimeout int
}

func NewClient(apiKey string, assistantID string) *Client {
	return &Client{OpenAIApiKey: apiKey, AssistantID: assistantID, Ticker: time.NewTicker(30*time.Minute), ThreadTimeout: 30}
}

func (c *Client) UpdateAdditionalInstructions(instructions string) {
  c.AdditionalInstructions = instructions
}
func (c *Client) Close() {
  log.Println("Recieved close command. Stopping ticker")
  c.Ticker.Stop()
}

func (c *Client) GetResponse(messageString string) string {
	if c.ThreadID == "" {
		log.Println("Tried to get response without thread set. Starting new thread")
		c.ThreadID = c.StartThread().ID
	}

	go func() {
		for range c.Ticker.C {
			log.Println("Recieved tick. Starting new thread.")
			c.ThreadID = c.StartThread().ID
		}
	}()

	sendMessage(messageString, c.ThreadID, c.OpenAIApiKey)
	initialRun := c.run()
	runId := initialRun.ID
	log.Println("Initial Run id: ", initialRun.ID)
	log.Println("Run status: ", initialRun.Status)
	runStatus := ""
	runDelay := 1
	for runStatus != "completed" {
		run := getRun(runId, c.ThreadID, c.OpenAIApiKey)
		log.Printf("Run status: %s\n", run.Status)
		runStatus = run.Status
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
	messageList := listMessages(c.ThreadID, c.OpenAIApiKey)
	log.Println("Recieived message from thread: ", c.ThreadID)
	log.Println(getFirstMessage(messageList))
	return getFirstMessage(messageList)
}

func (c *Client) StartThread() Thread {
	// reset ticker whenever we start a new thread
	c.Ticker.Reset(time.Duration(c.ThreadTimeout) * time.Minute)
	url := "https://api.openai.com/v1/threads"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
	addHeaders(req, c.OpenAIApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("POST error %s\n", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading resp %s\n", err)
	}

	var thread Thread
	if err := json.Unmarshal(responseBody, &thread); err != nil {
		log.Printf("Error unmarshalling thread:  %s\n", err)
	}
	log.Println("Successfully start thread: ", thread.ID)
	return thread
}

func (c *Client) ListAssistants() {
	url := "https://api.openai.com/v1/assistants"
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, c.OpenAIApiKey)
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

func (c *Client) run() Run {
	url := "https://api.openai.com/v1/threads/" + c.ThreadID + "/runs"

  log.Println(" add instructions: " , c.AdditionalInstructions)
	reqData := RunReq{AssistantId: c.AssistantID, Instructions: "", AdditionalInstructions: c.AdditionalInstructions}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		log.Println("Error Marshalling: ", err)
	}
  log.Println("Request Data: ", string(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	addHeaders(req, c.OpenAIApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("POST error: ", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading resp: ", err)
	}
	var run Run
	if err := json.Unmarshal(responseBody, &run); err != nil {
		log.Println("Error unmarshalling: ", err)
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

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	addHeaders(req, apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("POST error: ", err)
	}

	defer resp.Body.Close()

}

func getRun(runId string, threadId string, apiKey string) Run {
	url := "https://api.openai.com/v1/threads/" + threadId + "/runs/" + runId
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("GET error: ", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading resp: ", err)
	}

	var run Run
	if err := json.Unmarshal(responseBody, &run); err != nil {
		log.Println("Error unmarshalling:", err)
	}

	return run
}

func listMessages(threadId string, apiKey string) MessageList {
	url := "https://api.openai.com/v1/threads/" + threadId + "/messages"
	req, err := http.NewRequest("GET", url, nil)

	addHeaders(req, apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("GET error %s", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading resp: ", err)
	}

	var messageList MessageList
	if err := json.Unmarshal(responseBody, &messageList); err != nil {
		log.Println("Error unmarshalling: ", err)
	}

	return messageList
}

func getFirstMessage(messageList MessageList) string {
	firstId := messageList.FirstID
	for _, message := range messageList.Data {
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
