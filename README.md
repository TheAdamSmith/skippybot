# Skippybot

An AI Discord bot built using ChatGPT with a personality modeled after Skippy the Magnficent from Craig Alanson's [Expeditionary Force](https://www.amazon.com/Expeditionary-Force-17-book-series/dp/B07F7T8NPK)

## Functionality

Skippy has a lot of different functionality with an emphasis on gaming related tasks. 


You can ask Skippy questions just like you would any chat bot. 
By default Skippy only responds to @ messages: `@Skippy what is the meaning of the universe?`
Skippy can answer any questions that you would be able to ask ChatGPT

### AI Functions

Skippy has several different functions that can be invoked by asking him.

- Get the price of a stock the AI needs to be able to resolve the ticker. ex: `@Skippy what is the price of gamestop?`
- Get the weather. ex: `@Skippy what is the weather in Thompson Corners, Maine?`
- Set a reminder. ex: `@Skippy can you remind me in 30 minutes to take out the trash?`
    - When you set a reminder skippy will remind you in the channel that you asked for the reminder. You must acknowlege that you have received the reminder or else Skippy will continue to remind you about it 
- Set a morning message. Skippy will send a morning message in channel you sent the message in. You can optionally specify locations to fetch the weather from and stocks to get the price for. ex: `@Skippy can you set the morning message for 9:00 am? Get the weather for Thompson Corner, Maine and the stock price for gamestop`
- Generate an image. Skippy can generate an image for you. This feature is a work in progress. ex: `@Skippy can you generate an image of a fun party banana?`

### Discord Commands

Skippy has several built in discord commands with various functions. You can see these commands by starting a message with `/` 

- `/always_respond` this toggle if Skippy responds to all messages or just messages with an `@Skippy`
- `/send_message` use this to send an unprompted message to a channel of your choice. Can optionally have Skippy @ certain users
- `/track_game_usage` this enables Skippy's game tracking feature. Skippy will track your video game playing through Discord presence updates
    (you must be sharing your status with Discord for this feature to work). Optionally can set a daily limit that when reached Skippy will send you a message notifying you.
    This will default to a DM, but you can specify a channel you would like to get this reminder in.
- `/game_stats` This command will fetch your game stats. Defaults to todays stats, but can optionally specify the number of days to fetch your stats.
- `/whens_good` This command starts an interaction flow that can be used for scheduling a time for people to play games (or any other activity together).
    This will generate multiple messages for users to provide their availible times and provide buttons to have Skippy automatically create a Discord event. Try it out!

## Running the bot

Make sure you have Go 1.22 installed. 


Skippy requires multiple enviornment variables to run. 

```
# Required
OPEN_AI_KEY=<your-open-ai-key>
DISCORD_TOKEN=<your-discord-token>
ASSISTANT_ID=<your-openai-assistant-id>
# Optional, but neeeded for the weather and stock price functionality
ALPHA_VANTAGE_API_KEY=<your-alpha-vantage-key>
WEATHER_API_KEY=<key-for-weatherapi.com>
```
These can also be set with a .env

With the enviornment set you can
```
CGO_ENABLED=1 go build && ./skippybot
```

## Acknowlegements

Big thanks to the developers of [discordgo](https://github.com/bwmarrin/discordgo) and [go-openai](https://github.com/sashabaranov/go-openai) whose projects make this one possible!
