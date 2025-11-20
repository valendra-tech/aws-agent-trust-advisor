package vpctools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ListNATGatewaysTool lists NAT gateways with state and subnet info
type ListNATGatewaysTool struct {
	ec2Client *ec2.Client
}

// NewListNATGatewaysTool creates a new instance
func NewListNATGatewaysTool(awsConfig aws.Config) *ListNATGatewaysTool {
	return &ListNATGatewaysTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListNATGatewaysTool) Name() string {
	return "list_nat_gateways"
}

// Description describes the tool
func (t *ListNATGatewaysTool) Description() string {
	return "Lists VPC NAT gateways with state, VPC/subnet, connectivity type, and creation time."
}

// InputSchema returns JSON schema
func (t *ListNATGatewaysTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional VPC ID filter.",
			},
			"subnet_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional subnet ID filter.",
			},
		},
	}
}

// NATGatewayInfo output
type NATGatewayInfo struct {
	NatGatewayID     string  `json:"nat_gateway_id"`
	State            string  `json:"state"`
	VpcID            *string `json:"vpc_id,omitempty"`
	SubnetID         *string `json:"subnet_id,omitempty"`
	ConnectivityType *string `json:"connectivity_type,omitempty"`
	ElasticIP        *string `json:"elastic_ip,omitempty"`
	PrivateIP        *string `json:"private_ip,omitempty"`
	CreateTime       *string `json:"create_time,omitempty"`
}

// ListNATGatewaysOutput response
type ListNATGatewaysOutput struct {
	NATGateways []NATGatewayInfo `json:"nat_gateways"`
	Count       int              `json:"count"`
}

// Execute lists NAT gateways
func (t *ListNATGatewaysTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		VpcID    string `json:"vpc_id,omitempty"`
		SubnetID string `json:"subnet_id,omitempty"`
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
	if params.SubnetID != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("subnet-id"),
			Values: []string{params.SubnetID},
		})
	}

	paginator := ec2.NewDescribeNatGatewaysPaginator(t.ec2Client, &ec2.DescribeNatGatewaysInput{
		Filter: filters,
	})

	var items []NATGatewayInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
		}
		for _, ng := range page.NatGateways {
			if ng.NatGatewayId == nil || ng.State == "" {
				continue
			}
			info := NATGatewayInfo{
				NatGatewayID: aws.ToString(ng.NatGatewayId),
				State:        string(ng.State),
				VpcID:        ng.VpcId,
				SubnetID:     ng.SubnetId,
			}
			if ng.ConnectivityType != "" {
				ct := string(ng.ConnectivityType)
				info.ConnectivityType = &ct
			}
			if len(ng.NatGatewayAddresses) > 0 {
				addr := ng.NatGatewayAddresses[0]
				if addr.PublicIp != nil {
					info.ElasticIP = addr.PublicIp
				}
				if addr.PrivateIp != nil {
					info.PrivateIP = addr.PrivateIp
				}
			}
			if ng.CreateTime != nil {
				ct := ng.CreateTime.In(time.UTC).Format(time.RFC3339)
				info.CreateTime = &ct
			}
			items = append(items, info)
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].NatGatewayID < items[j].NatGatewayID })

	return ListNATGatewaysOutput{
		NATGateways: items,
		Count:       len(items),
	}, nil
}
