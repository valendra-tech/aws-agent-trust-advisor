package models

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  interface{} `json:"tool_calls,omitempty"`
}

type InvokeOptions struct {
	MaxTokens    int
	Temperature  float64
	Tools        []ToolDefinition
	SystemPrompt string
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type ModelInvoker interface {
	Invoke(ctx context.Context, client *bedrockruntime.Client, modelID string, messages []Message) (string, error)
	InvokeWithOptions(ctx context.Context, client *bedrockruntime.Client, modelID string, messages []Message, opts InvokeOptions) (*InvokeResult, error)
	// CreateToolResultMessage creates a message with a tool execution result
	CreateToolResultMessage(toolCallID string, result string) Message
	// CreateAssistantMessageWithToolCalls creates an assistant message that includes tool calls
	CreateAssistantMessageWithToolCalls(content string, toolCalls []ToolCall) Message
}

type InvokeResult struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason string
}

type ToolCall struct {
	ID    string
	Name  string
	Input map[string]interface{}
}
