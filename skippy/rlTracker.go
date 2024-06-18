package skippy

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
)

// PlayerStats represents the statistics for a single player.
type PlayerStats struct {
	Goals   int     `json:"goals"`
	Assists int     `json:"assists"`
	Saves   int     `json:"saves"`
	Shots   int     `json:"shots"`
	Demos   int     `json:"demos"`
	Score   int     `json:"score"`
	MMR     float64 `json:"mmr"`
}

// Game represents the structure of one Rocket League game, including team goals and individual player stats.
type Game struct {
	Timestamp string                 `json:"timestamp"`
	TeamColor string                 `json:"team_color"`
	TeamGoals int                    `json:"team_goals"`
	WinLoss   string                 `json:"win_loss"`
	HomeTeam  map[string]PlayerStats `json:"home_team"`
	AwayTeam  map[string]PlayerStats `json:"away_team"`
}

func StartRocketLeagueSession(
	ctx context.Context,
	filePath string,
	channelID string,
	dg *discordgo.Session,
	state *State,
	client *openai.Client,
) error {

	fileCh := make(chan string)

	if filePath != "" {
		log.Println("Starting rocket league session watching : ", filePath)
		interval := 5 * time.Second
		go WatchFolder(filePath, fileCh, interval)
	} else {
		log.Println("Could not read rocket league folder")
		return errors.New("receieved empty file path")
	}

	go func() {
		defer close(fileCh)
		for {
			select {
			case gameInfo := <-fileCh:
				log.Println("Received game info: ", gameInfo)
				messageReq := openai.MessageRequest{
					Role:    openai.ChatMessageRoleAssistant,
					Content: gameInfo,
				}
				getAndSendResponse(
					context.Background(),
					dg,
					channelID,
					messageReq,
					COMMENTATE_INSTRUCTIONS,
					client,
					state,
				)
			case <-ctx.Done():
				log.Println("Received cancel command")
				return
			}
		}
	}()

	return nil

}
func WatchFolder(filePath string, ch chan<- string, interval time.Duration) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Println("Error reading folder:", err)
		return
	}

	if !fileInfo.IsDir() {
		watchFile(filePath, ch, interval)
	}

	dir, err := os.Open(filePath)
	if err != nil {
		log.Println("Could not epen directory: ", filePath, err)
	}
	files, err := dir.ReadDir(0)
	if err != nil {
		log.Println("Unable to read dir")
		return
	}
	for _, file := range files {
		go watchFile(filePath+file.Name(), ch, interval)
	}
}

func watchFile(filePath string, ch chan<- string, interval time.Duration) {
	log.Println("Watching file: ", filePath)
	var lastModTime time.Time

	for {
		// Retrieve file info
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Println("Error checking file:", err)
			continue
		}

		modTime := fileInfo.ModTime()
		if lastModTime == (time.Time{}) {
			lastModTime = modTime
		}

		// Check if the modification time has changed
		if modTime.After(lastModTime) {
			log.Println("Detected file change: ", fileInfo.Name())
			lastModTime = modTime
			gameInfo := getGameData(filePath)
			if gameInfo != "" {
				ch <- gameInfo
			}
		}

		time.Sleep(interval)
	}
}

/*
assumes using RL Stat saver https://bakkesplugins.com/plugins/view/390
reads most recent game from the bottom up
*/
func getGameData(filePath string) string {
	if filepath.Ext(filePath) != ".csv" {
		log.Println("Attempted to read non csv file: ", filePath)
		return ""
	}
	file, err := os.Open(filePath)
	if err != nil {
		log.Println("Error could not open file", filePath)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)

	records, err := csvReader.ReadAll()
	if err != nil {
		log.Println("Could not read csv: ", filePath, err)
	}

	index := -1
	for i := len(records) - 1; i >= 0; i-- {
		if records[i][0] == "TEAM COLOR" {
			index = i
			break
		}
	}

	if index == -1 {
		log.Println("Could not find team data in csv: ", filePath)
		return ""
	}

	// only get last game and remove the unecessary cols
	return toJson(toGame(records[index:]))
}

func toJson(game Game) string {
	jsonData, err := json.MarshalIndent(game, "", "  ")
	if err != nil {
		log.Println("Unable to marshal game: ", err)
	}
	return string(jsonData)
}

func toGame(records [][]string) Game {
	game := Game{
		Timestamp: records[1][11],
		TeamGoals: atoi(records[1][9]),
		WinLoss:   records[1][10],
		HomeTeam:  make(map[string]PlayerStats),
		AwayTeam:  make(map[string]PlayerStats),
	}

	var homeTeamColor string
	for i, record := range records {
		if record[0] == "TEAM COLOR" {
			homeTeamColor = records[i+1][0]
			continue
		}

		playerStats := PlayerStats{
			Goals:   atoi(record[2]),
			Assists: atoi(record[3]),
			Saves:   atoi(record[4]),
			Shots:   atoi(record[5]),
			Demos:   atoi(record[6]),
			Score:   atoi(record[7]),
			MMR:     atof(record[8]),
		}

		if record[0] == homeTeamColor {
			game.HomeTeam[record[1]] = playerStats
		} else {
			game.AwayTeam[record[1]] = playerStats
		}
	}
	return game
}

//lint:ignore U1000 saving for later
func toCsv(records [][]string, startCol int, endCol int) string {
	var retVal string
	for _, record := range records {
		for i := startCol; i < endCol; i++ {
			retVal += record[i] + ","
		}
		retVal += "\n"
	}
	return retVal
}

func atoi(s string) int {
	s = strings.Trim(s, " ")
	val, err := strconv.Atoi(s)
	if err != nil {
		log.Println("Error parsing int: ", err)
	}
	return val
}

func atof(s string) float64 {
	s = strings.Trim(s, " ")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Fatal("Error parsing float: ", err)
	}
	return val
}
