package components

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

type ButtonFunc func() (discordgo.Button, func())

type ComponentClient interface {
	AddHandler(handler interface{}) func()
	// see discordgo.Session.InteractionRespond()
	InteractionRespond(
		interaction *discordgo.Interaction,
		resp *discordgo.InteractionResponse,
		options ...discordgo.RequestOption,
	) error
}

// SelectMenu creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
// returns an ActionRow component and a func that will remove the created handler
func SelectMenu(client ComponentClient, selectMenu discordgo.SelectMenu, onSelect func(i *discordgo.InteractionCreate)) (discordgo.ActionsRow, func()) {
	componentID := uuid.New().String()
	selectMenu.CustomID = componentID

	return discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				selectMenu,
			},
		}, client.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.Type != discordgo.InteractionMessageComponent || i.MessageComponentData().CustomID != componentID {
				return
			}
			onSelect(i)
			noOpResponse(client, i)
		})
}

// WithButton returns a ButtonFunc to be used with ButtonRow
// creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
func WithButton(client ComponentClient, button discordgo.Button, onClick func(i *discordgo.InteractionCreate)) ButtonFunc {
	return func() (discordgo.Button, func()) {
		componentID := uuid.New().String()
		button.CustomID = componentID

		return button,
			client.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
				if i.Type != discordgo.InteractionMessageComponent || i.MessageComponentData().CustomID != componentID {
					return
				}
				onClick(i)
				noOpResponse(client, i)
			})
	}
}

// WithSubmitButton returns a ButtonFunc with a submit button that dergisters its handler after click
// creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
// onClick should handle deregistering other message component handlers
func WithSubmitButton(client ComponentClient, button discordgo.Button, onClick func(i *discordgo.InteractionCreate)) ButtonFunc {
	return func() (discordgo.Button, func()) {
		componentID := uuid.New().String()
		button.CustomID = componentID
		var removeHandlerFunc func()

		removeHandlerFunc = client.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.Type != discordgo.InteractionMessageComponent || i.MessageComponentData().CustomID != componentID {
				return
			}
			onClick(i)
			noOpResponse(client, i)

			removeHandlerFunc()
		})

		return button, removeHandlerFunc
	}
}

// ButtonRow executes the ButtonFuncs to that create the buttons and register each handler. See WithButton and WithSubmitButton.
// returns an ActionRow component and a list of funcs that will remove each created handler
func ButtonRow(client ComponentClient, buttFuncs ...ButtonFunc) (discordgo.ActionsRow, []func()) {
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

func noOpResponse(client ComponentClient, i *discordgo.InteractionCreate) {
	if err := client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	},
	); err != nil {
		log.Println("Error sending response", err)
	}
}
