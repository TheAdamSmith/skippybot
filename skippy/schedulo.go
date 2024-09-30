package skippy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"skippybot/components"
	"sort"
	"strconv"
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
func generateWhensGoodResponse(
	dg DiscordSession,
	state *State,
	client *openai.Client,
	initialInteraction *discordgo.InteractionCreate,
) *discordgo.InteractionResponseData {
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

	userSelect := state.componentHandler.SelectMenu(
		discordgo.SelectMenu{
			MenuType:    discordgo.SelectMenuType(discordgo.UserSelectMenuComponent),
			Placeholder: "Select users",
			MaxValues:   25,
		},
		func(i *discordgo.InteractionCreate) {
			formData.UserIDS = i.MessageComponentData().Values
		})

	dateSelect := state.componentHandler.SelectMenu(
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

	startSelect := state.componentHandler.SelectMenu(
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

	endSelect := state.componentHandler.SelectMenu(
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

	var intOptions []discordgo.SelectMenuOption
	for i := 1; i <= 6; i++ {
		intOptions = append(intOptions,
			discordgo.SelectMenuOption{
				Label: strconv.Itoa(i),
				Value: strconv.Itoa(i),
			},
		)
	}

	button := state.componentHandler.WithSubmitButton(
		discordgo.Button{
			Label: "Send",
			Style: discordgo.PrimaryButton,
		},
		func(i *discordgo.InteractionCreate) {
			formData.StartTime = combineDateTime(formData.Date, formData.StartTime)
			formData.EndTime = combineDateTime(formData.Date, formData.EndTime)

			newContent := "Sending message..."
			if _, err := dg.InteractionResponseEdit(initialInteraction.Interaction, &discordgo.WebhookEdit{
				Content:    &newContent,
				Components: &[]discordgo.MessageComponent{},
			}); err != nil {
				log.Println("error updating message: ", err)
			}
			getUserAvailability(dg, state, client, initialInteraction, formData)
		},
	)

	buttonRow := components.ButtonRow(dg, button)

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

func getUserAvailability(
	dg DiscordSession,
	state *State,
	client *openai.Client,
	initialInteraction *discordgo.InteractionCreate,
	formData *WhensGoodForm,
) {
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

		message, respErr = sendUserAvailabilityResponse(dg, state, client, i, message, userAvailability, formData)
		if respErr != nil {
			log.Println("error sending user availability response", respErr)
		}
	}

	zero := 0
	timeSelect := state.componentHandler.SelectMenu(
		discordgo.SelectMenu{
			Placeholder: "When you on?",
			MinValues:   &zero,
			MaxValues:   len(timeOptions),
			Options:     timeOptions,
		}, onSelect)

	cantButton := state.componentHandler.WithButton(
		discordgo.Button{
			Style: discordgo.DangerButton,
			Label: "Can't",
		}, func(i *discordgo.InteractionCreate) {
			userAvailability[i.Member.User.ID] = []time.Time{}
			message, respErr = sendUserAvailabilityResponse(dg, state, client, i, message, userAvailability, formData)
			if respErr != nil {
				log.Println("error sending user availability response", respErr)
			}
		})
	buttonRow := components.ButtonRow(dg, cantButton)

	content, err := getUserAvailabilityContent(dg, state, client, initialInteraction, formData)
	if err != nil {
		log.Println("error generating content for user availability message", err)
	}

	if formData.StartTime.Weekday() == time.Now().Weekday() {
		content += "\n## Availability Today:"
	} else {
		content += "\n## Availability" + formData.StartTime.Weekday().String() + ":"
	}

	dg.ChannelMessageSendComplex(formData.ChannelID, &discordgo.MessageSend{
		Content: content,
		Components: []discordgo.MessageComponent{
			timeSelect,
			buttonRow,
		},
	})
}

func getUserAvailabilityContent(dg DiscordSession, state *State, client *openai.Client, initialInteraction *discordgo.InteractionCreate, formData *WhensGoodForm) (string, error) {
	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: makeTimeSelectInstructions(formData, initialInteraction),
	}

	response, err := GetResponse(
		context.Background(),
		dg,
		initialInteraction.ChannelID,
		messageReq,
		"",
		true,
		client,
		state,
		nil,
		nil,
	)
	if err != nil {
		return "", err
	}

	return response, nil
}

func sendUserAvailabilityResponse(
	dg DiscordSession,
	state *State,
	client *openai.Client,
	i *discordgo.InteractionCreate,
	message *discordgo.Message,
	userAvailability map[string][]time.Time,
	formData *WhensGoodForm,
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
		message, err := dg.ChannelMessageSendEmbed(i.ChannelID, messageEmbed)

		return message, err
	}

	var buttons []discordgo.Button
	for _, t := range commonTimes {
		if t.IsZero() {
			continue
		}

		buttons = append(buttons, state.componentHandler.WithSubmitButton(discordgo.Button{
			Label: fmt.Sprintf("Create event for %s🚀", t.Format("3:04 PM MST")),
		}, func(i *discordgo.InteractionCreate) {
			generateAndScheduleEvent(dg, state, client, i, formData.Game, userTimeMap[t], t)
		}))
	}

	var messageComponents []discordgo.MessageComponent
	if len(buttons) > 0 {
		eventButtonRow := components.ButtonRow(dg, buttons...)
		messageComponents = append(messageComponents, eventButtonRow)
	}

	_, err := dg.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    i.ChannelID,
		ID:         message.ID,
		Embed:      messageEmbed,
		Components: &messageComponents,
	})

	// if we are editing just return the passed in message
	return message, err
}

func generateAndScheduleEvent(dg DiscordSession, state *State, client *openai.Client, i *discordgo.InteractionCreate, activityName string, availableUserIDs []string, t time.Time) {
	if t.Before(time.Now()) {
		t = time.Now().Add(5 * time.Minute)
	}

	content := fmt.Sprintf("activity: %s\n time: %s\n users: ", activityName, t.Format("3:04 PM"))
	for _, userID := range availableUserIDs {
		content += UserMention(userID)
	}
	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: content,
	}

	funcArgs, err := GetToolResponse(context.Background(), dg, i.ChannelID, messageReq, "", GenerateEventTool, client, state)
	if err != nil {
		log.Println("could not generate event data", err)
		dg.ChannelMessageSend(i.ChannelID, ERROR_RESPONSE)
		return
	}

	var eventFuncArgs EventFuncArgs
	if err := json.Unmarshal([]byte(funcArgs[0].JsonValue), &eventFuncArgs); err != nil {
		log.Println("could not read eventFuncArgs", err)
		dg.ChannelMessageSend(i.ChannelID, ERROR_RESPONSE)
		return
	}

	if event, err := scheduleEvent(dg, i.GuildID, eventFuncArgs.Name, eventFuncArgs.Description, t); err != nil {
		log.Println("unabled to schedule event", err)
		dg.ChannelMessageSend(i.ChannelID, ERROR_RESPONSE)
	} else {
		message := eventFuncArgs.NotificationMessage + fmt.Sprintf("\nhttps://discord.com/events/%s/%s", event.GuildID, event.ID)
		dg.ChannelMessageSend(i.ChannelID, message)
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
				if timeSlot == t {
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
	return fmt.Sprintf(`you are generating a the message content in an interactive discord message. 
		%s is asking %s when is a good time for %s on %s. Generate just the content in your response, but be descriptive about the activity and snarky to the users. Do not include the current time.`,
		i.Member.User.Mention(),
		users,
		formData.Game,
		day,
	)
}

func combineDateTime(date, timeVal time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), timeVal.Hour(), timeVal.Minute(), 0, 0, timeVal.Location())
}
