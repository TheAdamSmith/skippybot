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
	formData := &WhensGoodForm{
		ChannelID: initialInteraction.ChannelID,
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
			log.Printf("Form was submitted with: %#v\n", *formData)

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

		message, respErr = sendUserAvailabilityResponse(dg, state, client, i, message, userAvailability, formData.Game)
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
			message, respErr = sendUserAvailabilityResponse(dg, state, client, i, message, userAvailability, formData.Game)
			if respErr != nil {
				log.Println("error sending user availability response", respErr)
			}
		})
	buttonRow := components.ButtonRow(dg, cantButton)

	content, err := getUserAvailabilityContent(dg, state, client, initialInteraction, formData)
	if err != nil {
		log.Println("error generating content for user availability message", err)
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

func sendUserAvailabilityResponse(dg DiscordSession, state *State, client *openai.Client, i *discordgo.InteractionCreate, message *discordgo.Message, userAvailability map[string][]time.Time, activityName string) (*discordgo.Message, error) {
	commonTimes, userTimeMap := findCommonTimes(userAvailability, 3)

	messageEmbed := &discordgo.MessageEmbed{
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
			generateAndScheduleEvent(dg, state, client, i, activityName, userTimeMap[t], t)
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
			Name:   " ",
			Value:  content,
			Inline: true,
		})
	}

	return fields
}

func findCommonTimes(userAvailability map[string][]time.Time, topN int) ([]time.Time, map[time.Time][]string) {
	timeMap := make(map[time.Time][]string)
	var timeList []time.Time
	for userID, timeSlots := range userAvailability {
		for _, timeSlot := range timeSlots {
			timeMap[timeSlot] = append(timeMap[timeSlot], userID)
			timeList = append(timeList, timeSlot)
		}
	}

	sort.Slice(timeList, func(i, j int) bool {
		return len(timeMap[timeList[i]]) > len(timeMap[timeList[j]])
	})
	topTimes := timeList
	if len(topTimes) > topN {
		topTimes = topTimes[:topN]
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

func makeTimeSelectInstructions(formData *WhensGoodForm, i *discordgo.InteractionCreate) string {
	users := i.Member.User.Mention()
	for _, userID := range formData.UserIDS {
		users += fmt.Sprintf(",%s", UserMention(userID))
	}

	return fmt.Sprintf(`you are generating a the message content in an interactive discord message for. 
		Asking %s when is a good time for %s. Generate just the content in your response, but be descriptive about the activity and snarky to the users. Do not include the current time.`,
		users,
		formData.Game,
	)
}
