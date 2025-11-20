package cloudwatchlogstools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// ListLogGroupsTool lists CloudWatch log groups
type ListLogGroupsTool struct {
	logsClient *cloudwatchlogs.Client
}

// NewListLogGroupsTool creates a new instance
func NewListLogGroupsTool(awsConfig aws.Config) *ListLogGroupsTool {
	return &ListLogGroupsTool{
		logsClient: cloudwatchlogs.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListLogGroupsTool) Name() string {
	return "list_log_groups"
}

// Description describes the tool
func (t *ListLogGroupsTool) Description() string {
	return "Lists CloudWatch log groups with stored bytes and retention days."
}

// InputSchema returns JSON schema
func (t *ListLogGroupsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"prefix": map[string]interface{}{
				"type":        "string",
				"description": "Optional name prefix to filter log groups.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of log groups to return. Default: 100",
				"minimum":     1,
				"maximum":     500,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of log groups to skip (for pagination). Default: 0",
				"minimum":     0,
				"default":     0,
			},
		},
		"required": []string{},
	}
}

// ListLogGroupsInput parameters
type ListLogGroupsInput struct {
	Prefix string `json:"prefix,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// LogGroupInfo output
type LogGroupInfo struct {
	Name          string  `json:"name"`
	StoredBytes   int64   `json:"stored_bytes"`
	RetentionDays *int32  `json:"retention_days,omitempty"`
	KmsKeyID      *string `json:"kms_key_id,omitempty"`
}

// ListLogGroupsOutput response
type ListLogGroupsOutput struct {
	LogGroups  []LogGroupInfo `json:"log_groups"`
	Count      int            `json:"count"`
	TotalCount int            `json:"total_count"`
	Offset     int            `json:"offset"`
	Limit      int            `json:"limit"`
	HasMore    bool           `json:"has_more"`
}

// Execute lists log groups
func (t *ListLogGroupsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListLogGroupsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(t.logsClient, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(params.Prefix),
	})

	var groups []LogGroupInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe log groups: %w", err)
		}
		for _, g := range page.LogGroups {
			if g.LogGroupName == nil {
				continue
			}
			info := LogGroupInfo{
				Name:        aws.ToString(g.LogGroupName),
				StoredBytes: aws.ToInt64(g.StoredBytes),
				KmsKeyID:    g.KmsKeyId,
			}
			if g.RetentionInDays != nil {
				info.RetentionDays = g.RetentionInDays
			}
			groups = append(groups, info)
		}
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	total := len(groups)
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= total {
		return ListLogGroupsOutput{
			LogGroups:  []LogGroupInfo{},
			Count:      0,
			TotalCount: total,
			Offset:     params.Offset,
			Limit:      params.Limit,
			HasMore:    false,
		}, nil
	}
	if end > total {
		end = total
	}

	selected := groups[start:end]

	return ListLogGroupsOutput{
		LogGroups:  selected,
		Count:      len(selected),
		TotalCount: total,
		Offset:     params.Offset,
		Limit:      params.Limit,
		HasMore:    end < total,
	}, nil
}
