package cloudtrailtools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

// LookupEventsTool searches recent CloudTrail events
type LookupEventsTool struct {
	client *cloudtrail.Client
}

// NewLookupEventsTool creates a new instance
func NewLookupEventsTool(awsConfig aws.Config) *LookupEventsTool {
	return &LookupEventsTool{
		client: cloudtrail.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *LookupEventsTool) Name() string {
	return "lookup_cloudtrail_events"
}

// Description describes the tool
func (t *LookupEventsTool) Description() string {
	return "Searches CloudTrail events with optional filters (event name, username, resource name/type, time range) and returns the latest matches."
}

// InputSchema returns JSON schema
func (t *LookupEventsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{},
		"properties": map[string]interface{}{
			"event_name": map[string]interface{}{
				"type":        "string",
				"description": "Optional event name to filter (e.g., RunInstances).",
			},
			"username": map[string]interface{}{
				"type":        "string",
				"description": "Optional username/role to filter.",
			},
			"resource_name": map[string]interface{}{
				"type":        "string",
				"description": "Optional resource name/ID to filter.",
			},
			"resource_type": map[string]interface{}{
				"type":        "string",
				"description": "Optional resource type to filter (e.g., AWS::EC2::Instance).",
			},
			"start_time": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "ISO8601 start time. Default: now - 24h.",
			},
			"end_time": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "ISO8601 end time. Default: now.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum events to return (CloudTrail max 50 per page). Default: 50",
				"minimum":     1,
				"maximum":     200,
				"default":     50,
			},
		},
	}
}

// LookupEventsInput parameters
type LookupEventsInput struct {
	EventName    string `json:"event_name,omitempty"`
	Username     string `json:"username,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	StartTime    string `json:"start_time,omitempty"`
	EndTime      string `json:"end_time,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

// CloudTrailEvent output
type CloudTrailEvent struct {
	EventID        string   `json:"event_id"`
	EventName      string   `json:"event_name"`
	EventTime      string   `json:"event_time"`
	Username       *string  `json:"username,omitempty"`
	ReadOnly       *string  `json:"read_only,omitempty"`
	EventSource    *string  `json:"event_source,omitempty"`
	Resources      []string `json:"resources,omitempty"`
	CloudTrailData *string  `json:"cloudtrail_event,omitempty"`
}

// LookupEventsOutput response
type LookupEventsOutput struct {
	Events []CloudTrailEvent `json:"events"`
	Count  int               `json:"count"`
}

// Execute searches events
func (t *LookupEventsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params LookupEventsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	if params.Limit == 0 {
		params.Limit = 50
	}

	end := time.Now().UTC()
	if params.EndTime != "" {
		val, err := time.Parse(time.RFC3339, params.EndTime)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time: %w", err)
		}
		end = val
	}
	start := end.Add(-24 * time.Hour)
	if params.StartTime != "" {
		val, err := time.Parse(time.RFC3339, params.StartTime)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time: %w", err)
		}
		start = val
	}

	var lookups []types.LookupAttribute
	if params.EventName != "" {
		lookups = append(lookups, types.LookupAttribute{
			AttributeKey:   types.LookupAttributeKeyEventName,
			AttributeValue: aws.String(params.EventName),
		})
	}
	if params.Username != "" {
		lookups = append(lookups, types.LookupAttribute{
			AttributeKey:   types.LookupAttributeKeyUsername,
			AttributeValue: aws.String(params.Username),
		})
	}
	if params.ResourceName != "" {
		lookups = append(lookups, types.LookupAttribute{
			AttributeKey:   types.LookupAttributeKeyResourceName,
			AttributeValue: aws.String(params.ResourceName),
		})
	}
	if params.ResourceType != "" {
		lookups = append(lookups, types.LookupAttribute{
			AttributeKey:   types.LookupAttributeKeyResourceType,
			AttributeValue: aws.String(params.ResourceType),
		})
	}

	paginator := cloudtrail.NewLookupEventsPaginator(t.client, &cloudtrail.LookupEventsInput{
		LookupAttributes: lookups,
		StartTime:        aws.Time(start),
		EndTime:          aws.Time(end),
		MaxResults:       aws.Int32(int32(minInt(params.Limit, 50))),
	})

	var events []CloudTrailEvent
	for paginator.HasMorePages() && len(events) < params.Limit {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup CloudTrail events: %w", err)
		}
		for _, e := range page.Events {
			ctEvent := CloudTrailEvent{
				EventID:     aws.ToString(e.EventId),
				EventName:   aws.ToString(e.EventName),
				EventSource: e.EventSource,
				Username:    e.Username,
			}
			if e.EventTime != nil {
				ctEvent.EventTime = e.EventTime.Format(time.RFC3339)
			}
			if e.ReadOnly != nil {
				ctEvent.ReadOnly = e.ReadOnly
			}
			if len(e.Resources) > 0 {
				for _, r := range e.Resources {
					if r.ResourceName != nil {
						ctEvent.Resources = append(ctEvent.Resources, *r.ResourceName)
					}
				}
			}
			if e.CloudTrailEvent != nil {
				ctEvent.CloudTrailData = e.CloudTrailEvent
			}
			events = append(events, ctEvent)
			if len(events) >= params.Limit {
				break
			}
		}
	}

	return LookupEventsOutput{
		Events: events,
		Count:  len(events),
	}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
