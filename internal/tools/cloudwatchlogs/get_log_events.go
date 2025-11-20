package cloudwatchlogstools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// GetLogEventsTool fetches log events from a group (optionally a stream or filter pattern)
type GetLogEventsTool struct {
	logsClient *cloudwatchlogs.Client
}

// NewGetLogEventsTool creates a new instance
func NewGetLogEventsTool(awsConfig aws.Config) *GetLogEventsTool {
	return &GetLogEventsTool{
		logsClient: cloudwatchlogs.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *GetLogEventsTool) Name() string {
	return "get_log_events"
}

// Description describes the tool
func (t *GetLogEventsTool) Description() string {
	return "Fetches CloudWatch Logs events for a log group with optional stream, filter pattern, time range, and limit."
}

// InputSchema returns JSON schema
func (t *GetLogEventsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"log_group"},
		"properties": map[string]interface{}{
			"log_group": map[string]interface{}{
				"type":        "string",
				"description": "Log group name.",
			},
			"log_stream": map[string]interface{}{
				"type":        "string",
				"description": "Optional log stream name. If omitted, the most recent streams are searched.",
			},
			"filter_pattern": map[string]interface{}{
				"type":        "string",
				"description": "Optional filter pattern for FilterLogEvents.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of events to return. Default: 100",
				"minimum":     1,
				"maximum":     10000,
				"default":     100,
			},
			"start_time_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Optional start time in milliseconds since epoch.",
			},
			"end_time_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Optional end time in milliseconds since epoch.",
			},
		},
	}
}

// GetLogEventsInput parameters
type GetLogEventsInput struct {
	LogGroup      string `json:"log_group"`
	LogStream     string `json:"log_stream,omitempty"`
	FilterPattern string `json:"filter_pattern,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	StartTimeMs   int64  `json:"start_time_ms,omitempty"`
	EndTimeMs     int64  `json:"end_time_ms,omitempty"`
}

// LogEvent represents a log event
type LogEvent struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
	LogStream string `json:"log_stream"`
	Ingestion int64  `json:"ingestion_time"`
}

// GetLogEventsOutput response
type GetLogEventsOutput struct {
	Events []LogEvent `json:"events"`
	Count  int        `json:"count"`
}

// Execute fetches events
func (t *GetLogEventsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params GetLogEventsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	if params.LogGroup == "" {
		return nil, fmt.Errorf("log_group is required")
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	var events []LogEvent

	if params.FilterPattern != "" || params.LogStream == "" {
		// Use FilterLogEvents (supports filter and automatic stream search)
		paginator := cloudwatchlogs.NewFilterLogEventsPaginator(t.logsClient, &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  aws.String(params.LogGroup),
			FilterPattern: aws.String(params.FilterPattern),
			Limit:         aws.Int32(int32(params.Limit)),
			StartTime:     toPtr(params.StartTimeMs),
			EndTime:       toPtr(params.EndTimeMs),
		})
		for paginator.HasMorePages() && len(events) < params.Limit {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to filter log events: %w", err)
			}
			for _, e := range page.Events {
				events = append(events, LogEvent{
					Timestamp: aws.ToInt64(e.Timestamp),
					Message:   aws.ToString(e.Message),
					LogStream: aws.ToString(e.LogStreamName),
					Ingestion: aws.ToInt64(e.IngestionTime),
				})
				if len(events) >= params.Limit {
					break
				}
			}
		}
	} else {
		// Specific stream
		paginator := cloudwatchlogs.NewGetLogEventsPaginator(t.logsClient, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(params.LogGroup),
			LogStreamName: aws.String(params.LogStream),
			Limit:         aws.Int32(int32(params.Limit)),
			StartTime:     toPtr(params.StartTimeMs),
			EndTime:       toPtr(params.EndTimeMs),
		})
		for paginator.HasMorePages() && len(events) < params.Limit {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get log events: %w", err)
			}
			for _, e := range page.Events {
				events = append(events, LogEvent{
					Timestamp: aws.ToInt64(e.Timestamp),
					Message:   aws.ToString(e.Message),
					LogStream: params.LogStream,
					Ingestion: aws.ToInt64(e.IngestionTime),
				})
				if len(events) >= params.Limit {
					break
				}
			}
		}
	}

	return GetLogEventsOutput{
		Events: events,
		Count:  len(events),
	}, nil
}

func toPtr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return aws.Int64(v)
}
