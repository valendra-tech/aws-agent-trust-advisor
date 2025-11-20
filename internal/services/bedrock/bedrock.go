package bedrock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/bedrock/models"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/logger"
)

type Service struct {
	client     *bedrockruntime.Client
	logger     *logger.Logger
	invokers   map[string]models.ModelInvoker
	maxRetries int
	retryDelay time.Duration
}

type Message = models.Message

func New(awsConfig aws.Config, log *logger.Logger) *Service {
	log.Info("Initializing AWS Bedrock service")
	client := bedrockruntime.NewFromConfig(awsConfig)

	invokers := map[string]models.ModelInvoker{
		"openai": &models.OpenAIInvoker{},
	}

	return &Service{
		client:     client,
		logger:     log,
		invokers:   invokers,
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}
}

func (s *Service) InvokeModel(ctx context.Context, modelID string, prompt string) (string, error) {
	messages := []Message{{Role: "user", Content: prompt}}
	return s.InvokeModelWithMessages(ctx, modelID, messages)
}

func (s *Service) InvokeModelWithMessages(ctx context.Context, modelID string, messages []Message) (string, error) {
	s.logger.Debug("Invoking Bedrock model: %s with %d messages", modelID, len(messages))

	// Determine model provider from model ID prefix
	var invoker models.ModelInvoker
	var found bool

	for prefix, inv := range s.invokers {
		if strings.HasPrefix(modelID, prefix+".") {
			invoker = inv
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("unsupported model type: %s", modelID)
	}

	// Retry logic with exponential backoff
	var response string
	var err error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			delay := s.retryDelay * time.Duration(1<<uint(attempt-1)) // Exponential backoff: 1s, 2s, 4s
			s.logger.Info("Retrying Bedrock invocation (attempt %d/%d) after %v", attempt+1, s.maxRetries+1, delay)
			time.Sleep(delay)
		}

		response, err = invoker.Invoke(ctx, s.client, modelID, messages)
		if err == nil {
			if attempt > 0 {
				s.logger.Info("Model invoked successfully after %d retries", attempt)
			} else {
				s.logger.Debug("Model invoked successfully")
			}
			return response, nil
		}

		s.logger.Error("Failed to invoke model (attempt %d/%d): %v", attempt+1, s.maxRetries+1, err)
	}

	return "", fmt.Errorf("failed after %d attempts: %w", s.maxRetries+1, err)
}

type InvokeOptions = models.InvokeOptions
type InvokeResult = models.InvokeResult
type ToolDefinition = models.ToolDefinition

func (s *Service) InvokeWithTools(ctx context.Context, modelID string, messages []Message, opts InvokeOptions) (*InvokeResult, error) {
	s.logger.Debug("Invoking Bedrock model: %s with %d messages and %d tools", modelID, len(messages), len(opts.Tools))

	// Determine model provider from model ID prefix
	var invoker models.ModelInvoker
	var found bool

	for prefix, inv := range s.invokers {
		if strings.HasPrefix(modelID, prefix+".") {
			invoker = inv
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("unsupported model type: %s", modelID)
	}

	// Retry logic with exponential backoff
	var result *InvokeResult
	var err error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			delay := s.retryDelay * time.Duration(1<<uint(attempt-1)) // Exponential backoff: 1s, 2s, 4s
			s.logger.Info("Retrying Bedrock invocation with tools (attempt %d/%d) after %v", attempt+1, s.maxRetries+1, delay)
			time.Sleep(delay)
		}

		result, err = invoker.InvokeWithOptions(ctx, s.client, modelID, messages, opts)
		if err == nil {
			if attempt > 0 {
				s.logger.Info("Model invoked successfully with tools after %d retries, stop_reason: %s, tool_calls: %d", attempt, result.StopReason, len(result.ToolCalls))
			} else {
				s.logger.Debug("Model invoked successfully, stop_reason: %s, tool_calls: %d", result.StopReason, len(result.ToolCalls))
			}
			return result, nil
		}

		s.logger.Error("Failed to invoke model with tools (attempt %d/%d): %v", attempt+1, s.maxRetries+1, err)
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", s.maxRetries+1, err)
}

// CreateToolResultMessage creates a message with a tool execution result using the appropriate format for the model
func (s *Service) CreateToolResultMessage(modelID string, toolCallID string, result string) Message {
	var invoker models.ModelInvoker
	for prefix, inv := range s.invokers {
		if strings.HasPrefix(modelID, prefix+".") {
			invoker = inv
			break
		}
	}
	if invoker == nil {
		// Fallback to generic message
		return Message{Role: "user", Content: result}
	}
	return invoker.CreateToolResultMessage(toolCallID, result)
}

// CreateAssistantMessageWithToolCalls creates an assistant message that includes tool calls
func (s *Service) CreateAssistantMessageWithToolCalls(modelID string, content string, toolCalls []models.ToolCall) Message {
	var invoker models.ModelInvoker
	for prefix, inv := range s.invokers {
		if strings.HasPrefix(modelID, prefix+".") {
			invoker = inv
			break
		}
	}
	if invoker == nil {
		// Fallback to generic message
		return Message{Role: "assistant", Content: content}
	}
	return invoker.CreateAssistantMessageWithToolCalls(content, toolCalls)
}
