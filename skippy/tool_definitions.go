package skippy

import (
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

var ALL_TOOLS = []openai.Tool{
	ToggleMorningMessageTool,
	GetStockPricesTool,
	GetWeatherTool,
	GenerateEventTool,
	SetReminderTool,
	GenerateImageTool,
}

var NO_TOOLS = []openai.Tool{}

var GetStockPricesTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &GetStockPriceFuncDef,
}

var GetStockPriceFuncDef = openai.FunctionDefinition{
	Name:        "get_stock_price",
	Description: "Get the current stock price",
	Parameters: &jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"symbol": {
				Type:        jsonschema.String,
				Description: "The stock symbol",
			},
		},
		Required: []string{
			"symbol",
		},
	},
}

var GetWeatherTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &GetWeatherFuncDef,
}

var GetWeatherFuncDef = openai.FunctionDefinition{
	Name:        "get_weather",
	Description: "Get the weather from a location",
	Parameters: &jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"location": {
				Type:        jsonschema.String,
				Description: "The location as a query parameter for Weather API",
			},
		},
		Required: []string{
			"location",
		},
	},
}

var GenerateEventTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &GenerateEventFuncDef,
}

var GenerateEventFuncDef = openai.FunctionDefinition{
	Name:        "generate_event",
	Description: "generate the name, description, and notification message",
	Parameters: &jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"name": {
				Type:        jsonschema.String,
				Description: "name of the event. come up with something interesting",
			},
			"description": {
				Type:        jsonschema.String,
				Description: "a description worthy of skippy's magnificence. do not include user id's in this field",
			},
			"notification_message": {
				Type:        jsonschema.String,
				Description: "the message that will be sent in the channel saying that you have scheduled this event. if user mentions are available please include them in the message",
			},
		},
		Required: []string{
			"name",
			"description",
			"notification_message",
		},
	},
}

var SetReminderTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &SetReminderFuncDef,
}

var SetReminderFuncDef = openai.FunctionDefinition{
	Name:        "set_reminder",
	Description: "Send a message with a timer. If a user id is present, be sure to include it in the message",
	Parameters: &jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"timer_length": {
				Type:        jsonschema.Number,
				Description: "Timer length in seconds",
			},
			"user_id": {
				Type:        jsonschema.String,
				Description: "ID of the user that requested the reminder",
			},
			"message": {
				Type:        jsonschema.String,
				Description: "The message to be sent full of Skippy's classic wit and lots of sarcasm. Reference the user ID as if it were a name in the message",
			},
		},
		Required: []string{
			"message",
			"timer_length",
		},
	},
}

var GenerateImageTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &GenerateImageFuncDef,
}

var GenerateImageFuncDef = openai.FunctionDefinition{
	Name:        "generate_image",
	Description: "Call a function to generate an image using DALL-E",
	Parameters: &jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"prompt": {
				Type:        jsonschema.String,
				Description: "The prompt provided by the user",
			},
		},
		Required: []string{
			"prompt",
		},
	},
}

var ToggleMorningMessageTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &ToggleMorningMessageFuncDef,
}

var ToggleMorningMessageFuncDef = openai.FunctionDefinition{
	Name:        "set_morning_message",
	Description: "Turn on/off the bot's morning message. Set this regardless of the requested time of day. Do not ask follow up questions. If this function is called, get_weather and get_stock_price should not be called",
	Parameters: &jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"enable": {
				Type:        jsonschema.Boolean,
				Description: "Is the morning message enabled",
			},
			"time": {
				Type:        jsonschema.String,
				Description: "The time of day to send the morning message in 24HR format. Does not have to be in the morning",
			},
			"weather_locations": {
				Type:        jsonschema.Array,
				Description: "The list of locations to get the weather for in the morning message",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"stocks": {
				Type:        jsonschema.Array,
				Description: "List of tickers to get the price for in the morning",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
		},
		Required: []string{
			"enable",
		},
	},
}
