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
	// TODO: move this out should have a secret manager
	apiKey := os.Getenv("ALPHA_VANTAGE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("unable to get alpha vantage api key")
	}

	baseURL := "https://www.alphavantage.co/query"

	u, err := url.Parse(baseURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return "", err
	}

	q := u.Query()
	q.Set("function", "GLOBAL_QUOTE")
	q.Set("symbol", symbol)
	q.Set("apikey", apiKey)

	u.RawQuery = q.Encode()

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

func getWeather(location string) (string, error) {
	// TODO: move this out should have a secret manager
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("unable to get alpha vantage api key")
	}

	baseURL := "http://api.weatherapi.com/v1/forecast.json"

	u, err := url.Parse(baseURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return "", err
	}

	q := u.Query()
	q.Set("key", apiKey)
	q.Set("q", location)

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

	weatherData := WeatherData{}
	err = json.Unmarshal(body, &weatherData)
	if err != nil {
		log.Println("error unmashalling: ", string(body))
		return "", err
	}

	var forecastDays []ForecastDay
	for _, forcastDay := range weatherData.Forecast.Forecastday {
		forcastDay.Hour = []Hour{}
		forecastDays = append(forecastDays, forcastDay)
	}
	// dailyForecast := weatherData.Forecast.Forecastday[0]

	// dailyForecast.Hour = []Hour{}
	jsonForecast, err := json.Marshal(forecastDays)
	if err != nil {
		return "", err
	}
	log.Println("Forecast: ", string(jsonForecast))
	return fmt.Sprintf(
		"here is the json data only comment on the temperature in freedom units, precipitation, and condition: %s. Only comment on the requested day. If no day was specified by the user only give the details for today",
		string(jsonForecast),
	), nil
}

type WeatherData struct {
	Location Location `json:"location"`
	Current  Current  `json:"current"`
	Forecast Forecast `json:"forecast"`
}

type Location struct {
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	TzID           string  `json:"tz_id"`
	LocaltimeEpoch int64   `json:"localtime_epoch"`
	Localtime      string  `json:"localtime"`
}

type Condition struct {
	Text string `json:"text"`
	Icon string `json:"icon"`
	Code int    `json:"code"`
}

type Current struct {
	LastUpdatedEpoch int64     `json:"last_updated_epoch"`
	LastUpdated      string    `json:"last_updated"`
	TempC            float64   `json:"temp_c"`
	TempF            float64   `json:"temp_f"`
	IsDay            int       `json:"is_day"`
	Condition        Condition `json:"condition"`
	WindMph          float64   `json:"wind_mph"`
	WindKph          float64   `json:"wind_kph"`
	WindDegree       int       `json:"wind_degree"`
	WindDir          string    `json:"wind_dir"`
	PressureMb       float64   `json:"pressure_mb"`
	PressureIn       float64   `json:"pressure_in"`
	PrecipMm         float64   `json:"precip_mm"`
	PrecipIn         float64   `json:"precip_in"`
	Humidity         int       `json:"humidity"`
	Cloud            int       `json:"cloud"`
	FeelslikeC       float64   `json:"feelslike_c"`
	FeelslikeF       float64   `json:"feelslike_f"`
	WindchillC       float64   `json:"windchill_c"`
	WindchillF       float64   `json:"windchill_f"`
	HeatindexC       float64   `json:"heatindex_c"`
	HeatindexF       float64   `json:"heatindex_f"`
	DewpointC        float64   `json:"dewpoint_c"`
	DewpointF        float64   `json:"dewpoint_f"`
	VisKm            float64   `json:"vis_km"`
	VisMiles         float64   `json:"vis_miles"`
	UV               float64   `json:"uv"`
	GustMph          float64   `json:"gust_mph"`
	GustKph          float64   `json:"gust_kph"`
}

type DayCondition struct {
	Text string `json:"text"`
	Icon string `json:"icon"`
	Code int    `json:"code"`
}

type Day struct {
	MaxtempC          float64      `json:"maxtemp_c"`
	MaxtempF          float64      `json:"maxtemp_f"`
	MintempC          float64      `json:"mintemp_c"`
	MintempF          float64      `json:"mintemp_f"`
	AvgtempC          float64      `json:"avgtemp_c"`
	AvgtempF          float64      `json:"avgtemp_f"`
	MaxwindMph        float64      `json:"maxwind_mph"`
	MaxwindKph        float64      `json:"maxwind_kph"`
	TotalprecipMm     float64      `json:"totalprecip_mm"`
	TotalprecipIn     float64      `json:"totalprecip_in"`
	TotalsnowCm       float64      `json:"totalsnow_cm"`
	AvgvisKm          float64      `json:"avgvis_km"`
	AvgvisMiles       float64      `json:"avgvis_miles"`
	Avghumidity       int          `json:"avghumidity"`
	DailyWillItRain   int          `json:"daily_will_it_rain"`
	DailyChanceOfRain int          `json:"daily_chance_of_rain"`
	DailyWillItSnow   int          `json:"daily_will_it_snow"`
	DailyChanceOfSnow int          `json:"daily_chance_of_snow"`
	Condition         DayCondition `json:"condition"`
	UV                float64      `json:"uv"`
}

type Astro struct {
	Sunrise          string `json:"sunrise"`
	Sunset           string `json:"sunset"`
	Moonrise         string `json:"moonrise"`
	Moonset          string `json:"moonset"`
	MoonPhase        string `json:"moon_phase"`
	MoonIllumination int    `json:"moon_illumination"`
	IsMoonUp         int    `json:"is_moon_up"`
	IsSunUp          int    `json:"is_sun_up"`
}

type HourCondition struct {
	Text string `json:"text"`
	Icon string `json:"icon"`
	Code int    `json:"code"`
}

type Hour struct {
	TimeEpoch    int64         `json:"time_epoch"`
	Time         string        `json:"time"`
	TempC        float64       `json:"temp_c"`
	TempF        float64       `json:"temp_f"`
	IsDay        int           `json:"is_day"`
	Condition    HourCondition `json:"condition"`
	WindMph      float64       `json:"wind_mph"`
	WindKph      float64       `json:"wind_kph"`
	WindDegree   int           `json:"wind_degree"`
	WindDir      string        `json:"wind_dir"`
	PressureMb   float64       `json:"pressure_mb"`
	PressureIn   float64       `json:"pressure_in"`
	PrecipMm     float64       `json:"precip_mm"`
	PrecipIn     float64       `json:"precip_in"`
	SnowCm       float64       `json:"snow_cm"`
	Humidity     int           `json:"humidity"`
	Cloud        int           `json:"cloud"`
	FeelslikeC   float64       `json:"feelslike_c"`
	FeelslikeF   float64       `json:"feelslike_f"`
	WindchillC   float64       `json:"windchill_c"`
	WindchillF   float64       `json:"windchill_f"`
	HeatindexC   float64       `json:"heatindex_c"`
	HeatindexF   float64       `json:"heatindex_f"`
	DewpointC    float64       `json:"dewpoint_c"`
	DewpointF    float64       `json:"dewpoint_f"`
	WillItRain   int           `json:"will_it_rain"`
	ChanceOfRain int           `json:"chance_of_rain"`
	WillItSnow   int           `json:"will_it_snow"`
	ChanceOfSnow int           `json:"chance_of_snow"`
	VisKm        float64       `json:"vis_km"`
	VisMiles     float64       `json:"vis_miles"`
	GustMph      float64       `json:"gust_mph"`
	GustKph      float64       `json:"gust_kph"`
	UV           float64       `json:"uv"`
}

type ForecastDay struct {
	Date      string `json:"date"`
	DateEpoch int64  `json:"date_epoch"`
	Day       Day    `json:"day"`
	Astro     Astro  `json:"astro"`
	Hour      []Hour `json:"hour"`
}

type Forecast struct {
	Forecastday []ForecastDay `json:"forecastday"`
}
