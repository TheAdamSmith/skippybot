package discord

type Thread struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"`
	CreatedAt int64             `json:"created_at"`
	Metadata  map[string]string `json:"metadata"`
}

type MessageReq struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RunReq struct {
	AssistantId            string `json:"assistant_id"`
	Instructions           string `json:"instructions,omitempty"`
	AdditionalInstructions string `json:"additional_instructions,omitempty"`
}

type MessageList struct {
	Object  string    `json:"object"`
	Data    []Message `json:"data"`
	FirstID string    `json:"first_id"`
	LastID  string    `json:"last_id"`
	HasMore bool      `json:"has_more"`
}

type Message struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	CreatedAt   int64                  `json:"created_at"`
	AssistantID *string                `json:"assistant_id"` // Using *string to handle null values
	ThreadID    string                 `json:"thread_id"`
	RunID       *string                `json:"run_id"` // Using *string to handle null values
	Role        string                 `json:"role"`
	Content     []Content              `json:"content"`
	FileIDs     []string               `json:"file_ids"`
	Metadata    map[string]interface{} `json:"metadata"` // Using interface{} for flexibility
}

type Content struct {
	Type string `json:"type"`
	Text Text   `json:"text"`
}

type Text struct {
	Value       string        `json:"value"`
	Annotations []interface{} `json:"annotations"` // Using interface{} for flexible data types
}

type Run struct {
	ID             string                 `json:"id"`
	Object         string                 `json:"object"`
	CreatedAt      int64                  `json:"created_at"`
	AssistantID    string                 `json:"assistant_id"`
	ThreadID       string                 `json:"thread_id"`
	Status         string                 `json:"status"`
	StartedAt      int64                  `json:"started_at"`
	ExpiresAt      *int64                 `json:"expires_at"`      // Nullable field
	RequiredAction RequiredAction         `json:"required_action"` // Nullable field
	CancelledAt    *int64                 `json:"cancelled_at"`    // Nullable field
	FailedAt       *int64                 `json:"failed_at"`       // Nullable field
	CompletedAt    int64                  `json:"completed_at"`
	LastError      *string                `json:"last_error"` // Nullable field
	Model          string                 `json:"model"`
	Instructions   *string                `json:"instructions"` // Nullable field
	Tools          []Tool                 `json:"tools"`
	FileIDs        []string               `json:"file_ids"`
	Metadata       map[string]interface{} `json:"metadata"`
	Usage          *interface{}           `json:"usage"` // Nullable field, assuming usage could be of any type
}

type FuncArgs struct {
	JsonValue string
	FuncName  string
	ToolID    string
}

func (r Run) GetFunctionArgs() []FuncArgs {
	toolCalls := r.RequiredAction.SubmitToolOutputs.ToolCalls
	result := make([]FuncArgs, len(toolCalls))
	for i, toolCall := range toolCalls {
		result[i] = FuncArgs{
			FuncName:  toolCall.Function.Name,
			JsonValue: toolCall.Function.Arguments,
			ToolID:    toolCall.ID,
		}
	}
	return result
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}
type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}
type RequiredAction struct {
	Type              string            `json:"type"`
	SubmitToolOutputs SubmitToolOutputs `json:"submit_tool_outputs"`
}

type SubmitToolOutputs struct {
	ToolCalls []ToolCall `json:"tool_calls"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolOutput struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
}

type ToolResponse struct {
	ToolOutputs []ToolOutput `json:"tool_outputs"`
}
