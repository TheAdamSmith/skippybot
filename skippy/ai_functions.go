package skippy

import (
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// TODO: should move all of the json definitions here
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

var GenerateEventTool = openai.Tool{
	Type:     openai.ToolTypeFunction,
	Function: &GenerateEventFuncDef,
}
