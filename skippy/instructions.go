package skippy

// The additional instructsions provided for each message type
const (
	// TODO: determine if this is needed
	DEFAULT_INSTRUCTIONS = `Try to be as helpful as possible while keeping the iconic skippy saracasm in your response.
	Use responses of varying lengths.
	`
	MORNING_MESSAGE_INSTRUCTIONS = `
	You are making creating morning wake up message for the users of a discord server. Make sure to mention @here in your message. 
	Be creative in the message you create in wishing everyone good morning. If there is weather data included in the message please give a brief overview of the weather for each location.
	if there is stock price information included in the message include that information in the message.
	`
	SEND_CHANNEL_MSG_INSTRUCTIONS   = `You are generating a message to send in a discord channel. Generate a message based on the prompt.`
	GENERATE_GAME_STAT_INSTRUCTIONS = `You are summarizing a users game sessions. 
	The message will be a a json formatted list of game sessions. 
	Please summarise the results of the sessions including total hours played and the most played game. If multiple days are incluced give the breakdown by each day.
	This is the user mention (%s) of the user you are summarizing. Please include it in your message.
	`
	GAME_LIMIT_REMINDER_INSTRUCTIONS_FORMAT = `You are reminding a discord user that they have exceeded their configured daily video game limit.
	You will get a list in json format of the users game sessions from today. Do not give them a summary but reference SOME of the games and session lengths in your response.
	Please tell them that it is time to touch grass. Keep your response brief.
	Please include this discord mention in your message %s
	`
	COMMENTATE_INSTRUCTIONS = `
	Messages will be sent in this thread that will contain the json results of a rocket league game.
	Announce the overall score and commentate on the performance of the home team. Come up with creative insults on their performance, but praise high performers
	`
	GENERAL_HELP = "# Skippybot\n\n" +
		"An AI Discord bot built using ChatGPT with a personality modeled after Skippy the Magnficent from Craig Alanson's " +
		"[Expeditionary Force](https://www.amazon.com/Expeditionary-Force-17-book-series/dp/B07F7T8NPK)\n\n" +

		"## Functionality\n\n" +
		"Skippy has a lot of different functionality with an emphasis on gaming related tasks.\n\n" +

		"You can ask Skippy questions just like you would any chat bot.\n" +
		"By default Skippy only responds to @ messages: `@Skippy what is the meaning of the universe?`\n" +
		"Skippy can answer any questions that you would be able to ask ChatGPT\n\n"

	AI_FUNCTIONS_HELP = "### AI Functions\n\n" +
		"Skippy has several different functions that can be invoked by asking him.\n\n" +

		"- Get the price of a stock the AI needs to be able to resolve the ticker. ex: `@Skippy what is the price of gamestop?`\n" +
		"- Get the weather. ex: `@Skippy what is the weather in Thompson Corners, Maine?`\n" +
		"- Set a reminder. ex: `@Skippy can you remind me in 30 minutes to take out the trash?`\n" +
		"    - When you set a reminder Skippy will remind you in the channel that you asked for the reminder. " +
		"You must acknowledge that you have received the reminder or else Skippy will continue to remind you about it\n" +
		"- Set a morning message. Skippy will send a morning message in the channel you sent the message in. " +
		"You can optionally specify locations to fetch the weather from and stocks to get the price for. ex: `@Skippy can you set the morning message for 9:00 am? Get the weather for Thompson Corner, Maine and the stock price for gamestop`\n" +
		"- Generate an image. Skippy can generate an image for you. This feature is a work in progress. ex: `@Skippy can you generate an image of a fun party banana?`\n\n"

	COMMANDS_HELP = "### Discord Commands\n\n" +
		"Skippy has several built in discord commands with various functions. You can see these commands by starting a message with `/`\n\n" +

		"- `/always_respond` this toggles if Skippy responds to all messages or just messages with an `@Skippy`\n" +
		"- `/send_message` use this to send an unprompted message to a channel of your choice. Can optionally have Skippy @ certain users\n" +
		"- `/track_game_usage` this enables Skippy's game tracking feature. Skippy will track your video game playing through Discord presence updates " +
		"(you must be sharing your status with Discord for this feature to work). Optionally can set a daily limit that when reached Skippy will send " +
		"you a message notifying you. This will default to a DM, but you can specify a channel you would like to get this reminder in.\n" +
		"- `/game_stats` This command will fetch your game stats. Defaults to today's stats, but can optionally specify the number of days to fetch your stats.\n" +
		"- `/whens_good` This command starts an interaction flow that can be used for scheduling a time for people to play games (or any other activity together)."
)
