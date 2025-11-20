package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/container"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/aws"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/bedrock"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/logger"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/tools"
)

const defaultSystemPrompt = `You are an AWS Trust Advisor AI Agent, an expert assistant specialized in AWS cloud services and best practices.

Your primary responsibilities:
- Help users optimize their AWS infrastructure for cost, performance, security, and reliability
- Provide guidance on AWS services and their proper usage
- Execute AWS-related operations using available tools
- Analyze AWS resources and provide recommendations
- Follow AWS Well-Architected Framework principles

Guidelines:
- Be concise and professional in your responses
- When asked to perform AWS operations, use the available tools
- Provide clear explanations of actions you take
- If you're uncertain about an operation, ask for clarification before proceeding
- Always prioritize security and best practices

Terminal Formatting - CRITICAL:
You are outputting to a terminal that supports ANSI escape codes for colors and formatting.

Use these EXACT strings in your output to create colored text:
- Bold text: \x1b[1mtext\x1b[0m
- Green (success/positive): \x1b[32mtext\x1b[0m
- Yellow (warning): \x1b[33mtext\x1b[0m
- Red (error): \x1b[31mtext\x1b[0m
- Blue (headers): \x1b[34mtext\x1b[0m
- Cyan (values/names): \x1b[36mtext\x1b[0m
- Bold Green: \x1b[1;32mtext\x1b[0m
- Bold Blue: \x1b[1;34mtext\x1b[0m
- Reset: \x1b[0m (always use after colored text)

Examples of correct usage:
- "Total: \x1b[1;32m88\x1b[0m buckets"
- "Bucket: \x1b[36mbucket-name\x1b[0m"
- "\x1b[1;34m📊 Section Header\x1b[0m"

IMPORTANT: Write "\x1b" as literal text in your response. The system will convert it to actual escape codes.

Tool Usage Best Practices:
- Use available tools to answer questions about AWS resources
- Request only the fields you actually need to answer the user's question
- Use pagination (limit/offset) for large result sets
- For time-based queries, use relative formats like "-7d", "-30d", "-1M" for convenience
- Combine related fields intelligently based on the question context`

var (
	awsProfile       string
	awsRegion        string
	modelID          string
	interactive      bool
	promptInput      string
	logLevel         string
	maxTokens        int
	temperature      float64
	systemPromptFile string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "aws-agent-trust-advisor",
	Short: "AWS Agent Trust Advisor - AI Agent CLI tool",
	Long:  `AWS Agent Trust Advisor is a CLI tool for creating and managing AI agents with AWS integration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&awsProfile, "profile", "p", "", "AWS profile to use")
	rootCmd.Flags().StringVarP(&awsRegion, "region", "r", "eu-west-1", "AWS region to use")
	rootCmd.Flags().StringVarP(&modelID, "model", "m", "openai.gpt-oss-120b-1:0", "Bedrock model ID to use")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", true, "Start interactive chat mode (default true)")
	rootCmd.Flags().StringVarP(&promptInput, "prompt", "P", "", "Run a single prompt in non-interactive mode and print the response")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, error)")
	rootCmd.Flags().IntVar(&maxTokens, "max-tokens", 2048, "Maximum tokens for model response")
	rootCmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Model temperature (0.0-1.0)")
	rootCmd.Flags().StringVarP(&systemPromptFile, "system-prompt", "s", "", "Path to file containing system prompt")
}

func run() error {
	// Create and build the dependency injection container
	c := container.New()
	if err := c.Build(container.BuildParams{
		AWSProfile: awsProfile,
		AWSRegion:  awsRegion,
		LogLevel:   logLevel,
	}); err != nil {
		return fmt.Errorf("failed to build container: %w", err)
	}

	// Invoke the main application logic with all dependencies
	return c.Invoke(func(log *logger.Logger, awsSvc *aws.Service, bedrockSvc *bedrock.Service, toolRegistry *tools.Registry) error {
		log.Info("AWS Agent Trust Advisor CLI started")
		log.Info("Using model: %s", modelID)

		// Verify AWS connection
		identity, err := awsSvc.GetCallerIdentity(context.Background())
		if err != nil {
			return fmt.Errorf("failed to verify AWS credentials: %w", err)
		}

		log.Info("Connected to AWS Account: %s", *identity.Account)
		log.Info("User ARN: %s", *identity.Arn)

		// Test Bedrock service
		log.Info("AWS Bedrock service initialized successfully")

		// List available tools
		toolList := toolRegistry.List()
		log.Info("Available tools: %d", len(toolList))
		for _, tool := range toolList {
			log.Debug("  - %s: %s", tool.Name(), tool.Description())
		}

		log.Info("All services initialized successfully")

		// Prompt mode overrides interactive
		if promptInput != "" {
			return runPromptMode(log, bedrockSvc, toolRegistry, promptInput)
		}

		// Start interactive mode by default
		if interactive {
			return startInteractiveMode(log, bedrockSvc, toolRegistry)
		}

		log.Info("Interactive mode disabled and no prompt provided. Exiting.")
		return nil
	})
}

func loadSystemPrompt(log *logger.Logger) string {
	// If a custom file is specified, load from file
	if systemPromptFile != "" {
		content, err := os.ReadFile(systemPromptFile)
		if err != nil {
			log.Error("Failed to read system prompt file %s: %v", systemPromptFile, err)
			log.Info("Falling back to default system prompt")
			return defaultSystemPrompt
		}
		prompt := strings.TrimSpace(string(content))
		log.Info("Loaded custom system prompt from %s (%d characters)", systemPromptFile, len(prompt))
		return prompt
	}

	// Use default system prompt
	log.Info("Using default system prompt (%d characters)", len(defaultSystemPrompt))
	return defaultSystemPrompt
}

// convertANSICodes converts escaped ANSI codes to real escape sequences
func convertANSICodes(text string) string {
	// First handle literal \x1b patterns
	replacements := map[string]string{
		`\x1b[0m`:    "\033[0m",
		`\x1b[1m`:    "\033[1m",
		`\x1b[31m`:   "\033[31m",
		`\x1b[32m`:   "\033[32m",
		`\x1b[33m`:   "\033[33m",
		`\x1b[34m`:   "\033[34m",
		`\x1b[36m`:   "\033[36m",
		`\x1b[1;32m`: "\033[1;32m",
		`\x1b[1;33m`: "\033[1;33m",
		`\x1b[1;34m`: "\033[1;34m",
		`\x1b[1;36m`: "\033[1;36m",
	}

	result := text
	for escaped, real := range replacements {
		result = strings.ReplaceAll(result, escaped, real)
	}

	// Handle Unicode escape sequences like \u001b[32m
	// Match \uXXXX[...m patterns where XXXX is the unicode for ESC (001b)
	re := regexp.MustCompile(`\\u001b\[([0-9;]+)m`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the color code
		codeMatch := regexp.MustCompile(`\[([0-9;]+)m`).FindStringSubmatch(match)
		if len(codeMatch) > 1 {
			return fmt.Sprintf("\033[%sm", codeMatch[1])
		}
		return match
	})

	return result
}

func startInteractiveMode(log *logger.Logger, bedrockSvc *bedrock.Service, toolRegistry *tools.Registry) error {
	fmt.Println("\n=== Interactive Chat Mode ===")
	fmt.Println("Type your messages and press Enter to send.")
	fmt.Println("Type 'exit' or 'quit' to end the conversation.")
	fmt.Println("Type 'clear' to clear conversation history.")
	fmt.Println("Type 'tools' to list available tools.")

	// Load system prompt (default or from file)
	systemPrompt := loadSystemPrompt(log)
	if systemPromptFile != "" {
		fmt.Println("Using custom system prompt from file.")
	} else {
		fmt.Println("Using default system prompt.")
	}
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	conversationHistory := []bedrock.Message{}

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("\nGoodbye!")
			break
		}

		if input == "clear" {
			conversationHistory = []bedrock.Message{}
			fmt.Println("\nConversation history cleared.")
			continue
		}

		if input == "tools" {
			fmt.Println("\nAvailable Tools:")
			for _, tool := range toolRegistry.List() {
				fmt.Printf("  - %s: %s\n", tool.Name(), tool.Description())
			}
			fmt.Println()
			continue
		}

		// Add user message to history
		conversationHistory = append(conversationHistory, bedrock.Message{
			Role:    "user",
			Content: input,
		})

		// Get tool definitions
		toolDefs := convertToolDefinitions(toolRegistry.GetDefinitions())

		// Invoke with tools and handle tool calls in a loop
		fmt.Print("Assistant: ")
		response, err := invokeWithToolLoop(context.Background(), log, bedrockSvc, toolRegistry, modelID, conversationHistory, toolDefs, systemPrompt)
		if err != nil {
			log.Error("Failed to get response: %v", err)
			fmt.Printf("Error: %v\n\n", err)
			// Remove the failed message from history
			conversationHistory = conversationHistory[:len(conversationHistory)-1]
			continue
		}

		// Convert escaped ANSI codes to real ones and print
		formattedResponse := convertANSICodes(response)
		fmt.Print(formattedResponse)
		fmt.Print("\n\n")

		// Add assistant response to history
		conversationHistory = append(conversationHistory, bedrock.Message{
			Role:    "assistant",
			Content: response,
		})
	}

	return nil
}

func runPromptMode(log *logger.Logger, bedrockSvc *bedrock.Service, toolRegistry *tools.Registry, prompt string) error {
	log.Info("Running single-prompt mode")

	// Load system prompt
	systemPrompt := loadSystemPrompt(log)

	// Prepare conversation with single user message
	messages := []bedrock.Message{
		{Role: "user", Content: prompt},
	}

	toolDefs := convertToolDefinitions(toolRegistry.GetDefinitions())

	response, err := invokeWithToolLoop(context.Background(), log, bedrockSvc, toolRegistry, modelID, messages, toolDefs, systemPrompt)
	if err != nil {
		return fmt.Errorf("failed to run prompt: %w", err)
	}

	fmt.Println(convertANSICodes(response))
	return nil
}

func convertToolDefinitions(toolDefs []tools.ToolDefinition) []bedrock.ToolDefinition {
	result := make([]bedrock.ToolDefinition, len(toolDefs))
	for i, td := range toolDefs {
		result[i] = bedrock.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		}
	}
	return result
}

func invokeWithToolLoop(ctx context.Context, log *logger.Logger, bedrockSvc *bedrock.Service, toolRegistry *tools.Registry, modelID string, messages []bedrock.Message, toolDefs []bedrock.ToolDefinition, systemPrompt string) (string, error) {
	maxIterations := 10
	var finalResponse string

	for i := 0; i < maxIterations; i++ {
		result, err := bedrockSvc.InvokeWithTools(ctx, modelID, messages, bedrock.InvokeOptions{
			MaxTokens:    maxTokens,
			Temperature:  temperature,
			Tools:        toolDefs,
			SystemPrompt: systemPrompt,
		})
		if err != nil {
			return "", err
		}

		// If no tool calls, collect the content and we're done
		if len(result.ToolCalls) == 0 {
			if result.Content != "" {
				finalResponse += result.Content
			}
			break
		}

		// If there are tool calls, don't collect pre-execution content
		// (some models hallucinate responses before executing tools)

		// Add assistant message with tool calls to history
		if len(result.ToolCalls) > 0 {
			assistantMsg := bedrockSvc.CreateAssistantMessageWithToolCalls(modelID, result.Content, result.ToolCalls)
			messages = append(messages, assistantMsg)
		}

		// Execute tool calls
		log.Info("Model requested %d tool call(s)", len(result.ToolCalls))
		for _, toolCall := range result.ToolCalls {
			log.Info("Executing tool: %s", toolCall.Name)
			fmt.Printf("\n[Using tool: %s]\n", toolCall.Name)

			// Convert tool input to JSON
			inputJSON, err := json.Marshal(toolCall.Input)
			if err != nil {
				log.Error("Failed to marshal tool input: %v", err)
				continue
			}

			// Execute the tool
			toolResult, err := toolRegistry.Execute(ctx, toolCall.Name, inputJSON)
			if err != nil {
				log.Error("Tool execution failed: %v", err)
				// Add error result to conversation so the model can react
				errorMsg := bedrockSvc.CreateToolResultMessage(modelID, toolCall.ID, fmt.Sprintf("error: %v", err))
				messages = append(messages, errorMsg)
				// Surface error to local user
				fmt.Printf("Tool error: %v\n", err)
				continue
			}

			// Convert result to JSON string
			resultJSON, err := json.Marshal(toolResult)
			if err != nil {
				log.Error("Failed to marshal tool result: %v", err)
				continue
			}

			log.Debug("Tool result: %s", string(resultJSON))

			// Add tool result to conversation using model-specific format
			resultMsg := bedrockSvc.CreateToolResultMessage(modelID, toolCall.ID, string(resultJSON))
			messages = append(messages, resultMsg)
		}
	}

	return finalResponse, nil
}
