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
}

func NewClient(apiKey string, assistantID string) *Client {
	return &Client{OpenAIApiKey: apiKey, AssistantID: assistantID}
}


func (c *Client) GetResponse(messageString string, threadID string, additionalInstructions string) string {
	sendMessage(messageString, threadID, c.OpenAIApiKey)
	initialRun := c.run(threadID, additionalInstructions)
	runId := initialRun.ID
	log.Println("Initial Run id: ", initialRun.ID)
	log.Println("Run status: ", initialRun.Status)
	runStatus := ""
	runDelay := 1
	for runStatus != "completed" {
		run := getRun(runId, threadID, c.OpenAIApiKey)
		log.Printf("Run status: %s\n", run.Status)
		runStatus = run.Status
		time.Sleep(time.Duration(100*runDelay) * time.Millisecond)
		runDelay++
	}
	messageList := listMessages(threadID, c.OpenAIApiKey)
	log.Println("Recieived message from thread: ", threadID)
	log.Println(getFirstMessage(messageList))
	return getFirstMessage(messageList)
}

func (c *Client) StartThread() Thread {
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
	log.Println("Successfully started thread: ", thread.ID)
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

func (c *Client) run(threadID string, additionalInstructions string) Run {
	url := "https://api.openai.com/v1/threads/" + threadID + "/runs"

	reqData := RunReq{AssistantId: c.AssistantID, Instructions: "", AdditionalInstructions: additionalInstructions}
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
