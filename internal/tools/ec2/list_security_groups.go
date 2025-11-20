package ec2tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ListSecurityGroupsTool lists security groups with optional VPC filter
type ListSecurityGroupsTool struct {
	ec2Client *ec2.Client
}

// NewListSecurityGroupsTool creates a new instance
func NewListSecurityGroupsTool(awsConfig aws.Config) *ListSecurityGroupsTool {
	return &ListSecurityGroupsTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns the tool name
func (t *ListSecurityGroupsTool) Name() string {
	return "list_security_groups"
}

// Description describes the tool
func (t *ListSecurityGroupsTool) Description() string {
	return "Lists security groups with optional VPC filter, showing inbound/outbound rule counts and descriptions."
}

// InputSchema returns the JSON schema
func (t *ListSecurityGroupsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional VPC ID to filter security groups.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of groups to return. Default: 100",
				"minimum":     1,
				"maximum":     500,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of groups to skip for pagination. Default: 0",
				"minimum":     0,
				"default":     0,
			},
			"tag_key": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag key filter.",
			},
			"tag_value": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag value filter (used with tag_key).",
			},
			"include_tags": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, include tags in the response.",
				"default":     false,
			},
		},
		"required": []string{},
	}
}

// ListSecurityGroupsInput parameters
type ListSecurityGroupsInput struct {
	VpcID       string `json:"vpc_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	TagKey      string `json:"tag_key,omitempty"`
	TagValue    string `json:"tag_value,omitempty"`
	IncludeTags bool   `json:"include_tags,omitempty"`
}

// SecurityGroupInfo output
type SecurityGroupInfo struct {
	GroupID          string            `json:"group_id"`
	Name             *string           `json:"name,omitempty"`
	Description      *string           `json:"description,omitempty"`
	VpcID            *string           `json:"vpc_id,omitempty"`
	InboundRules     int               `json:"inbound_rules"`
	OutboundRules    int               `json:"outbound_rules"`
	ReferencedGroups int               `json:"referenced_groups"`
	Tags             map[string]string `json:"tags,omitempty"`
}

// ListSecurityGroupsOutput response
type ListSecurityGroupsOutput struct {
	SecurityGroups []SecurityGroupInfo `json:"security_groups"`
	Count          int                 `json:"count"`
	TotalCount     int                 `json:"total_count"`
	Offset         int                 `json:"offset"`
	Limit          int                 `json:"limit"`
	HasMore        bool                `json:"has_more"`
}

// Execute lists security groups
func (t *ListSecurityGroupsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListSecurityGroupsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	var filters []types.Filter
	if params.VpcID != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{params.VpcID},
		})
	}
	if params.TagKey != "" {
		vals := []string{"*"}
		if params.TagValue != "" {
			vals = []string{params.TagValue}
		}
		filters = append(filters, types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", params.TagKey)),
			Values: vals,
		})
	}

	paginator := ec2.NewDescribeSecurityGroupsPaginator(t.ec2Client, &ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	})

	var groups []types.SecurityGroup
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe security groups: %w", err)
		}
		groups = append(groups, page.SecurityGroups...)
	}

	sort.Slice(groups, func(i, j int) bool {
		return aws.ToString(groups[i].GroupId) < aws.ToString(groups[j].GroupId)
	})

	total := len(groups)
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= total {
		return ListSecurityGroupsOutput{
			SecurityGroups: []SecurityGroupInfo{},
			Count:          0,
			TotalCount:     total,
			Offset:         params.Offset,
			Limit:          params.Limit,
			HasMore:        false,
		}, nil
	}
	if end > total {
		end = total
	}

	selected := groups[start:end]
	resp := make([]SecurityGroupInfo, 0, len(selected))

	for _, sg := range selected {
		if sg.GroupId == nil {
			continue
		}
		info := SecurityGroupInfo{
			GroupID:       *sg.GroupId,
			Name:          sg.GroupName,
			Description:   sg.Description,
			VpcID:         sg.VpcId,
			InboundRules:  len(sg.IpPermissions),
			OutboundRules: len(sg.IpPermissionsEgress),
		}

		referenced := 0
		for _, perm := range sg.IpPermissions {
			referenced += len(perm.UserIdGroupPairs)
		}
		info.ReferencedGroups = referenced

		if params.IncludeTags && len(sg.Tags) > 0 {
			info.Tags = make(map[string]string, len(sg.Tags))
			for _, tag := range sg.Tags {
				if tag.Key != nil && tag.Value != nil {
					info.Tags[*tag.Key] = *tag.Value
				}
			}
		}

		resp = append(resp, info)
	}

	return ListSecurityGroupsOutput{
		SecurityGroups: resp,
		Count:          len(resp),
		TotalCount:     total,
		Offset:         params.Offset,
		Limit:          params.Limit,
		HasMore:        end < total,
	}, nil
}
