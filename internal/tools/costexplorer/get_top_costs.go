package costexplorertools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type GetTopCostsTool struct {
	client *costexplorer.Client
}

func NewGetTopCostsTool(cfg aws.Config) *GetTopCostsTool {
	return &GetTopCostsTool{
		client: costexplorer.NewFromConfig(cfg),
	}
}

func (t *GetTopCostsTool) Name() string {
	return "get_top_costs"
}

func (t *GetTopCostsTool) Description() string {
	return "Get top cost contributors by service, region, or usage type. Useful for identifying the most expensive resources."
}

func (t *GetTopCostsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start_date": map[string]interface{}{
				"type":        "string",
				"description": "Start date in YYYY-MM-DD format or relative format like '-7d', '-30d'",
			},
			"end_date": map[string]interface{}{
				"type":        "string",
				"description": "End date in YYYY-MM-DD format or 'today'. Default: today",
			},
			"group_by": map[string]interface{}{
				"type":        "string",
				"description": "Group results by: SERVICE, REGION, USAGE_TYPE, INSTANCE_TYPE. Default: SERVICE",
				"enum":        []string{"SERVICE", "REGION", "USAGE_TYPE", "INSTANCE_TYPE", "LINKED_ACCOUNT"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Number of top items to return. Default: 10",
			},
		},
		"required": []string{"start_date"},
	}
}

type GetTopCostsInput struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date,omitempty"`
	GroupBy   string `json:"group_by,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type TopCostItem struct {
	Name    string  `json:"name"`
	Cost    string  `json:"cost"`
	Percent float64 `json:"percent"`
}

type GetTopCostsResponse struct {
	TopCosts   []TopCostItem `json:"top_costs"`
	TotalCost  string        `json:"total_cost"`
	OthersCost string        `json:"others_cost"`
	Currency   string        `json:"currency"`
	StartDate  string        `json:"start_date"`
	EndDate    string        `json:"end_date"`
	GroupBy    string        `json:"group_by"`
	Count      int           `json:"count"`
	TotalCount int           `json:"total_count"`
}

func (t *GetTopCostsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params GetTopCostsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse dates
	startDate, err := parseDate(params.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}

	endDate := startDate.AddDate(0, 0, 7) // Default: 7 days from start
	if params.EndDate != "" {
		endDate, err = parseDate(params.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date: %w", err)
		}
	}

	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Set group by
	groupBy := "SERVICE"
	if params.GroupBy != "" {
		groupBy = params.GroupBy
	}

	// Set limit
	limit := 10
	if params.Limit > 0 {
		limit = params.Limit
	}

	// Build request
	ceInput := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startDateStr),
			End:   aws.String(endDateStr),
		},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []types.GroupDefinition{
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String(groupBy),
			},
		},
	}

	// Execute request
	result, err := t.client.GetCostAndUsage(ctx, ceInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost and usage: %w", err)
	}

	// Aggregate costs across all time periods
	costMap := make(map[string]float64)
	totalCost := 0.0

	for _, item := range result.ResultsByTime {
		for _, group := range item.Groups {
			name := group.Keys[0]
			if cost, ok := group.Metrics["UnblendedCost"]; ok {
				amount, _ := parseFloat(aws.ToString(cost.Amount))
				costMap[name] += amount
				totalCost += amount
			}
		}
	}

	// Sort by cost
	type costPair struct {
		name string
		cost float64
	}
	pairs := make([]costPair, 0, len(costMap))
	for name, cost := range costMap {
		pairs = append(pairs, costPair{name, cost})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].cost > pairs[j].cost
	})

	// Build response
	response := GetTopCostsResponse{
		TopCosts:   make([]TopCostItem, 0, limit),
		TotalCost:  fmt.Sprintf("%.2f", totalCost),
		Currency:   "USD",
		StartDate:  startDateStr,
		EndDate:    endDateStr,
		GroupBy:    groupBy,
		TotalCount: len(pairs),
	}

	topCostSum := 0.0
	for i, pair := range pairs {
		if i >= limit {
			break
		}
		percent := 0.0
		if totalCost > 0 {
			percent = (pair.cost / totalCost) * 100
		}
		response.TopCosts = append(response.TopCosts, TopCostItem{
			Name:    pair.name,
			Cost:    fmt.Sprintf("%.2f", pair.cost),
			Percent: percent,
		})
		topCostSum += pair.cost
	}

	response.Count = len(response.TopCosts)
	response.OthersCost = fmt.Sprintf("%.2f", totalCost-topCostSum)

	return response, nil
}
