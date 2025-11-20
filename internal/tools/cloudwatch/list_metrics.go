package cloudwatchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// ListMetricsTool lists CloudWatch metrics with flexible filtering
type ListMetricsTool struct {
	cwClient *cloudwatch.Client
}

// NewListMetricsTool creates a new ListMetricsTool instance
func NewListMetricsTool(awsConfig aws.Config) *ListMetricsTool {
	return &ListMetricsTool{
		cwClient: cloudwatch.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListMetricsTool) Name() string {
	return "list_cloudwatch_metrics"
}

// Description returns a description of what the tool does
func (t *ListMetricsTool) Description() string {
	return "Lists CloudWatch metrics with flexible filtering by namespace, metric name, and dimensions. Returns metric details including available dimensions."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListMetricsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter by namespace (e.g., 'AWS/EC2', 'AWS/Lambda'). If not provided, lists metrics from all namespaces.",
			},
			"metric_name": map[string]interface{}{
				"type":        "string",
				"description": "Filter by metric name (e.g., 'CPUUtilization', 'NetworkIn'). Supports partial matching.",
			},
			"dimensions": map[string]interface{}{
				"type":        "object",
				"description": "Filter by specific dimension values (e.g., {\"InstanceId\": \"i-1234567890abcdef0\"})",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
			},
			"fields": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"name", "namespace", "dimensions", "dimension_count"},
				},
				"description": "Fields to include: name (always included), namespace, dimensions, dimension_count. Default: [name, namespace]",
				"default":     []string{"name", "namespace"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of metrics to return. Default: 100",
				"minimum":     1,
				"maximum":     500,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of metrics to skip for pagination. Default: 0",
				"minimum":     0,
				"default":     0,
			},
		},
		"required": []string{},
	}
}

// ListMetricsInput represents the input parameters
type ListMetricsInput struct {
	Namespace  string            `json:"namespace,omitempty"`
	MetricName string            `json:"metric_name,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Fields     []string          `json:"fields,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
}

// MetricDimension represents a metric dimension
type MetricDimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MetricInfo represents information about a CloudWatch metric
type MetricInfo struct {
	Name           string            `json:"name"`
	Namespace      *string           `json:"namespace,omitempty"`
	Dimensions     []MetricDimension `json:"dimensions,omitempty"`
	DimensionCount *int              `json:"dimension_count,omitempty"`
}

// ListMetricsOutput represents the output
type ListMetricsOutput struct {
	Metrics    []MetricInfo `json:"metrics"`
	Count      int          `json:"count"`
	TotalCount int          `json:"total_count"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
	HasMore    bool         `json:"has_more"`
}

// Execute runs the tool
func (t *ListMetricsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListMetricsInput
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
		params.Fields = []string{"name", "namespace"}
	}

	// Create field set for quick lookup
	fieldSet := make(map[string]bool)
	for _, field := range params.Fields {
		fieldSet[strings.ToLower(field)] = true
	}
	fieldSet["name"] = true // Always include name

	// Build ListMetrics input
	listInput := &cloudwatch.ListMetricsInput{}
	if params.Namespace != "" {
		listInput.Namespace = aws.String(params.Namespace)
	}
	if params.MetricName != "" {
		listInput.MetricName = aws.String(params.MetricName)
	}

	// Convert dimension filters
	if len(params.Dimensions) > 0 {
		for name, value := range params.Dimensions {
			listInput.Dimensions = append(listInput.Dimensions, types.DimensionFilter{
				Name:  aws.String(name),
				Value: aws.String(value),
			})
		}
	}

	// Collect all metrics
	var allMetrics []MetricInfo
	paginator := cloudwatch.NewListMetricsPaginator(t.cwClient, listInput)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list metrics: %w", err)
		}

		for _, metric := range output.Metrics {
			if metric.MetricName == nil {
				continue
			}

			metricInfo := MetricInfo{
				Name: *metric.MetricName,
			}

			// Add namespace if requested
			if fieldSet["namespace"] && metric.Namespace != nil {
				metricInfo.Namespace = metric.Namespace
			}

			// Add dimensions if requested
			if fieldSet["dimensions"] || fieldSet["dimension_count"] {
				var dims []MetricDimension
				for _, dim := range metric.Dimensions {
					if dim.Name != nil && dim.Value != nil {
						dims = append(dims, MetricDimension{
							Name:  *dim.Name,
							Value: *dim.Value,
						})
					}
				}

				if fieldSet["dimensions"] {
					metricInfo.Dimensions = dims
				}
				if fieldSet["dimension_count"] {
					count := len(dims)
					metricInfo.DimensionCount = &count
				}
			}

			allMetrics = append(allMetrics, metricInfo)
		}
	}

	totalCount := len(allMetrics)

	// Sort by name
	sort.Slice(allMetrics, func(i, j int) bool {
		return allMetrics[i].Name < allMetrics[j].Name
	})

	// Apply pagination
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= totalCount {
		return ListMetricsOutput{
			Metrics:    []MetricInfo{},
			Count:      0,
			TotalCount: totalCount,
			Offset:     params.Offset,
			Limit:      params.Limit,
			HasMore:    false,
		}, nil
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedMetrics := allMetrics[start:end]

	return ListMetricsOutput{
		Metrics:    paginatedMetrics,
		Count:      len(paginatedMetrics),
		TotalCount: totalCount,
		Offset:     params.Offset,
		Limit:      params.Limit,
		HasMore:    end < totalCount,
	}, nil
}
