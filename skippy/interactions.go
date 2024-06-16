package skippy

import (
	"context"
	"fmt"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

const (
	RL_SESH        = "rl_sesh"
	ALWAYS_RESPOND = "always_respond"
	SEND_MESSAGE   = "send_message"
)

func initSlashCommands(dg *discordgo.Session) ([]*discordgo.ApplicationCommand, error) {
	var commands []*discordgo.ApplicationCommand
	command := discordgo.ApplicationCommand{
		Name:        "send_message",
		Description: "Have the bot send a message",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "channel to send message on",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "message to send",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionMentionable,
				Name:        "mention",
				Description: "anyone to mention",
				Required:    false,
			},
		},
	}

	applicationCommand, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)

	if err != nil {
		log.Println("error creating application command: ", err)
	}

	command = discordgo.ApplicationCommand{
		Name:        "always_respond",
		Description: "Toggle auto respond when on Skippy will always respond to messages in this channel",
	}

	applicationCommand, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)

	if err != nil {
		log.Println("error creating application command: ", err)
	}

	command = discordgo.ApplicationCommand{
		Name:        "rl_sesh",
		Description: "start/stop rl sesh",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "startorstop",
				Description: "should be either start or stop",
				Required:    false,
			},
		},
	}

	applicationCommand, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)
	if err != nil {
		log.Println("error creating application command: ", err)
	}

	return commands, err
}

func onCommand(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {
	log.Println(i.ApplicationCommandData().Name)
	switch i.ApplicationCommandData().Name {
	case RL_SESH:
		handleRLSesh(dg, i, client, state)
	case ALWAYS_RESPOND:
		handleAlwaysRespond(dg, i, client, state)
	case SEND_MESSAGE:
		sendChannelMessage(dg, i, client, state)
	default:
		log.Println("recieved unrecognized command")
		err := dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Recieved unrecognized command",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}

}

func sendChannelMessage(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 2 {
		return fmt.Errorf("recieved an incorrect amount of options")
	}

	channel := options[0].ChannelValue(nil)
	channelID := channel.ID

	prompt := options[1].StringValue()
	var mentionString string

	if len(options) > 2 {
		mention := options[2].UserValue(nil)
		mentionString = mention.Mention()
		log.Println(mentionString)
	}

	message := "prompt: " + prompt + "\n"
	if mentionString != "" {
		message += "User ID :" + mentionString
	}

	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	}

	ctx := context.WithValue(context.Background(), DisableFunctions, true)

	go getAndSendResponse(
		ctx,
		dg,
		channelID,
		messageReq,
		SEND_CHANNEL_MSG_INSTRUCTIONS,
		client,
		state,
	)

	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sent the message",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
		return err
	}

	return nil
}

func handleAlwaysRespond(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {
	enabled := state.toggleAlwaysRespond(i.ChannelID, client)
	var message string
	if enabled {
		message = "Turned on always respond"
	} else {
		message = "Turned off always respond"
	}
	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}
}

func handleRLSesh(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) {

	if i.ApplicationCommandData().Name != "rl_sesh" {
		return
	}

	// TODO: find specific command
	// maybe add option to discord package?
	textOption := i.ApplicationCommandData().Options[0].Value // rl_sesh
	if textOption == "start" {
		log.Println("Handling rl_sesh start command. creating new thread")

		err := state.resetOpenAIThread(i.ChannelID, client)
		if err != nil {
			log.Println("Unable to create thread: ", err)
			err = dg.InteractionRespond(i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Unable to start chat thread",
					},
				})
			if err != nil {
				log.Printf("Error responding to slash command: %s\n", err)
			}
			return
		}

		ctx, cancelFunc := context.WithCancel(context.Background())
		state.threadMap[i.ChannelID].cancelFunc = cancelFunc

		message := "Started rocket league session"
		filePath := os.Getenv("RL_DIR")
		err = StartRocketLeagueSession(ctx, filePath, i.ChannelID, dg, state, client)
		if err != nil {
			message = "unable to start rocket leage session"
		}
		err = dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}

	if textOption == "stop" {

		var message string
		cancelFunc := state.threadMap[i.ChannelID].cancelFunc
		if cancelFunc == nil {
			message = "Unable to stop session no cancel function"
		} else {
			cancelFunc()
			message = "Stopped rocket league session"
		}

		err := dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			})
		if err != nil {
			log.Printf("Error responding to slash command: %s\n", err)
		}
	}

}

//lint:ignore U1000 saving for later
func deleteSlashCommands(dg *discordgo.Session) error {
	appCommands, err := dg.ApplicationCommands(dg.State.Application.ID, "")
	if err != nil {
		log.Println("Could not get application commands: ", err)
		return fmt.Errorf(
			"could not get application commands:%s",
			err,
		)
	}

	for _, appCommand := range appCommands {
		err = dg.ApplicationCommandDelete(dg.State.Application.ID, "", appCommand.ID)
		if err != nil {
			log.Println("Could not delete command", err)
			return err
		}
	}
	return nil
}
