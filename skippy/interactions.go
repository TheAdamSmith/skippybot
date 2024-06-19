package skippy

import (
	"context"
	"encoding/json"
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
	GAME_STATS     = "game_stats"
)

func initSlashCommands(dg *discordgo.Session) ([]*discordgo.ApplicationCommand, error) {
	var commands []*discordgo.ApplicationCommand
	command := discordgo.ApplicationCommand{
		Name:        SEND_MESSAGE,
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
		Name:        ALWAYS_RESPOND,
		Description: "Toggle auto respond when on Skippy will always respond to messages in this channel",
	}

	applicationCommand, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)

	if err != nil {
		log.Println("error creating application command: ", err)
	}

	command = discordgo.ApplicationCommand{
		Name:        GAME_STATS,
		Description: "Get your game stats",
	}

	applicationCommand, err = dg.ApplicationCommandCreate(dg.State.User.ID, "", &command)
	commands = append(commands, applicationCommand)

	if err != nil {
		log.Println("error creating application command: ", err)
	}
	command = discordgo.ApplicationCommand{
		Name:        RL_SESH,
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
	db Database,
) {
	log.Println(i.ApplicationCommandData().Name)
	switch i.ApplicationCommandData().Name {
	case RL_SESH:
		handleRLSesh(dg, i, client, state)
	case ALWAYS_RESPOND:
		handleAlwaysRespond(dg, i, client, state)
	case SEND_MESSAGE:
		sendChannelMessage(dg, i, client, state)
	case GAME_STATS:
		generateGameStats(dg, i, client, state, db)
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

func generateGameStats(
	dg *discordgo.Session,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
	db Database,
) error {
	sessions, err := db.GetGameSessionsByUser(i.Member.User.ID)
	if err != nil {
		log.Println("Unable to get game sessions: ", err)
		return err
	}

	jsonData, err := json.Marshal(sessions.ToGameSessionAI())
	if err != nil {
		log.Println("Unable to marshal json: ", err)
		return err
	}

	// respond before sending response to maintain consistent
	// ChannelTyping behavior
	err = dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Fetching your stats!",
				Flags:   discordgo.MessageFlagsLoading,
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
		return err
	}

	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: string(jsonData),
	}
	ctx := context.WithValue(context.Background(), DisableFunctions, true)
	go getAndSendResponse(
		ctx,
		dg,
		i.ChannelID,
		messageReq,
		fmt.Sprintf(GENERATE_GAME_STAT_INSTRUCTIONS, i.Member.Mention()),
		client,
		state,
	)
	return nil
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
				Content: "On it!",
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
	enabled := state.ToggleAlwaysRespond(i.ChannelID, client)
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

		err := state.ResetOpenAIThread(i.ChannelID, client)
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
		state.AddCancelFunc(i.ChannelID, cancelFunc, client)

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
		thread, exists := state.GetThread(i.ChannelID)
		if !exists {
			message = "unable to stop session no thread found"
		}
		cancelFunc := thread.cancelFunc
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
