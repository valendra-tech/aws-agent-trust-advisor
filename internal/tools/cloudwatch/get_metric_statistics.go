package cloudwatchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// GetMetricStatisticsTool retrieves metric statistics with flexible configuration
type GetMetricStatisticsTool struct {
	cwClient *cloudwatch.Client
}

// NewGetMetricStatisticsTool creates a new instance
func NewGetMetricStatisticsTool(awsConfig aws.Config) *GetMetricStatisticsTool {
	return &GetMetricStatisticsTool{
		cwClient: cloudwatch.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *GetMetricStatisticsTool) Name() string {
	return "get_cloudwatch_metric_statistics"
}

// Description returns a description of what the tool does
func (t *GetMetricStatisticsTool) Description() string {
	return "Retrieves CloudWatch metric statistics (values) over a time period. Supports various statistics like Average, Sum, Maximum, Minimum, SampleCount. Use this to analyze metric trends and values."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *GetMetricStatisticsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Metric namespace (e.g., 'AWS/EC2', 'AWS/Lambda'). Required.",
			},
			"metric_name": map[string]interface{}{
				"type":        "string",
				"description": "Metric name (e.g., 'CPUUtilization', 'Invocations'). Required.",
			},
			"dimensions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Dimension name (e.g., 'InstanceId', 'FunctionName')",
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "Dimension value (e.g., 'i-1234567890', 'my-function')",
						},
					},
					"required": []string{"name", "value"},
				},
				"description": "Metric dimensions to filter by. Required for most metrics.",
			},
			"statistics": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"Average", "Sum", "Minimum", "Maximum", "SampleCount"},
				},
				"description": "Statistics to retrieve. Default: [\"Average\"]",
				"default":     []string{"Average"},
			},
			"start_time": map[string]interface{}{
				"type":        "string",
				"description": "Start time in ISO 8601 format or relative (e.g., '2025-11-20T10:00:00Z', '-1h', '-24h', '-7d'). Default: -1h",
				"default":     "-1h",
			},
			"end_time": map[string]interface{}{
				"type":        "string",
				"description": "End time in ISO 8601 format or 'now'. Default: now",
				"default":     "now",
			},
			"period": map[string]interface{}{
				"type":        "integer",
				"description": "Period in seconds (60, 300, 3600, etc.). Must be multiple of 60. Default: 300 (5 minutes)",
				"minimum":     60,
				"default":     300,
			},
		},
		"required": []string{"namespace", "metric_name"},
	}
}

// GetMetricStatisticsInput represents the input parameters
type GetMetricStatisticsInput struct {
	Namespace  string           `json:"namespace"`
	MetricName string           `json:"metric_name"`
	Dimensions []DimensionInput `json:"dimensions,omitempty"`
	Statistics []string         `json:"statistics,omitempty"`
	StartTime  string           `json:"start_time,omitempty"`
	EndTime    string           `json:"end_time,omitempty"`
	Period     int32            `json:"period,omitempty"`
}

// DimensionInput represents a dimension filter
type DimensionInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DataPoint represents a metric data point
type DataPoint struct {
	Timestamp   string   `json:"timestamp"`
	Average     *float64 `json:"average,omitempty"`
	Sum         *float64 `json:"sum,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	SampleCount *float64 `json:"sample_count,omitempty"`
	Unit        string   `json:"unit,omitempty"`
}

// GetMetricStatisticsOutput represents the output
type GetMetricStatisticsOutput struct {
	MetricName string      `json:"metric_name"`
	Namespace  string      `json:"namespace"`
	DataPoints []DataPoint `json:"data_points"`
	Count      int         `json:"count"`
	Period     int32       `json:"period"`
	Statistics []string    `json:"statistics"`
}

// Execute runs the tool
func (t *GetMetricStatisticsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params GetMetricStatisticsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if params.MetricName == "" {
		return nil, fmt.Errorf("metric_name is required")
	}

	// Set defaults
	if len(params.Statistics) == 0 {
		params.Statistics = []string{"Average"}
	}
	if params.StartTime == "" {
		params.StartTime = "-1h"
	}
	if params.EndTime == "" {
		params.EndTime = "now"
	}
	if params.Period == 0 {
		params.Period = 300
	}

	// Parse times
	startTime, err := parseTime(params.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time: %w", err)
	}
	endTime, err := parseTime(params.EndTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end_time: %w", err)
	}

	// Convert dimensions
	var dimensions []types.Dimension
	for _, dim := range params.Dimensions {
		dimensions = append(dimensions, types.Dimension{
			Name:  aws.String(dim.Name),
			Value: aws.String(dim.Value),
		})
	}

	// Convert statistics to types
	var stats []types.Statistic
	for _, stat := range params.Statistics {
		stats = append(stats, types.Statistic(stat))
	}

	// Call GetMetricStatistics
	output, err := t.cwClient.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(params.Namespace),
		MetricName: aws.String(params.MetricName),
		Dimensions: dimensions,
		Statistics: stats,
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(params.Period),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metric statistics: %w", err)
	}

	// Convert data points
	var dataPoints []DataPoint
	for _, dp := range output.Datapoints {
		point := DataPoint{
			Timestamp:   dp.Timestamp.Format(time.RFC3339),
			Average:     dp.Average,
			Sum:         dp.Sum,
			Minimum:     dp.Minimum,
			Maximum:     dp.Maximum,
			SampleCount: dp.SampleCount,
		}
		if dp.Unit != "" {
			point.Unit = string(dp.Unit)
		}
		dataPoints = append(dataPoints, point)
	}

	return GetMetricStatisticsOutput{
		MetricName: params.MetricName,
		Namespace:  params.Namespace,
		DataPoints: dataPoints,
		Count:      len(dataPoints),
		Period:     params.Period,
		Statistics: params.Statistics,
	}, nil
}

// parseTime parses time strings (ISO 8601 or relative like -1h, -24h, -7d)
func parseTime(timeStr string) (time.Time, error) {
	if timeStr == "now" {
		return time.Now(), nil
	}

	// Try parsing as ISO 8601
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}

	// Try parsing as relative time
	if len(timeStr) > 1 && timeStr[0] == '-' {
		durationStr := timeStr[1:]
		var duration time.Duration

		// Parse duration
		if len(durationStr) > 1 {
			unit := durationStr[len(durationStr)-1]
			valueStr := durationStr[:len(durationStr)-1]
			var value int
			fmt.Sscanf(valueStr, "%d", &value)

			switch unit {
			case 'h':
				duration = time.Duration(value) * time.Hour
			case 'd':
				duration = time.Duration(value) * 24 * time.Hour
			case 'w':
				duration = time.Duration(value) * 7 * 24 * time.Hour
			case 'm':
				duration = time.Duration(value) * time.Minute
			default:
				return time.Time{}, fmt.Errorf("unknown time unit: %c", unit)
			}
		}

		return time.Now().Add(-duration), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
}
