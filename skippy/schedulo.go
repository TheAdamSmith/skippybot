package skippy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"skippybot/components"
	"sort"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
)

type EventScheduler struct {
	NumResponses     int
	EventDescription string
	ResponseMap      map[string][]time.Time
}

type WhensGoodForm struct {
	UserIDS   []string
	Game      string
	Date      time.Time
	StartTime time.Time
	ChannelID string
	EndTime   time.Time
}

// TODO: doc
func generateWhensGoodResponse(initialInteraction *discordgo.InteractionCreate, s *Skippy) *discordgo.InteractionResponseData {
	now := time.Now().Truncate(time.Hour)
	formData := &WhensGoodForm{
		ChannelID: initialInteraction.ChannelID,
		Date:      now,
		StartTime: now,
		EndTime:   now.Add(5 * time.Hour),
	}

	optionValue, ok := findCommandOption(
		initialInteraction.ApplicationCommandData().Options,
		GAME,
	)
	if ok {
		formData.Game = optionValue.StringValue()
	} else {
		formData.Game = "chillin"
	}

	userSelect := s.ComponentHandler.SelectMenu(
		discordgo.SelectMenu{
			MenuType:    discordgo.SelectMenuType(discordgo.UserSelectMenuComponent),
			Placeholder: "Select users",
			MaxValues:   25,
		},
		func(i *discordgo.InteractionCreate) {
			formData.UserIDS = i.MessageComponentData().Values
		})

	dateSelect := s.ComponentHandler.SelectMenu(
		discordgo.SelectMenu{
			Placeholder: "Start Time",
			MaxValues:   1,
			Options:     getDateOptions(),
		},
		func(i *discordgo.InteractionCreate) {
			if t, err := time.Parse(time.RFC3339, i.MessageComponentData().Values[0]); err != nil {
				log.Println("error parsing time: ", err)
			} else {
				formData.Date = t
			}
		})

	startSelect := s.ComponentHandler.SelectMenu(
		discordgo.SelectMenu{
			Placeholder: "Start Time",
			MaxValues:   1,
			Options:     getTimeOptions(1 * time.Hour),
		},
		func(i *discordgo.InteractionCreate) {
			if t, err := time.Parse(time.RFC3339, i.MessageComponentData().Values[0]); err != nil {
				log.Println("error parsing time: ", err)
			} else {
				formData.StartTime = t
			}
		})

	endSelect := s.ComponentHandler.SelectMenu(
		discordgo.SelectMenu{
			Placeholder: "End Time",
			MaxValues:   1,
			Options:     getTimeOptions(1 * time.Hour),
		},
		func(i *discordgo.InteractionCreate) {
			if t, err := time.Parse(time.RFC3339, i.MessageComponentData().Values[0]); err != nil {
				log.Println("error parsing time: ", err)
			} else {
				formData.EndTime = t
			}
		})


	button := s.ComponentHandler.WithSubmitButton(
		discordgo.Button{
			Label: "Send",
			Style: discordgo.PrimaryButton,
		},
		func(i *discordgo.InteractionCreate) {
			formData.StartTime = combineDateTime(formData.Date, formData.StartTime)
			formData.EndTime = combineDateTime(formData.Date, formData.EndTime)

			newContent := "Sending message..."
			if _, err := s.DiscordSession.InteractionResponseEdit(initialInteraction.Interaction, &discordgo.WebhookEdit{
				Content:    &newContent,
				Components: &[]discordgo.MessageComponent{},
			}); err != nil {
				log.Println("error updating message: ", err)
			}
			getUserAvailability(initialInteraction, formData, s)
		},
	)

	buttonRow := components.ButtonRow(s.DiscordSession, button)

	return &discordgo.InteractionResponseData{
		Flags: discordgo.MessageFlagsEphemeral,
		Components: []discordgo.MessageComponent{
			userSelect,
			dateSelect,
			startSelect,
			endSelect,
			buttonRow,
		},
	}
}

func getUserAvailability(initialInteraction *discordgo.InteractionCreate, formData *WhensGoodForm, s *Skippy) {
	var timeOptions []discordgo.SelectMenuOption
	var dur time.Duration

	if formData.EndTime.Hour()-formData.StartTime.Hour() < 12 {
		dur = 30 * time.Minute
	} else {
		dur = time.Hour
	}

	for t := formData.StartTime; t.Before(formData.EndTime); t = t.Add(dur) {
		option := discordgo.SelectMenuOption{
			Label: t.Format("3:04 PM"),
			Value: t.Format(time.RFC3339),
		}

		timeOptions = append(timeOptions, option)
	}

	userAvailability := make(map[string][]time.Time)
	var respErr error
	var message *discordgo.Message

	onSelect := func(i *discordgo.InteractionCreate) {
		var timeSlots []time.Time
		for _, s := range i.MessageComponentData().Values {
			if t, err := time.Parse(time.RFC3339, s); err != nil {
				log.Println("error parsing time: ", err)
			} else {
				timeSlots = append(timeSlots, t)
			}
		}

		userAvailability[i.Member.User.ID] = timeSlots

		message, respErr = sendUserAvailabilityResponse(i, message, userAvailability, formData, s)
		if respErr != nil {
			log.Println("error sending user availability response", respErr)
		}
	}

	zero := 0
	timeSelect := s.ComponentHandler.SelectMenu(
		discordgo.SelectMenu{
			Placeholder: "When you on?",
			MinValues:   &zero,
			MaxValues:   len(timeOptions),
			Options:     timeOptions,
		}, onSelect)

	cantButton := s.ComponentHandler.WithButton(
		discordgo.Button{
			Style: discordgo.DangerButton,
			Label: "Can't",
		}, func(i *discordgo.InteractionCreate) {
			userAvailability[i.Member.User.ID] = []time.Time{}
			message, respErr = sendUserAvailabilityResponse(i, message, userAvailability, formData, s)
			if respErr != nil {
				log.Println("error sending user availability response", respErr)
			}
		})
	buttonRow := components.ButtonRow(s.DiscordSession, cantButton)

	content, err := getUserAvailabilityContent(initialInteraction, formData, s)

	if err != nil {
		log.Println("error generating content for user availability message", err)
	}

	if formData.StartTime.Weekday() == time.Now().Weekday() {
		content += "\n## Availability Today:"
	} else {
		content += "\n## Availability" + formData.StartTime.Weekday().String() + ":"
	}

	_, err = s.DiscordSession.ChannelMessageSendComplex(formData.ChannelID, &discordgo.MessageSend{
		Content: content,
		Components: []discordgo.MessageComponent{
			timeSelect,
			buttonRow,
		},
	})
	if err != nil {
		log.Println("error sending channel message", err)
	}
}

func getUserAvailabilityContent(initialInteraction *discordgo.InteractionCreate, formData *WhensGoodForm, s *Skippy) (string, error) {
	instructions := makeTimeSelectInstructions(formData, initialInteraction)
	response, err := GetResponse(
		context.Background(),
		s,
		ResponseReq{
			ChannelID:    initialInteraction.ID,
			AdditionalInstructions:     	 instructions,
			DisableTools: true,
		},
	)
	if err != nil {
		return "", err
	}

	return response, nil
}

func sendUserAvailabilityResponse(
	i *discordgo.InteractionCreate,
	message *discordgo.Message,
	userAvailability map[string][]time.Time,
	formData *WhensGoodForm,
	s *Skippy,
) (*discordgo.Message, error) {
	commonTimes, userTimeMap := findCommonTimes(userAvailability, 3)

	title := "Today"
	if formData.StartTime.Weekday() != time.Now().Weekday() {
		title = formData.StartTime.Weekday().String()
	}

	messageEmbed := &discordgo.MessageEmbed{
		Title:  title,
		Fields: getUserAvailabilityFields(userAvailability, commonTimes),
	}

	// a bit confusing because MassegeEdit returns nil
	// we only need to get the message once to get the id
	if message == nil {
		message, err := s.DiscordSession.ChannelMessageSendEmbed(i.ChannelID, messageEmbed)

		return message, err
	}

	var buttons []discordgo.Button
	for _, t := range commonTimes {
		if t.IsZero() {
			continue
		}

		buttons = append(buttons, s.ComponentHandler.WithSubmitButton(discordgo.Button{
			Label: fmt.Sprintf("Create event for %s🚀", t.Format("3:04 PM MST")),
		}, func(i *discordgo.InteractionCreate) {
			generateAndScheduleEvent(i, formData.Game, userTimeMap[t], t, s)
		}))
	}

	var messageComponents []discordgo.MessageComponent
	if len(buttons) > 0 {
		eventButtonRow := components.ButtonRow(s.DiscordSession, buttons...)
		messageComponents = append(messageComponents, eventButtonRow)
	}

	_, err := s.DiscordSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    i.ChannelID,
		ID:         message.ID,
		Embed:      messageEmbed,
		Components: &messageComponents,
	})

	// if we are editing just return the passed in message
	return message, err
}

func generateAndScheduleEvent(i *discordgo.InteractionCreate, activityName string, availableUserIDs []string, t time.Time, s *Skippy) {
	if t.Before(time.Now()) {
		t = time.Now().Add(5 * time.Minute)
	}

	content := fmt.Sprintf("activity: %s\n time: %s\n users: ", activityName, t.Format("3:04 PM"))
	for _, userID := range availableUserIDs {
		content += UserMention(userID)
	}

	funcArgs, err := GetResponse(context.Background(), s, ResponseReq{
		ChannelID:        i.ChannelID,
		AdditionalInstructions:          content,
		Tools:            []openai.Tool{GenerateEventTool},
		RequireTools:     true,
		ReturnToolOutput: true,
	})
	if err != nil {
		log.Println("could not generate event data", err)
		s.DiscordSession.ChannelMessageSend(i.ChannelID, ERROR_RESPONSE)
		return
	}

	var eventFuncArgs EventFuncArgs
	if err := json.Unmarshal([]byte(funcArgs), &eventFuncArgs); err != nil {
		log.Println("could not read eventFuncArgs", err)
		s.DiscordSession.ChannelMessageSend(i.ChannelID, ERROR_RESPONSE)
		return
	}

	if event, err := scheduleEvent(s.DiscordSession, i.GuildID, eventFuncArgs.Name, eventFuncArgs.Description, t); err != nil {
		log.Println("unabled to schedule event", err)
		s.DiscordSession.ChannelMessageSend(i.ChannelID, ERROR_RESPONSE)
	} else {
		message := eventFuncArgs.NotificationMessage + fmt.Sprintf("\nhttps://discord.com/events/%s/%s", event.GuildID, event.ID)
		s.DiscordSession.ChannelMessageSend(i.ChannelID, message)
	}
}

func getUserAvailabilityFields(userAvailability map[string][]time.Time, commonTimes []time.Time) []*discordgo.MessageEmbedField {
	var fields []*discordgo.MessageEmbedField
	for userID, timeSlots := range userAvailability {
		content := fmt.Sprintf("%s \n", UserMention(userID))

		if len(timeSlots) == 0 {
			content = fmt.Sprintf("%s 🚫...😡\n ", content)
		}

		for _, timeSlot := range timeSlots {
			isCommonTime := false
			for _, t := range commonTimes {
				if timeSlot.Equal(t) {
					isCommonTime = true
				}
			}
			if isCommonTime {
				content = content + fmt.Sprintf("- **%s** \n", timeSlot.Format("3:04 PM MST"))
			} else {
				content = content + fmt.Sprintf("- %s \n", timeSlot.Format("3:04 PM MST"))
			}
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   " ", // space is needed because this field can't be blank
			Value:  content,
			Inline: true,
		})
	}

	return fields
}

func findCommonTimes(userAvailability map[string][]time.Time, topN int) ([]time.Time, map[time.Time][]string) {
	timeMap := make(map[time.Time][]string)
	for userID, timeSlots := range userAvailability {
		for _, timeSlot := range timeSlots {
			timeMap[timeSlot] = append(timeMap[timeSlot], userID)
		}
	}

	var timeList []time.Time
	for timeSlot := range timeMap {
		timeList = append(timeList, timeSlot)
	}

	sort.Slice(timeList, func(i, j int) bool {
		return len(timeMap[timeList[i]]) > len(timeMap[timeList[j]])
	})

	topTimes := []time.Time{}
	for i, t := range timeList {
		if i >= topN {
			break
		}
		if len(timeMap[timeList[i]]) > 1 {
			topTimes = append(topTimes, t)
		}
	}

	commonTimes := make(map[time.Time][]string)
	for _, t := range topTimes {
		commonTimes[t] = timeMap[t]
	}

	return topTimes, commonTimes
}

// TODO: get server config and pass in timezone
func getTimeOptions(d time.Duration) []discordgo.SelectMenuOption {
	startTime := time.Now().Truncate(time.Hour)
	var timeOptions []discordgo.SelectMenuOption

	for t := startTime; t.Before(startTime.Add(24 * time.Hour)); t = t.Add(d) {
		option := discordgo.SelectMenuOption{
			Label: t.Format("3:04 PM"),
			Value: t.Format(time.RFC3339),
		}

		timeOptions = append(timeOptions, option)
	}

	return timeOptions
}

// TODO: timezone
func getDateOptions() []discordgo.SelectMenuOption {
	t := time.Now()
	dateOptions := []discordgo.SelectMenuOption{
		{
			Label:   "Today",
			Value:   t.Format(time.RFC3339),
			Default: true,
		},
	}

	for i := 1; i < 7; i++ {
		t = t.Add(24 * time.Hour)
		option := discordgo.SelectMenuOption{
			Label: t.Weekday().String(),
			Value: t.Format(time.RFC3339),
		}

		dateOptions = append(dateOptions, option)
	}

	return dateOptions
}

func makeTimeSelectInstructions(formData *WhensGoodForm, i *discordgo.InteractionCreate) string {
	var users string
	for _, userID := range formData.UserIDS {
		if userID != i.Member.User.ID {
			users += fmt.Sprintf(",%s", UserMention(userID))
		}
	}
	if len(formData.UserIDS) == 0 {
		users = "anyone"
	}
	day := "Today"
	if formData.StartTime.Weekday() != time.Now().Weekday() {
		day = formData.StartTime.Weekday().String()
	}
	return fmt.Sprintf(`
		A user is asking whens good for %s on %s with an interactive discord message.
		The mention string of the user asking: %s
		The mention strings for the users they are scheduling with: %s	
		Please generate a message telling the users this an make sure to include all mention strings
		generate only the content don't respond directly to this prompt
		`,
		formData.Game,
		day,
		i.Member.User.Mention(),
		users,
	)
}

func combineDateTime(date, timeVal time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), timeVal.Hour(), timeVal.Minute(), 0, 0, timeVal.Location())
}
