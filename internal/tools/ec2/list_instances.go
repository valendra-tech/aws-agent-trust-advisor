package ec2tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ListInstancesTool lists EC2 instances with optional filters
type ListInstancesTool struct {
	ec2Client *ec2.Client
}

// NewListInstancesTool creates a new instance
func NewListInstancesTool(awsConfig aws.Config) *ListInstancesTool {
	return &ListInstancesTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns the tool name
func (t *ListInstancesTool) Name() string {
	return "list_ec2_instances"
}

// Description describes the tool
func (t *ListInstancesTool) Description() string {
	return "Lists EC2 instances with optional filters for state and tags. Returns identifiers, state, type, AZ, IPs, VPC/subnet and basic security group info."
}

// InputSchema for the tool
func (t *ListInstancesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"states": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"pending", "running", "stopping", "stopped", "terminated", "shutting-down"},
				},
				"description": "Filter instances by state. If not provided, all states are returned.",
			},
			"tag_key": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag key to filter instances.",
			},
			"tag_value": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag value to filter instances (used with tag_key).",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of instances to return. Default: 50",
				"minimum":     1,
				"maximum":     500,
				"default":     50,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of instances to skip for pagination. Default: 0",
				"minimum":     0,
				"default":     0,
			},
			"include_tags": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, includes the instance tags in the response.",
				"default":     false,
			},
		},
		"required": []string{},
	}
}

// ListInstancesInput holds parameters
type ListInstancesInput struct {
	States      []string `json:"states,omitempty"`
	TagKey      string   `json:"tag_key,omitempty"`
	TagValue    string   `json:"tag_value,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	IncludeTags bool     `json:"include_tags,omitempty"`
}

// InstanceInfo describes an instance
type InstanceInfo struct {
	InstanceID       string             `json:"instance_id"`
	Name             *string            `json:"name,omitempty"`
	State            *string            `json:"state,omitempty"`
	InstanceType     *string            `json:"instance_type,omitempty"`
	AvailabilityZone *string            `json:"availability_zone,omitempty"`
	PublicIP         *string            `json:"public_ip,omitempty"`
	PrivateIP        *string            `json:"private_ip,omitempty"`
	VpcID            *string            `json:"vpc_id,omitempty"`
	SubnetID         *string            `json:"subnet_id,omitempty"`
	SecurityGroups   []SecurityGroupRef `json:"security_groups,omitempty"`
	LaunchTime       *string            `json:"launch_time,omitempty"`
	Tags             map[string]string  `json:"tags,omitempty"`
}

// ListInstancesOutput holds the results
type ListInstancesOutput struct {
	Instances  []InstanceInfo `json:"instances"`
	Count      int            `json:"count"`
	TotalCount int            `json:"total_count"`
	Offset     int            `json:"offset"`
	Limit      int            `json:"limit"`
	HasMore    bool           `json:"has_more"`
}

// Execute lists instances
func (t *ListInstancesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListInstancesInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	if params.Limit == 0 {
		params.Limit = 50
	}

	var filters []types.Filter
	if len(params.States) > 0 {
		values := make([]string, 0, len(params.States))
		for _, st := range params.States {
			values = append(values, strings.ToLower(st))
		}
		filters = append(filters, types.Filter{
			Name:   aws.String("instance-state-name"),
			Values: values,
		})
	}
	if params.TagKey != "" {
		tagFilter := types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", params.TagKey)),
			Values: []string{"*"},
		}
		if params.TagValue != "" {
			tagFilter.Values = []string{params.TagValue}
		}
		filters = append(filters, tagFilter)
	}

	paginator := ec2.NewDescribeInstancesPaginator(t.ec2Client, &ec2.DescribeInstancesInput{
		Filters: filters,
	})

	var instances []types.Instance
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}
		for _, res := range page.Reservations {
			instances = append(instances, res.Instances...)
		}
	}

	sort.Slice(instances, func(i, j int) bool {
		return aws.ToString(instances[i].InstanceId) < aws.ToString(instances[j].InstanceId)
	})

	total := len(instances)
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= total {
		return ListInstancesOutput{
			Instances:  []InstanceInfo{},
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

	selected := instances[start:end]

	out := make([]InstanceInfo, 0, len(selected))
	for _, inst := range selected {
		if inst.InstanceId == nil {
			continue
		}

		info := InstanceInfo{
			InstanceID: *inst.InstanceId,
		}

		for _, tag := range inst.Tags {
			if aws.ToString(tag.Key) == "Name" {
				info.Name = tag.Value
				break
			}
		}

		if inst.State != nil && inst.State.Name != "" {
			state := string(inst.State.Name)
			info.State = &state
		}
		if inst.InstanceType != "" {
			tp := string(inst.InstanceType)
			info.InstanceType = &tp
		}
		if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
			info.AvailabilityZone = inst.Placement.AvailabilityZone
		}
		if inst.PublicIpAddress != nil {
			info.PublicIP = inst.PublicIpAddress
		}
		if inst.PrivateIpAddress != nil {
			info.PrivateIP = inst.PrivateIpAddress
		}
		if inst.VpcId != nil {
			info.VpcID = inst.VpcId
		}
		if inst.SubnetId != nil {
			info.SubnetID = inst.SubnetId
		}
		if len(inst.SecurityGroups) > 0 {
			for _, sg := range inst.SecurityGroups {
				if sg.GroupId == nil {
					continue
				}
				ref := SecurityGroupRef{
					GroupID:   *sg.GroupId,
					GroupName: sg.GroupName,
				}
				info.SecurityGroups = append(info.SecurityGroups, ref)
			}
		}
		if inst.LaunchTime != nil {
			lt := inst.LaunchTime.In(time.UTC).Format(time.RFC3339)
			info.LaunchTime = &lt
		}
		if params.IncludeTags && len(inst.Tags) > 0 {
			info.Tags = make(map[string]string, len(inst.Tags))
			for _, tag := range inst.Tags {
				if tag.Key != nil && tag.Value != nil {
					info.Tags[*tag.Key] = *tag.Value
				}
			}
		}

		out = append(out, info)
	}

	return ListInstancesOutput{
		Instances:  out,
		Count:      len(out),
		TotalCount: total,
		Offset:     params.Offset,
		Limit:      params.Limit,
		HasMore:    end < total,
	}, nil
}
