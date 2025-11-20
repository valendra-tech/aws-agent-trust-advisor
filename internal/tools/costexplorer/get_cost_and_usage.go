package costexplorertools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type GetCostAndUsageTool struct {
	client *costexplorer.Client
}

func NewGetCostAndUsageTool(cfg aws.Config) *GetCostAndUsageTool {
	return &GetCostAndUsageTool{
		client: costexplorer.NewFromConfig(cfg),
	}
}

func (t *GetCostAndUsageTool) Name() string {
	return "get_cost_and_usage"
}

func (t *GetCostAndUsageTool) Description() string {
	return "Retrieve AWS cost and usage data for a specific time period. Supports daily, monthly granularity and grouping by service, region, or usage type."
}

func (t *GetCostAndUsageTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start_date": map[string]interface{}{
				"type":        "string",
				"description": "Start date in YYYY-MM-DD format or relative format like '-7d', '-30d', '-1M'",
			},
			"end_date": map[string]interface{}{
				"type":        "string",
				"description": "End date in YYYY-MM-DD format or relative format like 'today', '-1d'. If not provided, defaults to today.",
			},
			"granularity": map[string]interface{}{
				"type":        "string",
				"description": "Time granularity: DAILY or MONTHLY. Default: DAILY",
				"enum":        []string{"DAILY", "MONTHLY"},
			},
			"group_by": map[string]interface{}{
				"type":        "array",
				"description": "Group results by dimension: SERVICE, REGION, USAGE_TYPE, INSTANCE_TYPE, etc.",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"SERVICE", "REGION", "USAGE_TYPE", "INSTANCE_TYPE", "LINKED_ACCOUNT", "OPERATION"},
				},
			},
			"filter_service": map[string]interface{}{
				"type":        "string",
				"description": "Filter by specific AWS service (e.g., 'Amazon Elastic Compute Cloud - Compute', 'Amazon Simple Storage Service')",
			},
			"metrics": map[string]interface{}{
				"type":        "array",
				"description": "Metrics to retrieve. Default: ['UnblendedCost']",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"UnblendedCost", "BlendedCost", "AmortizedCost", "UsageQuantity"},
				},
			},
		},
		"required": []string{"start_date"},
	}
}

type GetCostAndUsageInput struct {
	StartDate     string   `json:"start_date"`
	EndDate       string   `json:"end_date,omitempty"`
	Granularity   string   `json:"granularity,omitempty"`
	GroupBy       []string `json:"group_by,omitempty"`
	FilterService string   `json:"filter_service,omitempty"`
	Metrics       []string `json:"metrics,omitempty"`
}

type CostDataPoint struct {
	TimePeriod map[string]interface{} `json:"time_period"`
	Total      map[string]interface{} `json:"total,omitempty"`
	Groups     []CostGroup            `json:"groups,omitempty"`
}

type CostGroup struct {
	Keys    []string               `json:"keys"`
	Metrics map[string]interface{} `json:"metrics"`
}

type GetCostAndUsageResponse struct {
	ResultsByTime []CostDataPoint `json:"results_by_time"`
	Summary       CostSummary     `json:"summary"`
}

type CostSummary struct {
	TotalCost    string `json:"total_cost"`
	Currency     string `json:"currency"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Granularity  string `json:"granularity"`
	DimensionKey string `json:"dimension_key,omitempty"`
}

func (t *GetCostAndUsageTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params GetCostAndUsageInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse dates
	startDate, err := parseDate(params.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}

	endDate := time.Now()
	if params.EndDate != "" {
		endDate, err = parseDate(params.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date: %w", err)
		}
	}

	// Format dates for API (YYYY-MM-DD)
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Set granularity
	granularity := types.GranularityDaily
	if params.Granularity == "MONTHLY" {
		granularity = types.GranularityMonthly
	}

	// Set metrics
	metrics := params.Metrics
	if len(metrics) == 0 {
		metrics = []string{"UnblendedCost"}
	}

	// Build request
	ceInput := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startDateStr),
			End:   aws.String(endDateStr),
		},
		Granularity: granularity,
		Metrics:     metrics,
	}

	// Add grouping
	if len(params.GroupBy) > 0 {
		ceInput.GroupBy = make([]types.GroupDefinition, len(params.GroupBy))
		for i, dim := range params.GroupBy {
			ceInput.GroupBy[i] = types.GroupDefinition{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String(dim),
			}
		}
	}

	// Add service filter
	if params.FilterService != "" {
		ceInput.Filter = &types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionService,
				Values: []string{params.FilterService},
			},
		}
	}

	// Execute request
	result, err := t.client.GetCostAndUsage(ctx, ceInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost and usage: %w", err)
	}

	// Parse results
	response := GetCostAndUsageResponse{
		ResultsByTime: make([]CostDataPoint, 0, len(result.ResultsByTime)),
		Summary: CostSummary{
			StartDate:   startDateStr,
			EndDate:     endDateStr,
			Granularity: params.Granularity,
			Currency:    "USD",
		},
	}

	totalCost := 0.0

	for _, item := range result.ResultsByTime {
		dataPoint := CostDataPoint{
			TimePeriod: map[string]interface{}{
				"start": aws.ToString(item.TimePeriod.Start),
				"end":   aws.ToString(item.TimePeriod.End),
			},
		}

		if item.Total != nil {
			dataPoint.Total = make(map[string]interface{})
			for metric, value := range item.Total {
				dataPoint.Total[metric] = map[string]interface{}{
					"amount": aws.ToString(value.Amount),
					"unit":   aws.ToString(value.Unit),
				}
				if metric == "UnblendedCost" || metric == "BlendedCost" || metric == "AmortizedCost" {
					if cost, err := parseFloat(aws.ToString(value.Amount)); err == nil {
						totalCost += cost
					}
				}
			}
		}

		if len(item.Groups) > 0 {
			dataPoint.Groups = make([]CostGroup, 0, len(item.Groups))
			for _, group := range item.Groups {
				cg := CostGroup{
					Keys:    group.Keys,
					Metrics: make(map[string]interface{}),
				}
				for metric, value := range group.Metrics {
					cg.Metrics[metric] = map[string]interface{}{
						"amount": aws.ToString(value.Amount),
						"unit":   aws.ToString(value.Unit),
					}
				}
				dataPoint.Groups = append(dataPoint.Groups, cg)
			}
		}

		response.ResultsByTime = append(response.ResultsByTime, dataPoint)
	}

	response.Summary.TotalCost = fmt.Sprintf("%.2f", totalCost)
	if len(params.GroupBy) > 0 {
		response.Summary.DimensionKey = params.GroupBy[0]
	}

	return response, nil
}

func parseDate(dateStr string) (time.Time, error) {
	// Handle relative dates
	if dateStr == "today" {
		return time.Now(), nil
	}

	// Handle negative relative dates like "-7d", "-30d", "-1M"
	if len(dateStr) > 1 && dateStr[0] == '-' {
		return parseRelativeDate(dateStr[1:])
	}

	// Try parsing as YYYY-MM-DD
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format, use YYYY-MM-DD or relative format like '-7d', '-30d'")
	}
	return t, nil
}

func parseRelativeDate(relative string) (time.Time, error) {
	now := time.Now()

	if len(relative) < 2 {
		return time.Time{}, fmt.Errorf("invalid relative date")
	}

	unit := relative[len(relative)-1]
	valueStr := relative[:len(relative)-1]

	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return time.Time{}, fmt.Errorf("invalid relative date value")
	}

	switch unit {
	case 'd', 'D':
		return now.AddDate(0, 0, -value), nil
	case 'M':
		return now.AddDate(0, -value, 0), nil
	case 'y', 'Y':
		return now.AddDate(-value, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid relative date unit, use 'd' (days), 'M' (months), or 'y' (years)")
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
