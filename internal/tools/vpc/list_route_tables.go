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

// ListRouteTablesTool lists route tables with routes and associations
type ListRouteTablesTool struct {
	ec2Client *ec2.Client
}

// NewListRouteTablesTool creates a new instance
func NewListRouteTablesTool(awsConfig aws.Config) *ListRouteTablesTool {
	return &ListRouteTablesTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListRouteTablesTool) Name() string {
	return "list_route_tables"
}

// Description describes the tool
func (t *ListRouteTablesTool) Description() string {
	return "Lists VPC route tables with main flag, associations, and summarized routes."
}

// InputSchema returns JSON schema
func (t *ListRouteTablesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional VPC ID filter.",
			},
		},
	}
}

// RouteTableInfo output
type RouteTableInfo struct {
	RouteTableID string            `json:"route_table_id"`
	VpcID        *string           `json:"vpc_id,omitempty"`
	IsMain       bool              `json:"is_main"`
	Associations []string          `json:"associations,omitempty"`
	Routes       []string          `json:"routes,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// ListRouteTablesOutput response
type ListRouteTablesOutput struct {
	RouteTables []RouteTableInfo `json:"route_tables"`
	Count       int              `json:"count"`
}

// Execute lists route tables
func (t *ListRouteTablesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		VpcID string `json:"vpc_id,omitempty"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	var filters []types.Filter
	if params.VpcID != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("vpc-id"),
			Values: []string{params.VpcID},
		})
	}

	paginator := ec2.NewDescribeRouteTablesPaginator(t.ec2Client, &ec2.DescribeRouteTablesInput{
		Filters: filters,
	})

	var items []RouteTableInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe route tables: %w", err)
		}
		for _, rt := range page.RouteTables {
			if rt.RouteTableId == nil {
				continue
			}
			info := RouteTableInfo{
				RouteTableID: aws.ToString(rt.RouteTableId),
				VpcID:        rt.VpcId,
			}
			for _, assoc := range rt.Associations {
				if assoc.Main != nil && *assoc.Main {
					info.IsMain = true
				}
				if assoc.SubnetId != nil {
					info.Associations = append(info.Associations, fmt.Sprintf("subnet:%s", *assoc.SubnetId))
				} else if assoc.GatewayId != nil {
					info.Associations = append(info.Associations, fmt.Sprintf("igw:%s", *assoc.GatewayId))
				}
			}
			for _, r := range rt.Routes {
				dest := aws.ToString(r.DestinationCidrBlock)
				if dest == "" && r.DestinationIpv6CidrBlock != nil {
					dest = *r.DestinationIpv6CidrBlock
				}
				target := ""
				switch {
				case r.GatewayId != nil:
					target = fmt.Sprintf("igw:%s", *r.GatewayId)
				case r.NatGatewayId != nil:
					target = fmt.Sprintf("nat:%s", *r.NatGatewayId)
				case r.TransitGatewayId != nil:
					target = fmt.Sprintf("tgw:%s", *r.TransitGatewayId)
				case r.VpcPeeringConnectionId != nil:
					target = fmt.Sprintf("pcx:%s", *r.VpcPeeringConnectionId)
				case r.NetworkInterfaceId != nil:
					target = fmt.Sprintf("eni:%s", *r.NetworkInterfaceId)
				default:
					target = string(r.Origin)
				}
				state := string(r.State)
				info.Routes = append(info.Routes, fmt.Sprintf("%s -> %s (%s)", dest, target, state))
			}
			if len(rt.Tags) > 0 {
				info.Tags = make(map[string]string, len(rt.Tags))
				for _, tag := range rt.Tags {
					if tag.Key != nil && tag.Value != nil {
						info.Tags[*tag.Key] = *tag.Value
					}
				}
			}
			items = append(items, info)
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].RouteTableID < items[j].RouteTableID })

	return ListRouteTablesOutput{
		RouteTables: items,
		Count:       len(items),
	}, nil
}
