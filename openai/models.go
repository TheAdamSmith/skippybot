package openai

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
	AssistantId  string `json:"assistant_id"`
	Instructions string `json:"instructions"`
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
	ID           string                 `json:"id"`
	Object       string                 `json:"object"`
	CreatedAt    int64                  `json:"created_at"`
	AssistantID  string                 `json:"assistant_id"`
	ThreadID     string                 `json:"thread_id"`
	Status       string                 `json:"status"`
	StartedAt    int64                  `json:"started_at"`
	ExpiresAt    *int64                 `json:"expires_at"`   // Nullable field
	CancelledAt  *int64                 `json:"cancelled_at"` // Nullable field
	FailedAt     *int64                 `json:"failed_at"`    // Nullable field
	CompletedAt  int64                  `json:"completed_at"`
	LastError    *string                `json:"last_error"` // Nullable field
	Model        string                 `json:"model"`
	Instructions *string                `json:"instructions"` // Nullable field
	Tools        []Tool                 `json:"tools"`
	FileIDs      []string               `json:"file_ids"`
	Metadata     map[string]interface{} `json:"metadata"`
	Usage        *interface{}           `json:"usage"` // Nullable field, assuming usage could be of any type
}

type Tool struct {
	Type string `json:"type"`
}

