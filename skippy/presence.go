package skippy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func OnPresenceUpdateDebounce(
	p *discordgo.PresenceUpdate,
	debouncer *Debouncer,
	s *Skippy,
) {
	debouncer.Debounce(p.User.ID, func() {
		OnPresenceUpdate(p, s)
	})
}

func OnPresenceUpdate(p *discordgo.PresenceUpdate, s *Skippy) {
	if _, exists := s.Config.UserConfigMap[p.User.ID]; !exists {
		return
	}

	game, isPlayingGame := getCurrentGame(p)
	isPlayingGame = isPlayingGame && game != ""

	userPresence, exists := s.State.GetPresence(p.User.ID)

	member, err := s.DiscordSession.GetState().Member(p.GuildID, p.User.ID)
	if err != nil {
		log.Println("Could not get user: ", err)
		return
	}

	startedPlaying := isPlayingGame &&
		(!exists || exists && !userPresence.IsPlayingGame)
	stoppedPlaying := exists && userPresence.IsPlayingGame && !isPlayingGame

	if startedPlaying {
		log.Printf("User %s started playing %s\n", member.User.Username, game)
		s.State.UpdatePresence(p.User.ID, WithStatus(p.Status), WithIsPlayingGame(isPlayingGame), WithGame(game), WithTimeStarted(time.Now()))
	}

	if stoppedPlaying {
		duration := time.Since(userPresence.TimeStarted)
		if duration < s.Config.MinGameSessionDuration {
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

		err = s.DB.CreateGameSession(userSession)
		if err != nil {
			log.Println("Unable to create game session: ", err)
		}

		s.State.UpdatePresence(p.User.ID, WithStatus(p.Status), WithIsPlayingGame(isPlayingGame), WithGame(""), WithTimeStarted(time.Time{}))
	}
}

func PollPresenceStatus(ctx context.Context, s *Skippy) {
	now := time.Now()
	for userID, userConfig := range s.Config.UserConfigMap {
		totTime := time.Duration(0)
		presence, exists := s.State.GetPresence(userID)
		if exists && now.Sub(presence.LastLimitReminder) < 24*time.Hour || !userConfig.Remind {
			continue
		}

		if exists && presence.IsPlayingGame {
			totTime = totTime + now.Sub(presence.TimeStarted)
		}

		storedDuration, err := s.DB.GetGameSessionSum(userID, 0)
		if err != nil {
			log.Println("could not get sum from database", err)
		}

		totTime = totTime + storedDuration
		if totTime > userConfig.DailyLimit {
			channelID := s.Config.UserConfigMap[userID].LimitReminderChannelID
			if channelID == "" {
				channel, err := s.DiscordSession.UserChannelCreate(userID)
				if err != nil {
					log.Println("could not create user channel", err)
					continue
				}
				channelID = channel.ID
			}

			log.Printf("User (%s) hit limit. Attempting to send reminder on %s.\n", userID, channelID)

			sessions, err := s.DB.GetGameSessionsByUserAndDays(userID, 0)
			if err != nil {
				log.Println("Unable to get game sessions: ", err)
				continue
			}

			aiGameSessions := ToGameSessionAI(sessions)

			if exists && presence.IsPlayingGame {
				aiGameSessions = append(aiGameSessions, GameSessionAI{
					Game:       presence.Game,
					StartedAt:  presence.TimeStarted,
					TimePlayed: time.Since(presence.TimeStarted).String(),
				})
			}

			content := ""
			if aiGameSessions == nil || len(aiGameSessions) == 0 {
				log.Println("found user over limit without any game sessions. continuing")
				continue
			} else {
				jsonData, err := json.Marshal(aiGameSessions)
				if err != nil {
					log.Println("Unable to marshal json: ", err)
					continue
				}
				content = string(jsonData)
			}

			err = getAndSendResponse(
				ctx,
				s,
				ResponseReq{
					ChannelID:              channelID,
					Message:                content,
					AdditionalInstructions: fmt.Sprintf(GAME_LIMIT_REMINDER_INSTRUCTIONS_FORMAT, UserMention(userID)),
					DisableTools:           true,
				},
			)
			if err != nil {
				log.Println("could not send response", err)
				continue
			}
			s.State.UpdatePresence(userID, WithLastLimitReminder(now))
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
