package vpctools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ListFlowLogsTool lists VPC flow logs
type ListFlowLogsTool struct {
	ec2Client *ec2.Client
}

// NewListFlowLogsTool creates a new instance
func NewListFlowLogsTool(awsConfig aws.Config) *ListFlowLogsTool {
	return &ListFlowLogsTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListFlowLogsTool) Name() string {
	return "list_flow_logs"
}

// Description describes the tool
func (t *ListFlowLogsTool) Description() string {
	return "Lists VPC flow logs with their resource, status, destination (CloudWatch or S3), and traffic type."
}

// InputSchema returns JSON schema
func (t *ListFlowLogsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional resource ID filter (VPC, subnet, ENI).",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum flow logs to return. Default: 100",
				"minimum":     1,
				"maximum":     1000,
				"default":     100,
			},
		},
	}
}

// FlowLogInfo output
type FlowLogInfo struct {
	FlowLogID      string  `json:"flow_log_id"`
	ResourceID     string  `json:"resource_id"`
	TrafficType    string  `json:"traffic_type"`
	LogDestination string  `json:"log_destination"`
	LogFormat      *string `json:"log_format,omitempty"`
	DeliverStatus  *string `json:"delivery_status,omitempty"`
	MaxAggregation *int32  `json:"max_aggregation_interval_sec,omitempty"`
}

// ListFlowLogsOutput response
type ListFlowLogsOutput struct {
	FlowLogs []FlowLogInfo `json:"flow_logs"`
	Count    int           `json:"count"`
	HasMore  bool          `json:"has_more"`
}

// Execute lists flow logs
func (t *ListFlowLogsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		ResourceID string `json:"resource_id,omitempty"`
		Limit      int    `json:"limit,omitempty"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	var filters []types.Filter
	if params.ResourceID != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("resource-id"),
			Values: []string{params.ResourceID},
		})
	}

	paginator := ec2.NewDescribeFlowLogsPaginator(t.ec2Client, &ec2.DescribeFlowLogsInput{
		Filter: filters,
	})

	var items []FlowLogInfo
	for paginator.HasMorePages() && len(items) < params.Limit {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe flow logs: %w", err)
		}
		for _, fl := range page.FlowLogs {
			if fl.FlowLogId == nil || fl.ResourceId == nil {
				continue
			}
			dest := aws.ToString(fl.LogDestination)
			if fl.LogDestinationType == types.LogDestinationTypeCloudWatchLogs && fl.LogGroupName != nil {
				dest = fmt.Sprintf("cw:%s", aws.ToString(fl.LogGroupName))
			}
			info := FlowLogInfo{
				FlowLogID:      aws.ToString(fl.FlowLogId),
				ResourceID:     aws.ToString(fl.ResourceId),
				TrafficType:    string(fl.TrafficType),
				LogDestination: dest,
				LogFormat:      fl.LogFormat,
				DeliverStatus:  fl.DeliverLogsStatus,
			}
			if fl.MaxAggregationInterval != nil {
				info.MaxAggregation = fl.MaxAggregationInterval
			}
			items = append(items, info)
			if len(items) >= params.Limit {
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].FlowLogID < items[j].FlowLogID })

	return ListFlowLogsOutput{
		FlowLogs: items,
		Count:    len(items),
		HasMore:  paginator.HasMorePages(),
	}, nil
}
