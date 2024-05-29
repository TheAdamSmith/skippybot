package discord

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

// https://www.alphavantage.co/documentation/
type GlobalQuote struct {
	Symbol           string `json:"01. symbol"`
	Open             string `json:"02. open"`
	High             string `json:"03. high"`
	Low              string `json:"04. low"`
	Price            string `json:"05. price"`
	Volume           string `json:"06. volume"`
	LatestTradingDay string `json:"07. latest trading day"`
	PreviousClose    string `json:"08. previous close"`
	Change           string `json:"09. change"`
	ChangePercent    string `json:"10. change percent"`
}

type ApiResponse struct {
	GlobalQuote GlobalQuote `json:"Global Quote"`
}

func getStockPrice(symbol string) (string, error) {
	apiKey := os.Getenv("ALPHA_VANTAGE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("unable to get alpha vantage api key")
	}
	// Define the base URL
	baseURL := "https://www.alphavantage.co/query"

	// Parse the base URL
	u, err := url.Parse(baseURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return "", err
	}

	// Create a URL values object and add the query parameters
	q := u.Query()
	q.Set("function", "GLOBAL_QUOTE")
	q.Set("symbol", symbol)
	q.Set("apikey", apiKey)

	u.RawQuery = q.Encode()

	log.Println("Final URL:", u.String())

	response, err := http.Get(u.String())
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}

	apiResponse := ApiResponse{}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Println("error unmashalling: ", string(body))
		return "", err
	}
	log.Println("The price is: ", apiResponse.GlobalQuote.Price)
	return apiResponse.GlobalQuote.Price, nil
}
