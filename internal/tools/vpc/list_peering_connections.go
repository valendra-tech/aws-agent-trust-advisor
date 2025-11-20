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

// ListPeeringConnectionsTool lists VPC peering connections
type ListPeeringConnectionsTool struct {
	ec2Client *ec2.Client
}

// NewListPeeringConnectionsTool creates a new instance
func NewListPeeringConnectionsTool(awsConfig aws.Config) *ListPeeringConnectionsTool {
	return &ListPeeringConnectionsTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListPeeringConnectionsTool) Name() string {
	return "list_vpc_peering_connections"
}

// Description describes the tool
func (t *ListPeeringConnectionsTool) Description() string {
	return "Lists VPC peering connections with requester/accepter VPCs, status, and creation time."
}

// InputSchema returns JSON schema
func (t *ListPeeringConnectionsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"initiating-request", "pending-acceptance", "active", "deleted", "rejected", "failed", "expired"},
				"description": "Optional status filter.",
			},
		},
	}
}

// PeeringInfo output
type PeeringInfo struct {
	PeeringID     string            `json:"peering_id"`
	Status        string            `json:"status"`
	Requester     *string           `json:"requester_vpc,omitempty"`
	Accepter      *string           `json:"accepter_vpc,omitempty"`
	RequesterCIDR *string           `json:"requester_cidr,omitempty"`
	AccepterCIDR  *string           `json:"accepter_cidr,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// ListPeeringConnectionsOutput response
type ListPeeringConnectionsOutput struct {
	Peerings []PeeringInfo `json:"peerings"`
	Count    int           `json:"count"`
}

// Execute lists peerings
func (t *ListPeeringConnectionsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		Status string `json:"status,omitempty"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	var filters []types.Filter
	if params.Status != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("status-code"),
			Values: []string{params.Status},
		})
	}

	paginator := ec2.NewDescribeVpcPeeringConnectionsPaginator(t.ec2Client, &ec2.DescribeVpcPeeringConnectionsInput{
		Filters: filters,
	})

	var items []PeeringInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe VPC peering connections: %w", err)
		}
		for _, pcx := range page.VpcPeeringConnections {
			if pcx.VpcPeeringConnectionId == nil || pcx.Status == nil {
				continue
			}
			info := PeeringInfo{
				PeeringID: aws.ToString(pcx.VpcPeeringConnectionId),
				Status:    string(pcx.Status.Code),
			}
			if pcx.RequesterVpcInfo != nil {
				info.Requester = pcx.RequesterVpcInfo.VpcId
				info.RequesterCIDR = pcx.RequesterVpcInfo.CidrBlock
			}
			if pcx.AccepterVpcInfo != nil {
				info.Accepter = pcx.AccepterVpcInfo.VpcId
				info.AccepterCIDR = pcx.AccepterVpcInfo.CidrBlock
			}
			if len(pcx.Tags) > 0 {
				info.Tags = make(map[string]string, len(pcx.Tags))
				for _, tag := range pcx.Tags {
					if tag.Key != nil && tag.Value != nil {
						info.Tags[*tag.Key] = *tag.Value
					}
				}
			}
			items = append(items, info)
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].PeeringID < items[j].PeeringID })

	return ListPeeringConnectionsOutput{
		Peerings: items,
		Count:    len(items),
	}, nil
}
