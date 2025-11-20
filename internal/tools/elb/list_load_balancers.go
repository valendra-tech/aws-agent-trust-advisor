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

// ListLoadBalancersTool lists Application Load Balancers with flexible configuration
type ListLoadBalancersTool struct {
	elbClient *elasticloadbalancingv2.Client
}

// NewListLoadBalancersTool creates a new instance
func NewListLoadBalancersTool(awsConfig aws.Config) *ListLoadBalancersTool {
	return &ListLoadBalancersTool{
		elbClient: elasticloadbalancingv2.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListLoadBalancersTool) Name() string {
	return "list_load_balancers"
}

// Description returns a description of what the tool does
func (t *ListLoadBalancersTool) Description() string {
	return "Lists Application Load Balancers (ALBs) and Network Load Balancers (NLBs) with flexible field selection. Returns details like DNS name, state, availability zones, security groups, and more."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListLoadBalancersTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"load_balancer_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"application", "network", "gateway"},
				"description": "Filter by load balancer type. Options: application (ALB), network (NLB), gateway. If not specified, returns all types.",
			},
			"fields": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"name", "arn", "dns_name", "type", "scheme", "state", "availability_zones", "security_groups", "vpc_id", "created_time"},
				},
				"description": "Fields to include: name (always included), arn, dns_name, type, scheme, state, availability_zones, security_groups, vpc_id, created_time. Default: [name, dns_name, state, type]",
				"default":     []string{"name", "dns_name", "state", "type"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of load balancers to return. Default: 100",
				"minimum":     1,
				"maximum":     400,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of load balancers to skip for pagination. Default: 0",
				"minimum":     0,
				"default":     0,
			},
		},
		"required": []string{},
	}
}

// ListLoadBalancersInput represents the input parameters
type ListLoadBalancersInput struct {
	LoadBalancerType string   `json:"load_balancer_type,omitempty"`
	Fields           []string `json:"fields,omitempty"`
	Limit            int      `json:"limit,omitempty"`
	Offset           int      `json:"offset,omitempty"`
}

// AvailabilityZone represents an AZ
type AvailabilityZone struct {
	ZoneName string `json:"zone_name"`
	SubnetID string `json:"subnet_id"`
}

// LoadBalancerInfo represents information about a load balancer
type LoadBalancerInfo struct {
	Name              string             `json:"name"`
	ARN               *string            `json:"arn,omitempty"`
	DNSName           *string            `json:"dns_name,omitempty"`
	Type              *string            `json:"type,omitempty"`
	Scheme            *string            `json:"scheme,omitempty"`
	State             *string            `json:"state,omitempty"`
	AvailabilityZones []AvailabilityZone `json:"availability_zones,omitempty"`
	SecurityGroups    []string           `json:"security_groups,omitempty"`
	VpcID             *string            `json:"vpc_id,omitempty"`
	CreatedTime       *string            `json:"created_time,omitempty"`
}

// ListLoadBalancersOutput represents the output
type ListLoadBalancersOutput struct {
	LoadBalancers []LoadBalancerInfo `json:"load_balancers"`
	Count         int                `json:"count"`
	TotalCount    int                `json:"total_count"`
	Offset        int                `json:"offset"`
	Limit         int                `json:"limit"`
	HasMore       bool               `json:"has_more"`
}

// Execute runs the tool
func (t *ListLoadBalancersTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListLoadBalancersInput
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
		params.Fields = []string{"name", "dns_name", "state", "type"}
	}

	// Create field set for quick lookup
	fieldSet := make(map[string]bool)
	for _, field := range params.Fields {
		fieldSet[strings.ToLower(field)] = true
	}
	fieldSet["name"] = true // Always include name

	// List all load balancers
	var allLBs []types.LoadBalancer
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(t.elbClient, &elasticloadbalancingv2.DescribeLoadBalancersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list load balancers: %w", err)
		}

		for _, lb := range output.LoadBalancers {
			// Filter by type if specified
			if params.LoadBalancerType != "" && lb.Type != types.LoadBalancerTypeEnum(params.LoadBalancerType) {
				continue
			}
			allLBs = append(allLBs, lb)
		}
	}

	// Sort by name
	sort.Slice(allLBs, func(i, j int) bool {
		if allLBs[i].LoadBalancerName != nil && allLBs[j].LoadBalancerName != nil {
			return *allLBs[i].LoadBalancerName < *allLBs[j].LoadBalancerName
		}
		return false
	})

	totalCount := len(allLBs)

	// Apply pagination
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= totalCount {
		return ListLoadBalancersOutput{
			LoadBalancers: []LoadBalancerInfo{},
			Count:         0,
			TotalCount:    totalCount,
			Offset:        params.Offset,
			Limit:         params.Limit,
			HasMore:       false,
		}, nil
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedLBs := allLBs[start:end]

	// Convert to output format
	var loadBalancers []LoadBalancerInfo
	for _, lb := range paginatedLBs {
		if lb.LoadBalancerName == nil {
			continue
		}

		lbInfo := LoadBalancerInfo{
			Name: *lb.LoadBalancerName,
		}

		// Add fields based on request
		if fieldSet["arn"] && lb.LoadBalancerArn != nil {
			lbInfo.ARN = lb.LoadBalancerArn
		}

		if fieldSet["dns_name"] && lb.DNSName != nil {
			lbInfo.DNSName = lb.DNSName
		}

		if fieldSet["type"] {
			lbType := string(lb.Type)
			lbInfo.Type = &lbType
		}

		if fieldSet["scheme"] {
			scheme := string(lb.Scheme)
			lbInfo.Scheme = &scheme
		}

		if fieldSet["state"] && lb.State != nil {
			state := string(lb.State.Code)
			lbInfo.State = &state
		}

		if fieldSet["availability_zones"] {
			var azs []AvailabilityZone
			for _, az := range lb.AvailabilityZones {
				if az.ZoneName != nil && az.SubnetId != nil {
					azs = append(azs, AvailabilityZone{
						ZoneName: *az.ZoneName,
						SubnetID: *az.SubnetId,
					})
				}
			}
			if len(azs) > 0 {
				lbInfo.AvailabilityZones = azs
			}
		}

		if fieldSet["security_groups"] && len(lb.SecurityGroups) > 0 {
			lbInfo.SecurityGroups = lb.SecurityGroups
		}

		if fieldSet["vpc_id"] && lb.VpcId != nil {
			lbInfo.VpcID = lb.VpcId
		}

		if fieldSet["created_time"] && lb.CreatedTime != nil {
			createdTime := lb.CreatedTime.Format("2006-01-02 15:04:05 MST")
			lbInfo.CreatedTime = &createdTime
		}

		loadBalancers = append(loadBalancers, lbInfo)
	}

	return ListLoadBalancersOutput{
		LoadBalancers: loadBalancers,
		Count:         len(loadBalancers),
		TotalCount:    totalCount,
		Offset:        params.Offset,
		Limit:         params.Limit,
		HasMore:       end < totalCount,
	}, nil
}
