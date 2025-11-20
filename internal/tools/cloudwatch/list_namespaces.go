package cloudwatchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

// ListNamespacesTool lists CloudWatch namespaces (metric sources)
type ListNamespacesTool struct {
	cwClient *cloudwatch.Client
}

// NewListNamespacesTool creates a new ListNamespacesTool instance
func NewListNamespacesTool(awsConfig aws.Config) *ListNamespacesTool {
	return &ListNamespacesTool{
		cwClient: cloudwatch.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListNamespacesTool) Name() string {
	return "list_cloudwatch_namespaces"
}

// Description returns a description of what the tool does
func (t *ListNamespacesTool) Description() string {
	return "Lists CloudWatch metric namespaces (sources of metrics like AWS/EC2, AWS/RDS, AWS/Lambda, etc.). Useful for discovering what AWS services are actively publishing metrics."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListNamespacesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "Filter namespaces by prefix or substring (e.g., 'AWS/EC2', 'AWS/', 'Lambda'). Case-insensitive.",
			},
			"include_custom": map[string]interface{}{
				"type":        "boolean",
				"description": "Include custom (non-AWS) namespaces. Default: true",
				"default":     true,
			},
		},
		"required": []string{},
	}
}

// ListNamespacesInput represents the input parameters
type ListNamespacesInput struct {
	Filter        string `json:"filter,omitempty"`
	IncludeCustom *bool  `json:"include_custom,omitempty"`
}

// NamespaceInfo represents information about a CloudWatch namespace
type NamespaceInfo struct {
	Namespace   string `json:"namespace"`
	MetricCount int    `json:"metric_count"`
	IsCustom    bool   `json:"is_custom"`
}

// ListNamespacesOutput represents the output
type ListNamespacesOutput struct {
	Namespaces []NamespaceInfo `json:"namespaces"`
	Count      int             `json:"count"`
}

// Execute runs the tool
func (t *ListNamespacesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListNamespacesInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	// Set default
	includeCustom := true
	if params.IncludeCustom != nil {
		includeCustom = *params.IncludeCustom
	}

	// List all metrics to get unique namespaces
	namespaceCounts := make(map[string]int)
	paginator := cloudwatch.NewListMetricsPaginator(t.cwClient, &cloudwatch.ListMetricsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list metrics: %w", err)
		}

		for _, metric := range output.Metrics {
			if metric.Namespace != nil {
				namespaceCounts[*metric.Namespace]++
			}
		}
	}

	// Convert to slice and filter
	var namespaces []NamespaceInfo
	for ns, count := range namespaceCounts {
		isCustom := !strings.HasPrefix(ns, "AWS/")

		// Apply filters
		if !includeCustom && isCustom {
			continue
		}

		if params.Filter != "" {
			filterLower := strings.ToLower(params.Filter)
			nsLower := strings.ToLower(ns)
			if !strings.Contains(nsLower, filterLower) {
				continue
			}
		}

		namespaces = append(namespaces, NamespaceInfo{
			Namespace:   ns,
			MetricCount: count,
			IsCustom:    isCustom,
		})
	}

	// Sort by namespace name
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Namespace < namespaces[j].Namespace
	})

	return ListNamespacesOutput{
		Namespaces: namespaces,
		Count:      len(namespaces),
	}, nil
}
