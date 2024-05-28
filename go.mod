module skippybot

go 1.18

require (
	github.com/bwmarrin/discordgo v0.27.1
	github.com/joho/godotenv v1.5.1
	github.com/sashabaranov/go-openai v1.24.0
)

require (
	github.com/gorilla/websocket v1.4.2 // indirect
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/sys v0.4.0 // indirect
)

// temporary fix for Assistants v2 incompatability see: https://github.com/sashabaranov/go-openai/pull/754
replace github.com/sashabaranov/go-openai => github.com/TheAdamSmith/go-openai v0.0.0-20240527214403-9e9dbcb8a10c
