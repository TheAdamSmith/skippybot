package skippy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

func onPresenceUpdateDebounce(
	dg DiscordSession,
	p *discordgo.PresenceUpdate,
	s *State,
	db Database,
	debouncer *Debouncer,
	config *Config,
) {
	debouncer.Debounce(p.User.ID, func() {
		onPresenceUpdate(dg, p, s, db, config)
	})
}

func onPresenceUpdate(
	dg DiscordSession,
	p *discordgo.PresenceUpdate,
	s *State,
	db Database,
	config *Config,
) {
	if _, exists := config.UserConfigMap[p.User.ID]; !exists {
		return
	}

	game, isPlayingGame := getCurrentGame(p)
	isPlayingGame = isPlayingGame && game != ""

	userPresence, exists := s.GetPresence(p.User.ID)

	member, err := dg.GetState().Member(p.GuildID, p.User.ID)
	if err != nil {
		log.Println("Could not get user: ", err)
		return
	}

	startedPlaying := isPlayingGame &&
		(!exists || exists && !userPresence.IsPlayingGame)
	stoppedPlaying := exists && userPresence.IsPlayingGame && !isPlayingGame

	if startedPlaying {
		log.Printf("User %s started playing %s\n", member.User.Username, game)
		s.UpdatePresence(p.User.ID, WithStatus(p.Status), WithIsPlayingGame(isPlayingGame), WithGame(game), WithTimeStarted(time.Now()))
	}

	if stoppedPlaying {
		duration := time.Since(userPresence.TimeStarted)
		if duration < config.MinGameSessionDuration {
			return
		}

		log.Printf(
			"User %s stopped playing game %s, after %s\n",
			member.User.Username,
			userPresence.Game,
			duration,
		)

		userSession := &GameSession{
			UserID:    p.User.ID,
			Game:      userPresence.Game,
			StartedAt: userPresence.TimeStarted,
			Duration:  duration,
		}

		err = db.CreateGameSession(userSession)
		if err != nil {
			log.Println("Unable to create game session: ", err)
		}

		s.UpdatePresence(p.User.ID, WithStatus(p.Status), WithIsPlayingGame(isPlayingGame), WithGame(""), WithTimeStarted(time.Time{}))
	}
}

func pollPresenceStatus(
	ctx context.Context,
	dg DiscordSession,
	client *openai.Client,
	state *State,
	db Database,
	config *Config,
) {
	now := time.Now()
	for userID, userConfig := range config.UserConfigMap {
		totTime := time.Duration(0)
		presence, exists := state.GetPresence(userID)
		if exists && now.Sub(presence.LastLimitReminder) < 24*time.Hour || !userConfig.Remind {
			continue
		}

		if exists && presence.IsPlayingGame {
			totTime = totTime + now.Sub(presence.TimeStarted)
		}

		storedDuration, err := db.GetGameSessionSum(userID, 0)
		if err != nil {
			log.Println("could not get sum from database", err)
		}

		totTime = totTime + storedDuration
		if totTime > userConfig.DailyLimit {
			channelID := config.UserConfigMap[userID].LimitReminderChannelID
			if channelID == "" {
				channel, err := dg.UserChannelCreate(userID)
				if err != nil {
					log.Println("could not create user channel", err)
					continue
				}
				channelID = channel.ID
			}

			log.Printf("User (%s) hit limit. Attempting to send reminder on %s.\n", userID, channelID)

			sessions, err := db.GetGameSessionsByUserAndDays(userID, 0)
			if err != nil {
				log.Println("Unable to get game sessions: ", err)
				continue
			}

			aiGameSessions := ToGameSessionAI(sessions)

			if exists && presence.IsPlayingGame {
				aiGameSessions = append(aiGameSessions, GameSessionAI{
					Game:       presence.Game,
					StartedAt:  presence.TimeStarted,
					TimePlayed: time.Now().Sub(presence.TimeStarted).String(),
				})
			}

			content := ""
			if aiGameSessions == nil {
				content = "Please respond saying that there were no games found for this user"
			} else {
				jsonData, err := json.Marshal(aiGameSessions)
				if err != nil {
					log.Println("Unable to marshal json: ", err)
					continue
				}
				content = string(jsonData)
			}
			messageReq := openai.MessageRequest{
				Role:    openai.ChatMessageRoleAssistant,
				Content: content,
			}

			err = getAndSendResponseWithoutTools(
				ctx,
				dg,
				channelID,
				messageReq,
				fmt.Sprintf(GAME_LIMIT_REMINDER_INSTRUCTIONS_FORMAT, UserMention(userID)),
				client,
				state,
			)
			if err != nil {
				log.Println("could not send response", err)
				continue
			}
			state.UpdatePresence(userID, WithLastLimitReminder(now))
		}
	}
}

func getCurrentGame(p *discordgo.PresenceUpdate) (string, bool) {
	for _, activity := range p.Activities {
		if activity.Type == discordgo.ActivityTypeGame {
			return activity.Name, true
		}
	}
	return "", false
}
