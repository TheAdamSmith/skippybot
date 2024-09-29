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

type ComponentHandler struct {
	client            ComponentClient // discord client
	callbackFuncs     map[string]func(*discordgo.InteractionCreate)
	removeHandlerFunc func()
}

func NewComponentHandler(client ComponentClient) *ComponentHandler {
	componentHandler := &ComponentHandler{
		client:        client,
		callbackFuncs: map[string]func(*discordgo.InteractionCreate){},
	}

	removeHandlerFunc := client.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionMessageComponent {
			return
		}
		if callbackFunc, ok := componentHandler.callbackFuncs[i.MessageComponentData().CustomID]; ok {
			noOpResponse(componentHandler.client, i)
			callbackFunc(i)
		} else {
			if err := client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "This component has expired",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			},
			); err != nil {
				log.Println("Error sending response", err)
			}
		}
	})

	componentHandler.removeHandlerFunc = removeHandlerFunc

	return componentHandler
}

func (c *ComponentHandler) Close() {
	c.removeHandlerFunc()
}

// SelectMenu creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
// returns an ActionRow component
func (c *ComponentHandler) SelectMenu(selectMenu discordgo.SelectMenu, onSelect func(i *discordgo.InteractionCreate)) discordgo.ActionsRow {
	componentID := uuid.New().String()
	selectMenu.CustomID = componentID
	c.callbackFuncs[componentID] = onSelect

	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			selectMenu,
		},
	}
}

// WithButton returns a ButtonFunc to be used with ButtonRow
// creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
func (c *ComponentHandler) WithButton(button discordgo.Button, onClick func(i *discordgo.InteractionCreate)) discordgo.Button {
	componentID := uuid.New().String()
	button.CustomID = componentID
	c.callbackFuncs[componentID] = onClick

	return button
}

// WithSubmitButton returns a ButtonFunc with a submit button that dergisters its handler after click
// creates and overrides a new CustomID creates a handler that will execute onClick and send a default response
func (c *ComponentHandler) WithSubmitButton(button discordgo.Button, onClick func(i *discordgo.InteractionCreate)) discordgo.Button {
	componentID := uuid.New().String()
	button.CustomID = componentID
	c.callbackFuncs[componentID] = func(i *discordgo.InteractionCreate) {
		onClick(i)
		delete(c.callbackFuncs, componentID)
	}

	return button
}

// ButtonRow executes the ButtonFuncs to that create the buttons and register each handler. See WithButton and WithSubmitButton.
// returns an ActionRow component
func ButtonRow(client ComponentClient, buttons ...discordgo.Button) discordgo.ActionsRow {
	components := make([]discordgo.MessageComponent, len(buttons))
	for i, button := range buttons {
		components[i] = button
	}
	return discordgo.ActionsRow{
		Components: components,
	}
}

func noOpResponse(client ComponentClient, i *discordgo.InteractionCreate) {
	if err := client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	},
	); err != nil {
		log.Println("Error sending response", err)
	}
}
