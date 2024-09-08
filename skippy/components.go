package skippy

import (
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

type EventScheduler struct {
	NumResponses     int
	EventDescription string
	ResponseMap      map[string][]time.Time
}

type WhensGoodForm struct {
	Game      string
	Users     []string
	StartTime time.Time
	EndTime   time.Time
}

type ButtonFunc func() (discordgo.Button, func())

// TODO: doc
func generateWhensGoodResponse(dg DiscordSession, initialInteraction *discordgo.InteractionCreate) *discordgo.InteractionResponseData {
	formData := &WhensGoodForm{}

	optionValue, ok := findCommandOption(
		initialInteraction.ApplicationCommandData().Options,
		GAME,
	)
	if ok {
		formData.Game = optionValue.StringValue()
	}

	var removeHandlerFuncs []func()
	userSelect, removeHandlerFunc := SelectMenu(dg,
		discordgo.SelectMenu{
			MenuType:    discordgo.SelectMenuType(discordgo.UserSelectMenuComponent),
			Placeholder: "Select users",
			MaxValues:   25,
		},
		func(i *discordgo.InteractionCreate) {
			formData.Users = i.MessageComponentData().Values
		})
	removeHandlerFuncs = append(removeHandlerFuncs, removeHandlerFunc)

	startSelect, removeHandlerFunc := SelectMenu(dg,
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

	endSelect, removeHandlerFunc := SelectMenu(dg,
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

	buttFunc := WithSubmitButton(dg,
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
		},
	)
	buttonRow, _ := ButtonRow(dg, buttFunc)

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

// SelectMenu creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
// returns an ActionRow component and a func that will remove the created handler
func SelectMenu(dg DiscordSession, selectMenu discordgo.SelectMenu, onSelect func(i *discordgo.InteractionCreate)) (discordgo.ActionsRow, func()) {
	componentID := uuid.New().String()
	selectMenu.CustomID = componentID

	return discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				selectMenu,
			},
		}, dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.Type != discordgo.InteractionMessageComponent || i.MessageComponentData().CustomID != componentID {
				return
			}
			onSelect(i)
			noOpResponse(dg, i)
		})
}

// WithButton returns a ButtonFunc to be used with ButtonRow
// creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
func WithButton(dg DiscordSession, button discordgo.Button, onClick func(i *discordgo.InteractionCreate)) ButtonFunc {
	return func() (discordgo.Button, func()) {
		componentID := uuid.New().String()
		button.CustomID = componentID

		return button,
			dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
				if i.Type != discordgo.InteractionMessageComponent || i.MessageComponentData().CustomID != componentID {
					return
				}
				onClick(i)
				noOpResponse(dg, i)
			})
	}
}

// WithSubmitButton returns a ButtonFunc with a submit button that dergisters its handler after click
// creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
// onClick should handle deregistering other message component handlers
func WithSubmitButton(dg DiscordSession, button discordgo.Button, onClick func(i *discordgo.InteractionCreate)) ButtonFunc {
	return func() (discordgo.Button, func()) {
		componentID := uuid.New().String()
		button.CustomID = componentID
		var removeHandlerFunc func()

		removeHandlerFunc = dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.Type != discordgo.InteractionMessageComponent || i.MessageComponentData().CustomID != componentID {
				return
			}
			onClick(i)
			noOpResponse(dg, i)

			removeHandlerFunc()
		})

		return button, removeHandlerFunc
	}
}

// ButtonRow executes the ButtonFuncs to that create the buttons and register each handler. See WithButton and WithSubmitButton.
// returns an ActionRow component and a list of funcs that will remove each created handler
func ButtonRow(dg DiscordSession, buttFuncs ...ButtonFunc) (discordgo.ActionsRow, []func()) {
	var components []discordgo.MessageComponent
	var handlerFuncs []func()

	for _, buttFunc := range buttFuncs {
		button, handlerFunc := buttFunc()
		components = append(components, button)
		handlerFuncs = append(handlerFuncs, handlerFunc)
	}

	return discordgo.ActionsRow{
		Components: components,
	}, handlerFuncs
}

func getTimeOptions(d time.Duration) []discordgo.SelectMenuOption {
	startTime := time.Date(0, 1, 1, time.Now().Hour(), 0, 0, 0, time.Local)
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

func noOpResponse(dg DiscordSession, i *discordgo.InteractionCreate) {
	if err := dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	},
	); err != nil {
		log.Println("Error sending response", err)
	}
}
