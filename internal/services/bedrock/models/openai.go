package models

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

type OpenAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type OpenAIRequest struct {
	Messages    []interface{} `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature,omitempty"`
	Tools       []OpenAITool  `json:"tools,omitempty"`
	ToolChoice  string        `json:"tool_choice,omitempty"`
}

type OpenAIResponseMessage struct {
	Content   string                `json:"content"`
	ToolCalls []OpenAIToolCallBlock `json:"tool_calls,omitempty"`
}

type OpenAIToolCallBlock struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message      OpenAIResponseMessage `json:"message"`
		FinishReason string                `json:"finish_reason"`
	} `json:"choices"`
}

type OpenAIInvoker struct{}

func (o *OpenAIInvoker) Invoke(ctx context.Context, client *bedrockruntime.Client, modelID string, messages []Message) (string, error) {
	result, err := o.InvokeWithOptions(ctx, client, modelID, messages, InvokeOptions{
		MaxTokens:   2048,
		Temperature: 0.7,
	})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func (o *OpenAIInvoker) InvokeWithOptions(ctx context.Context, client *bedrockruntime.Client, modelID string, messages []Message, opts InvokeOptions) (*InvokeResult, error) {
	// Convert messages to OpenAI format (interface{} to support both regular and tool messages)
	// If system prompt is provided, add it as the first message
	startIdx := 0
	if opts.SystemPrompt != "" {
		startIdx = 1
	}

	openaiMessages := make([]interface{}, len(messages)+startIdx)

	// Add system prompt as first message if provided
	if opts.SystemPrompt != "" {
		openaiMessages[0] = map[string]interface{}{
			"role":    "system",
			"content": opts.SystemPrompt,
		}
	}

	for i, msg := range messages {
		msgMap := map[string]interface{}{
			"role": msg.Role,
		}

		// Add content if present
		if msg.Content != "" {
			msgMap["content"] = msg.Content
		}

		// Add tool_call_id for tool role messages
		if msg.ToolCallID != "" {
			msgMap["tool_call_id"] = msg.ToolCallID
		}

		// Add tool_calls for assistant messages with tool calls
		if msg.ToolCalls != nil {
			msgMap["tool_calls"] = msg.ToolCalls
		}

		openaiMessages[i+startIdx] = msgMap
	}

	request := OpenAIRequest{
		Messages:    openaiMessages,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
	}

	// Add tools if provided
	if len(opts.Tools) > 0 {
		openaiTools := make([]OpenAITool, len(opts.Tools))
		for i, tool := range opts.Tools {
			openaiTools[i] = OpenAITool{
				Type: "function",
				Function: OpenAIToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
		}
		request.Tools = openaiTools
		request.ToolChoice = "auto"
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	output, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        requestBody,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to invoke model: %w", err)
	}

	var response OpenAIResponse
	if err := json.Unmarshal(output.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := response.Choices[0]

	// Clean content by removing reasoning tags if present
	content := choice.Message.Content
	// Remove <reasoning>...</reasoning> tags
	if strings.Contains(content, "<reasoning>") {
		re := regexp.MustCompile(`<reasoning>.*?</reasoning>`)
		content = re.ReplaceAllString(content, "")
		content = strings.TrimSpace(content)
	}

	result := &InvokeResult{
		Content:    content,
		StopReason: choice.FinishReason,
	}

	// Parse tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			// Handle empty or malformed arguments
			if tc.Function.Arguments == "" || tc.Function.Arguments == "{}" || tc.Function.Arguments == `{"":{}` {
				args = make(map[string]interface{})
			} else {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					// If arguments are malformed, use empty object
					fmt.Printf("Warning: failed to parse tool arguments '%s': %v\n", tc.Function.Arguments, err)
					args = make(map[string]interface{})
				}
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: args,
			})
		}
	}

	return result, nil
}

func (o *OpenAIInvoker) CreateToolResultMessage(toolCallID string, result string) Message {
	return Message{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    result,
	}
}

func (o *OpenAIInvoker) CreateAssistantMessageWithToolCalls(content string, toolCalls []ToolCall) Message {
	// Convert tool calls to OpenAI format
	toolCallsJSON := make([]map[string]interface{}, len(toolCalls))
	for i, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Input)
		toolCallsJSON[i] = map[string]interface{}{
			"id":   tc.ID,
			"type": "function",
			"function": map[string]interface{}{
				"name":      tc.Name,
				"arguments": string(argsJSON),
			},
		}
	}
	return Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCallsJSON,
	}
}
