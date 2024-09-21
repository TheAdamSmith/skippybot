package skippy

import (
	"fmt"
	"log"
	"skippybot/components"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
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
	}

	var removeHandlerFuncs []func()
	userSelect, removeHandlerFunc := components.SelectMenu(dg,
		discordgo.SelectMenu{
			MenuType:    discordgo.SelectMenuType(discordgo.UserSelectMenuComponent),
			Placeholder: "Select users",
			MaxValues:   25,
		},
		func(i *discordgo.InteractionCreate) {
			formData.UserIDS = i.MessageComponentData().Values
		})
	removeHandlerFuncs = append(removeHandlerFuncs, removeHandlerFunc)

	startSelect, removeHandlerFunc := components.SelectMenu(dg,
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
	removeHandlerFuncs = append(removeHandlerFuncs, removeHandlerFunc)

	endSelect, removeHandlerFunc := components.SelectMenu(dg,
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
	removeHandlerFuncs = append(removeHandlerFuncs, removeHandlerFunc)

	var intOptions []discordgo.SelectMenuOption
	for i := 1; i <= 6; i++ {
		intOptions = append(intOptions,
			discordgo.SelectMenuOption{
				Label: strconv.Itoa(i),
				Value: strconv.Itoa(i),
			},
		)
	}

	buttFunc := components.WithSubmitButton(dg,
		discordgo.Button{
			Label: "Send",
			Style: discordgo.PrimaryButton,
		},
		func(i *discordgo.InteractionCreate) {
			log.Printf("Form was submitted with: %#v\n", *formData)
			for _, removeHandlerFunc := range removeHandlerFuncs {
				removeHandlerFunc()
			}

			newContent := "Sending message..."
			if _, err := dg.InteractionResponseEdit(initialInteraction.Interaction, &discordgo.WebhookEdit{
				Content:    &newContent,
				Components: &[]discordgo.MessageComponent{},
			}); err != nil {
				log.Println("error updating message: ", err)
			}
			getUserAvailability(dg, formData)
		},
	)

	// ignore removeHandler because we used a submit button
	buttonRow, _ := components.ButtonRow(dg, buttFunc)

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
	var removeHandlerFuncs []func()
	var err error
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

		message, err = sendUserAvailabilityResponse(dg, i, message, userAvailability)
		if err != nil {
			log.Println("error sending user availability response", err)
		}
	}

	zero := 0
	timeSelect, removeFunc := components.SelectMenu(dg,
		discordgo.SelectMenu{
			Placeholder: "When you on?",
			MinValues:   &zero,
			MaxValues:   len(timeOptions),
			Options:     timeOptions,
		}, onSelect)
	removeHandlerFuncs = append(removeHandlerFuncs, removeFunc)

	buttFunc := components.WithButton(dg, discordgo.Button{
		Style: discordgo.DangerButton,
		Label: "Can't",
	}, func(i *discordgo.InteractionCreate) {
		userAvailability[i.Member.User.ID] = []time.Time{}
		message, err = sendUserAvailabilityResponse(dg, i, message, userAvailability)
		if err != nil {
			log.Println("error sending user availability response", err)
		}
	})
	buttonRow, removeFuncs := components.ButtonRow(dg, buttFunc)
	removeHandlerFuncs = append(removeHandlerFuncs, removeFuncs...)

	time.AfterFunc(6*time.Hour, func() {
		for _, removeFunc := range removeHandlerFuncs {
			removeFunc()
		}
	})

	dg.ChannelMessageSendComplex(formData.ChannelID, &discordgo.MessageSend{
		Components: []discordgo.MessageComponent{
			timeSelect,
			buttonRow,
		},
	})
}

func sendUserAvailabilityResponse(dg DiscordSession, i *discordgo.InteractionCreate, message *discordgo.Message, userAvailability map[string][]time.Time) (*discordgo.Message, error) {
	commonTimes := findCommonTimes(userAvailability, 3)

	messageEmbed := &discordgo.MessageEmbed{
		Fields: getUserAvailabilityFields(userAvailability, commonTimes),
	}

	// a bit confusing because MassegeEdit returns nil
	// we only need to get the message once to get the id
	if message == nil {
		message, err := dg.ChannelMessageSendEmbed(i.ChannelID, messageEmbed)

		return message, err
	}

	var buttFuncs []components.ButtonFunc
	for _, t := range commonTimes {
		if t.IsZero() {
			continue
		}

		buttFuncs = append(buttFuncs, components.WithSubmitButton(dg, discordgo.Button{
			Label: fmt.Sprintf("Create event for %s🚀", t.Format("3:04 PM MST")),
		}, func(i *discordgo.InteractionCreate) {
		}))
	}

	var messageComponents []discordgo.MessageComponent
	if len(buttFuncs) > 0 {
		// ignore removeFuncs because we use submitButtons
		eventButtonRow, _ := components.ButtonRow(dg, buttFuncs...)
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

func findCommonTimes(userAvailability map[string][]time.Time, topN int) []time.Time {
	timeCount := make(map[time.Time]int)
	for _, timeSlots := range userAvailability {
		for _, timeSlot := range timeSlots {
			timeCount[timeSlot]++
		}
	}

	times := make([]time.Time, topN)
	for timeSlot, count := range timeCount {
		for i := 0; i < topN; i++ {
			// want at least 2 responses
			if timeCount[times[i]] < count && count > 1 {
				times[i] = timeSlot
				break
			}
		}
	}

	return times
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
