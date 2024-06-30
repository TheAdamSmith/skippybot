module skippybot

go 1.18

require (
	github.com/bwmarrin/discordgo v0.27.1
	github.com/joho/godotenv v1.5.1
	github.com/sashabaranov/go-openai v1.24.0
	gorm.io/driver/sqlite v1.5.6
	gorm.io/gorm v1.25.10
)

require (
	github.com/go-co-op/gocron/v2 v2.7.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/sys v0.4.0 // indirect
)

// temporary fix for Assistants v2 incompatability see: https://github.com/sashabaranov/go-openai/pull/754
replace github.com/sashabaranov/go-openai => github.com/TheAdamSmith/go-openai v0.0.0-20240527214403-9e9dbcb8a10c
