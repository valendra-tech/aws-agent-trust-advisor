package elbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// ListTargetGroupsTool lists Target Groups with flexible configuration
type ListTargetGroupsTool struct {
	elbClient *elasticloadbalancingv2.Client
}

// NewListTargetGroupsTool creates a new instance
func NewListTargetGroupsTool(awsConfig aws.Config) *ListTargetGroupsTool {
	return &ListTargetGroupsTool{
		elbClient: elasticloadbalancingv2.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListTargetGroupsTool) Name() string {
	return "list_target_groups"
}

// Description returns a description of what the tool does
func (t *ListTargetGroupsTool) Description() string {
	return "Lists Target Groups with flexible field selection. Returns details like health check settings, target type, port, protocol, VPC, and associated load balancers."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListTargetGroupsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"load_balancer_arn": map[string]interface{}{
				"type":        "string",
				"description": "Filter target groups by load balancer ARN. If provided, only returns target groups associated with this load balancer.",
			},
			"fields": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"name", "arn", "target_type", "protocol", "port", "vpc_id", "health_check_protocol", "health_check_port", "health_check_path", "health_check_enabled", "health_check_interval", "load_balancer_arns", "target_health"},
				},
				"description": "Fields to include: name (always included), arn, target_type, protocol, port, vpc_id, health_check_protocol, health_check_port, health_check_path, health_check_enabled, health_check_interval, load_balancer_arns, target_health. Default: [name, target_type, protocol, port]",
				"default":     []string{"name", "target_type", "protocol", "port"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of target groups to return. Default: 100",
				"minimum":     1,
				"maximum":     400,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of target groups to skip for pagination. Default: 0",
				"minimum":     0,
				"default":     0,
			},
		},
		"required": []string{},
	}
}

// ListTargetGroupsInput represents the input parameters
type ListTargetGroupsInput struct {
	LoadBalancerARN string   `json:"load_balancer_arn,omitempty"`
	Fields          []string `json:"fields,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	Offset          int      `json:"offset,omitempty"`
}

// TargetHealth represents health status of targets
type TargetHealth struct {
	HealthyCount   int `json:"healthy_count"`
	UnhealthyCount int `json:"unhealthy_count"`
	TotalCount     int `json:"total_count"`
}

// TargetGroupInfo represents information about a target group
type TargetGroupInfo struct {
	Name                string        `json:"name"`
	ARN                 *string       `json:"arn,omitempty"`
	TargetType          *string       `json:"target_type,omitempty"`
	Protocol            *string       `json:"protocol,omitempty"`
	Port                *int32        `json:"port,omitempty"`
	VpcID               *string       `json:"vpc_id,omitempty"`
	HealthCheckProtocol *string       `json:"health_check_protocol,omitempty"`
	HealthCheckPort     *string       `json:"health_check_port,omitempty"`
	HealthCheckPath     *string       `json:"health_check_path,omitempty"`
	HealthCheckEnabled  *bool         `json:"health_check_enabled,omitempty"`
	HealthCheckInterval *int32        `json:"health_check_interval,omitempty"`
	LoadBalancerARNs    []string      `json:"load_balancer_arns,omitempty"`
	TargetHealth        *TargetHealth `json:"target_health,omitempty"`
}

// ListTargetGroupsOutput represents the output
type ListTargetGroupsOutput struct {
	TargetGroups []TargetGroupInfo `json:"target_groups"`
	Count        int               `json:"count"`
	TotalCount   int               `json:"total_count"`
	Offset       int               `json:"offset"`
	Limit        int               `json:"limit"`
	HasMore      bool              `json:"has_more"`
}

// Execute runs the tool
func (t *ListTargetGroupsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListTargetGroupsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	// Set defaults
	if params.Limit == 0 {
		params.Limit = 100
	}
	if len(params.Fields) == 0 {
		params.Fields = []string{"name", "target_type", "protocol", "port"}
	}

	// Create field set for quick lookup
	fieldSet := make(map[string]bool)
	for _, field := range params.Fields {
		fieldSet[strings.ToLower(field)] = true
	}
	fieldSet["name"] = true // Always include name

	// Build input for describe target groups
	describeInput := &elasticloadbalancingv2.DescribeTargetGroupsInput{}
	if params.LoadBalancerARN != "" {
		describeInput.LoadBalancerArn = aws.String(params.LoadBalancerARN)
	}

	// List all target groups
	var allTGs []types.TargetGroup
	paginator := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(t.elbClient, describeInput)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list target groups: %w", err)
		}

		allTGs = append(allTGs, output.TargetGroups...)
	}

	// Sort by name
	sort.Slice(allTGs, func(i, j int) bool {
		if allTGs[i].TargetGroupName != nil && allTGs[j].TargetGroupName != nil {
			return *allTGs[i].TargetGroupName < *allTGs[j].TargetGroupName
		}
		return false
	})

	totalCount := len(allTGs)

	// Apply pagination
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= totalCount {
		return ListTargetGroupsOutput{
			TargetGroups: []TargetGroupInfo{},
			Count:        0,
			TotalCount:   totalCount,
			Offset:       params.Offset,
			Limit:        params.Limit,
			HasMore:      false,
		}, nil
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedTGs := allTGs[start:end]

	// Convert to output format
	var targetGroups []TargetGroupInfo
	for _, tg := range paginatedTGs {
		if tg.TargetGroupName == nil {
			continue
		}

		tgInfo := TargetGroupInfo{
			Name: *tg.TargetGroupName,
		}

		// Add fields based on request
		if fieldSet["arn"] && tg.TargetGroupArn != nil {
			tgInfo.ARN = tg.TargetGroupArn
		}

		if fieldSet["target_type"] {
			targetType := string(tg.TargetType)
			tgInfo.TargetType = &targetType
		}

		if fieldSet["protocol"] {
			protocol := string(tg.Protocol)
			tgInfo.Protocol = &protocol
		}

		if fieldSet["port"] && tg.Port != nil {
			tgInfo.Port = tg.Port
		}

		if fieldSet["vpc_id"] && tg.VpcId != nil {
			tgInfo.VpcID = tg.VpcId
		}

		if fieldSet["health_check_protocol"] {
			protocol := string(tg.HealthCheckProtocol)
			tgInfo.HealthCheckProtocol = &protocol
		}

		if fieldSet["health_check_port"] && tg.HealthCheckPort != nil {
			tgInfo.HealthCheckPort = tg.HealthCheckPort
		}

		if fieldSet["health_check_path"] && tg.HealthCheckPath != nil {
			tgInfo.HealthCheckPath = tg.HealthCheckPath
		}

		if fieldSet["health_check_enabled"] && tg.HealthCheckEnabled != nil {
			tgInfo.HealthCheckEnabled = tg.HealthCheckEnabled
		}

		if fieldSet["health_check_interval"] && tg.HealthCheckIntervalSeconds != nil {
			tgInfo.HealthCheckInterval = tg.HealthCheckIntervalSeconds
		}

		if fieldSet["load_balancer_arns"] && len(tg.LoadBalancerArns) > 0 {
			tgInfo.LoadBalancerARNs = tg.LoadBalancerArns
		}

		// Get target health if requested
		if fieldSet["target_health"] && tg.TargetGroupArn != nil {
			health, err := t.getTargetHealth(ctx, *tg.TargetGroupArn)
			if err == nil {
				tgInfo.TargetHealth = &health
			}
		}

		targetGroups = append(targetGroups, tgInfo)
	}

	return ListTargetGroupsOutput{
		TargetGroups: targetGroups,
		Count:        len(targetGroups),
		TotalCount:   totalCount,
		Offset:       params.Offset,
		Limit:        params.Limit,
		HasMore:      end < totalCount,
	}, nil
}

// getTargetHealth retrieves health status of targets in a target group
func (t *ListTargetGroupsTool) getTargetHealth(ctx context.Context, targetGroupARN string) (TargetHealth, error) {
	output, err := t.elbClient.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(targetGroupARN),
	})
	if err != nil {
		return TargetHealth{}, err
	}

	health := TargetHealth{
		TotalCount: len(output.TargetHealthDescriptions),
	}

	for _, desc := range output.TargetHealthDescriptions {
		if desc.TargetHealth != nil {
			if desc.TargetHealth.State == types.TargetHealthStateEnumHealthy {
				health.HealthyCount++
			} else {
				health.UnhealthyCount++
			}
		}
	}

	return health, nil
}
