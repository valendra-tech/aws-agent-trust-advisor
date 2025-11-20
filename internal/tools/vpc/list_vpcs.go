package vpctools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ListVPCsTool lists VPCs with CIDR, state, DNS settings, and tag Name
type ListVPCsTool struct {
	ec2Client *ec2.Client
}

// NewListVPCsTool creates a new instance
func NewListVPCsTool(awsConfig aws.Config) *ListVPCsTool {
	return &ListVPCsTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListVPCsTool) Name() string {
	return "list_vpcs"
}

// Description describes the tool
func (t *ListVPCsTool) Description() string {
	return "Lists VPCs with CIDR blocks, state, DNS hostnames/support flags, and Name tag."
}

// InputSchema returns JSON schema
func (t *ListVPCsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
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
				"description": "If true, include all tags for each VPC.",
				"default":     false,
			},
		},
	}
}

// VPCInfo output
type VPCInfo struct {
	VpcID         string            `json:"vpc_id"`
	Name          *string           `json:"name,omitempty"`
	CidrBlock     *string           `json:"cidr_block,omitempty"`
	Ipv6CidrBlock *string           `json:"ipv6_cidr_block,omitempty"`
	State         *string           `json:"state,omitempty"`
	DNSHostnames  *bool             `json:"dns_hostnames,omitempty"`
	DNSSupport    *bool             `json:"dns_support,omitempty"`
	IsDefault     *bool             `json:"is_default,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// ListVPCsOutput response
type ListVPCsOutput struct {
	VPCs  []VPCInfo `json:"vpcs"`
	Count int       `json:"count"`
}

// Execute lists VPCs
func (t *ListVPCsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		TagKey      string `json:"tag_key,omitempty"`
		TagValue    string `json:"tag_value,omitempty"`
		IncludeTags bool   `json:"include_tags,omitempty"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	var filters []types.Filter
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

	out, err := t.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	var vpcs []VPCInfo
	for _, v := range out.Vpcs {
		if v.VpcId == nil {
			continue
		}
		info := VPCInfo{
			VpcID:     aws.ToString(v.VpcId),
			CidrBlock: v.CidrBlock,
			State:     nil,
			IsDefault: v.IsDefault,
		}
		if v.State != "" {
			state := string(v.State)
			info.State = &state
		}
		if len(v.Ipv6CidrBlockAssociationSet) > 0 {
			info.Ipv6CidrBlock = v.Ipv6CidrBlockAssociationSet[0].Ipv6CidrBlock
		}
		for _, tag := range v.Tags {
			if aws.ToString(tag.Key) == "Name" && tag.Value != nil {
				info.Name = tag.Value
			}
			if params.IncludeTags && tag.Key != nil && tag.Value != nil {
				if info.Tags == nil {
					info.Tags = make(map[string]string)
				}
				info.Tags[*tag.Key] = *tag.Value
			}
		}

		// DNS attributes
		dns, err := t.ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
			Attribute: types.VpcAttributeNameEnableDnsHostnames,
			VpcId:     v.VpcId,
		})
		if err == nil && dns.EnableDnsHostnames != nil {
			info.DNSHostnames = dns.EnableDnsHostnames.Value
		}
		dnss, err := t.ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
			Attribute: types.VpcAttributeNameEnableDnsSupport,
			VpcId:     v.VpcId,
		})
		if err == nil && dnss.EnableDnsSupport != nil {
			info.DNSSupport = dnss.EnableDnsSupport.Value
		}

		vpcs = append(vpcs, info)
	}

	sort.Slice(vpcs, func(i, j int) bool { return strings.Compare(vpcs[i].VpcID, vpcs[j].VpcID) < 0 })

	return ListVPCsOutput{
		VPCs:  vpcs,
		Count: len(vpcs),
	}, nil
}
