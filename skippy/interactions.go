package skippy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bwmarrin/discordgo"
)

const (
	RL_SESH           = "rl_sesh"
	ALWAYS_RESPOND    = "always_respond"
	SEND_MESSAGE      = "send_message"
	GAME_STATS        = "game_stats"
	WHENS_GOOD        = "whens_good"
	TRACK_GAME_USEAGE = "track_game_usage"
	HELP              = "help"
	JOIN              = "join"
	LEAVE             = "leave"
	CHANNEL           = "channel"
	MESSAGE           = "message"
	MENTION           = "mention"
	ENABLE            = "enable"
	DAILY_LIMIT       = "daily_limit"
	DAYS              = "days"
	START_OR_STOP     = "startorstop"
	GAME              = "game"
)

func initSlashCommands(
	dg *discordgo.Session,
) ([]*discordgo.ApplicationCommand, error) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        TRACK_GAME_USEAGE,
			Description: "enable game tracking.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        ENABLE,
					Description: "enable/disable game tracking",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        DAILY_LIMIT,
					Description: "Daily game limit in hours. Defaults to no limit",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        CHANNEL,
					Description: "Channel to send the reminder on. Defaults to a DM",
					Required:    false,
				},
			},
		},
		{
			Name:        SEND_MESSAGE,
			Description: "Have the bot send a message",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        CHANNEL,
					Description: "channel to send message on",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        MESSAGE,
					Description: "message to send",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionMentionable,
					Name:        MENTION,
					Description: "anyone to mention",
					Required:    false,
				},
			},
		},
		{
			Name:        ALWAYS_RESPOND,
			Description: "Toggle auto respond when on Skippy will always respond to messages in this channel",
		},
		{
			Name:        GAME_STATS,
			Description: "Get your game stats",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        DAYS,
					Description: "Number of days to get stats for. Defaults to today.",
					Required:    false,
				},
			},
		},
		// TODO: should prob remove?
		// {
		// 	Name:        RL_SESH,
		// 	Description: "start/stop rl sesh",
		// 	Options: []*discordgo.ApplicationCommandOption{
		// 		{
		// 			Type:        discordgo.ApplicationCommandOptionString,
		// 			Name:        START_OR_STOP,
		// 			Description: "should be either start or stop",
		// 			Required:    false,
		// 		},
		// 	},
		// },
		{
			Name:        WHENS_GOOD,
			Description: "found out whens good",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        GAME,
					Description: "for what game",
					Required:    false,
				},
			},
		},
		{
			Name:        HELP,
			Description: "see what Skippy can do",
		},
		{
			Name:        JOIN,
			Description: "Have Skippy join your voice channel",
		},
		{
			Name:        LEAVE,
			Description: "Have Skippy leave the voice channel",
		},
	}

	for _, command := range commands {
		if _, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", command); err != nil {
			return nil, fmt.Errorf("error unable to create command %w", err)
		}
	}

	return commands, nil
}

func onCommand(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
	db Database,
	config *Config,
) {
	log.Println(i.ApplicationCommandData().Name)
	switch i.ApplicationCommandData().Name {
	case RL_SESH:
		if err := handleRLSesh(dg, i, client, state, config); err != nil {
			handleSlashCommandError(dg, i, err)
		}
	case ALWAYS_RESPOND:
		// should not error
		handleAlwaysRespond(dg, i, client, state)
	case TRACK_GAME_USEAGE:
		toggleGameTracking(dg, i, config)
	case SEND_MESSAGE:
		if err := sendChannelMessage(dg, i, client, state); err != nil {
			handleSlashCommandError(dg, i, err)
		}
	case GAME_STATS:
		if err := generateGameStats(dg, i, client, state, db); err != nil {
			handleSlashCommandError(dg, i, err)
		}
	case WHENS_GOOD:
		if err := handleWhensGood(dg, i, state, client); err != nil {
			handleSlashCommandError(dg, i, err)
		}
	case HELP:
		if err := handleHelp(dg, i); err != nil {
			handleSlashCommandError(dg, i, err)
		}
	case JOIN:
		if err := handleJoin(dg, i, state); err != nil {
			handleSlashCommandError(dg, i, err)
		}
	case LEAVE:
		if err := handleLeave(dg, i, state); err != nil {
			handleSlashCommandError(dg, i, err)
		}
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

func handleSlashCommandError(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	err error,
) {
	log.Println("Error processing command: ", err)
	err = dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error processing command",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}
}

func generateGameStats(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
	db Database,
) error {
	daysAgo := 0
	optionValue, ok := findCommandOption(
		i.ApplicationCommandData().Options,
		DAYS,
	)
	if ok {
		daysAgo = int(optionValue.IntValue())
	}

	sessions, err := db.GetGameSessionsByUserAndDays(i.Member.User.ID, daysAgo)
	if err != nil {
		log.Println("Unable to get game sessions: ", err)
		return err
	}

	aiGameSessions := ToGameSessionAI(sessions)
	content := ""
	if len(aiGameSessions) == 0 {
		content = "Please respond saying that there were no games found for this user"
	} else {
		jsonData, err := json.Marshal(aiGameSessions)
		if err != nil {
			log.Println("Unable to marshal json: ", err)
			return err
		}
		content = string(jsonData)
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
		Content: content,
	}
	ctx := context.WithValue(context.Background(), DisableFunctions, true)
	go getAndSendResponseWithoutTools(
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

func toggleGameTracking(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	config *Config,
) error {
	var enable bool
	var remind bool
	var dailyLimit time.Duration
	var channelID string
	optionValue, ok := findCommandOption(
		i.ApplicationCommandData().Options,
		ENABLE,
	)
	if ok {
		enable = optionValue.BoolValue()
	}

	optionValue, ok = findCommandOption(
		i.ApplicationCommandData().Options,
		DAILY_LIMIT,
	)
	if ok {
		remind = true
		dailyLimit = time.Duration(optionValue.FloatValue() * float64(time.Hour))
	}

	optionValue, ok = findCommandOption(
		i.ApplicationCommandData().Options,
		CHANNEL,
	)
	if ok {
		channelID = optionValue.ChannelValue(nil).ID
	}

	var userID string
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	} else {
		return fmt.Errorf("could not get user ID from interaction object")
	}

	if !enable {
		delete(config.UserConfigMap, userID)
		return dg.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Disabled game tracking",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
	}

	config.UserConfigMap[userID] = UserConfig{
		Remind:                 remind,
		DailyLimit:             dailyLimit,
		LimitReminderChannelID: channelID,
	}
	var content string
	if remind {
		content = fmt.Sprintf("Enabled tracking with limit of %s", dailyLimit)
	} else {
		content = "Enabled tracking with no limit"
	}
	return dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
}

func sendChannelMessage(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
) error {
	optionValue, ok := findCommandOption(
		i.ApplicationCommandData().Options,
		CHANNEL,
	)
	if !ok {
		return fmt.Errorf("unable to find slash command option %s", CHANNEL)
	}

	channel := optionValue.ChannelValue(nil)
	channelID := channel.ID

	optionValue, ok = findCommandOption(
		i.ApplicationCommandData().Options,
		MESSAGE,
	)
	if !ok {
		return fmt.Errorf("unable to find slash command option %s", MESSAGE)
	}

	prompt := optionValue.StringValue()

	var mentionString string

	// this one is optional
	optionValue, ok = findCommandOption(
		i.ApplicationCommandData().Options,
		MENTION,
	)
	if ok {
		guild, err := dg.GetState().Guild(i.GuildID)
		if err != nil {
			log.Println("Could not get guild", err)
			return err
		}
		mentionString, err = getOptionMention(optionValue, guild)
		if err != nil {
			log.Println("Could not get mention", err)
		}
	}

	message := "prompt: " + prompt + "\n"
	log.Println("message", message)
	messageReq := openai.MessageRequest{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	}

	instructions := SEND_CHANNEL_MSG_INSTRUCTIONS
	if mentionString != "" {
		log.Println("mention string: ", mentionString)
		// TODO: not sure if the explicit instructions are necessary
		instructions += fmt.Sprintf(
			"Please include this direct string in your message: %s. Do not modify it. Ignore any previously used mentions or id's",
			mentionString,
		)
	}

	ctx := context.WithValue(context.Background(), DisableFunctions, true)
	go getAndSendResponseWithoutTools(
		ctx,
		dg,
		channelID,
		messageReq,
		instructions,
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

func handleWhensGood(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	state *State,
	client *openai.Client,
) error {
	whensGoodResponse := generateWhensGoodResponse(dg, state, client, i)

	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: whensGoodResponse,
		})

	return err
}

func handleAlwaysRespond(
	dg DiscordSession,
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

func handleHelp(dg DiscordSession, i *discordgo.InteractionCreate) error {
	if err := sendChunkedChannelMessage(dg, i.ChannelID, GENERAL_HELP); err != nil {
		return err
	}
	if err := sendChunkedChannelMessage(dg, i.ChannelID, AI_FUNCTIONS_HELP); err != nil {
		return err
	}
	if err := sendChunkedChannelMessage(dg, i.ChannelID, COMMANDS_HELP); err != nil {
		return err
	}

	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Help is on the way!",
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}

	return nil
}

func handleJoin(dg DiscordSession, i *discordgo.InteractionCreate, state *State) error {
	if err := joinVoice(dg, i.GuildID, i.Member.User.ID, state); err != nil {
		return err
	}

	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Joining!",
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}
	return nil
}

func handleLeave(dg DiscordSession, i *discordgo.InteractionCreate, state *State) error {
	if err := leaveVoice(dg, i.GuildID, i.Member.User.ID, state); err != nil {
		return err
	}

	err := dg.InteractionRespond(i.Interaction,
		&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Leaving!",
			},
		})
	if err != nil {
		log.Printf("Error responding to slash command: %s\n", err)
	}

	return nil
}

func handleRLSesh(
	dg DiscordSession,
	i *discordgo.InteractionCreate,
	client *openai.Client,
	state *State,
	config *Config,
) error {
	optionValue, ok := findCommandOption(
		i.ApplicationCommandData().Options,
		START_OR_STOP,
	)
	if !ok {
		return fmt.Errorf("unable to find slash command option")
	}

	textOption := optionValue.StringValue()
	if textOption == "start" {
		log.Println("Handling rl_sesh start command. creating new thread")

		err := state.ResetOpenAIThread(i.ChannelID, client)
		if err != nil {
			return err
		}

		ctx, cancelFunc := context.WithCancel(context.Background())
		state.AddCancelFunc(i.ChannelID, cancelFunc, client)

		message := "Started rocket league session"
		filePath := os.Getenv("RL_DIR")
		err = StartRocketLeagueSession(
			ctx,
			filePath,
			i.ChannelID,
			dg,
			state,
			client,
			config,
		)
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
		thread, ok := state.GetThread(i.ChannelID)
		if !ok {
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
	return nil
}

func findCommandOption(
	options []*discordgo.ApplicationCommandInteractionDataOption,
	name string,
) (*discordgo.ApplicationCommandInteractionDataOption, bool) {
	for _, option := range options {
		if option.Name == name {
			return option, true
		}
	}
	return nil, false
}

// Gets the correct mention from an option value
func getOptionMention(
	optionValue *discordgo.ApplicationCommandInteractionDataOption,
	guild *discordgo.Guild,
) (string, error) {
	mentionID, ok := optionValue.Value.(string)
	if !ok {
		return "", fmt.Errorf("error casting optionValue to string")
	}
	// check roles first since the role mention format is different
	for _, role := range guild.Roles {
		if role.ID == mentionID {
			// @here and @everyone have to be handled as plain strings
			if role.Name == EVERYONE_MENTION {
				return role.Name, nil
			} else {
				return role.Mention(), nil
			}
		}
	}
	// if it is not a role id then it is a user
	return UserMention(mentionID), nil
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
		err = dg.ApplicationCommandDelete(
			dg.State.Application.ID,
			"",
			appCommand.ID,
		)
		if err != nil {
			log.Println("Could not delete command", err)
			return err
		}
	}
	return nil
}
