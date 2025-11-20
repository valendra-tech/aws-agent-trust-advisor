package tools

import (
	"context"
	"encoding/json"
)

// Tool represents a function that can be called by the agent
type Tool interface {
	// Name returns the name of the tool
	Name() string
	// Description returns a description of what the tool does
	Description() string
	// InputSchema returns the JSON schema for the tool's input parameters
	InputSchema() map[string]interface{}
	// Execute runs the tool with the given input and returns the result
	Execute(ctx context.Context, input json.RawMessage) (interface{}, error)
}

// ToolDefinition represents the schema definition of a tool for Bedrock
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolUse represents a request from the model to use a tool
type ToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}
