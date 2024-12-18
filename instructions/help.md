{BOT_NAME} has a lot of different functionality with an emphasis on gaming-related tasks.

You can ask {BOT_NAME} questions just like you would any chatbot.
By default, {BOT_NAME} only responds to @ messages: `{BOT_MENTION} what is the meaning of the universe?`
{BOT_NAME} can answer any questions that you would be able to ask ChatGPT.

## AI Functions

{BOT_NAME} has several different functions that can be invoked by asking.

- Get the price of a stock. The AI needs to be able to resolve the ticker. ex: `{BOT_MENTION} what is the price of gamestop?`
- Get the weather. ex: `{BOT_MENTION} what is the weather in Portland?`
- Set a reminder. ex: `{BOT_MENTION} can you remind me in 30 minutes to take out the trash?`
    - {required}When you set a reminder, I will remind you in the channel that you asked for the reminder. You must acknowledge that you have received the reminder or else I will continue to remind you about it.{required}
- Set a morning message. {BOT_NAME} will send a morning message in the channel you sent the message in. You can optionally specify locations to fetch the weather from and stocks to get the price for. ex: `{BOT_MENTION} can you set the morning message for 9:00 am? Get the weather for Portland, OR and the stock price for gamestop.`
- Generate an image. {BOT_NAME} can generate an image for you. This feature is a work in progress. ex: `{BOT_MENTION} can you generate an image of a fun party banana?`

## Discord Commands

You have several built-in Discord commands with various functions. Commands are used by starting a message with `/`.

- `/always_respond` toggles if you respond to all messages or just messages with an `{BOT_MENTION}`.
- `/send_message` use this to send an unprompted message to a channel of your choice. Can optionally @ certain users.
- `/track_game_usage` enables the game tracking feature. {BOT_NAME} will track your video game playing through Discord presence updates {required}(You must be sharing your game status with Discord for this feature to work). Optionally can set a daily limit that, when reached, {BOT_NAME} will send you a message notifying you. This will default to a DM, but you can specify a channel you would like to get this reminder in.{required}
- `/game_stats` fetches your game stats. {required}Defaults to today's stats{required}
- `/whens_good` starts an interaction flow that can be used for scheduling a time for people to play games (or any other activity together). {required}Try it out!{required}
