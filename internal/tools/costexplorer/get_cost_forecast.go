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

type GetCostForecastTool struct {
	client *costexplorer.Client
}

func NewGetCostForecastTool(cfg aws.Config) *GetCostForecastTool {
	return &GetCostForecastTool{
		client: costexplorer.NewFromConfig(cfg),
	}
}

func (t *GetCostForecastTool) Name() string {
	return "get_cost_forecast"
}

func (t *GetCostForecastTool) Description() string {
	return "Get forecasted AWS costs for future time periods based on historical data. Useful for budget planning."
}

func (t *GetCostForecastTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start_date": map[string]interface{}{
				"type":        "string",
				"description": "Start date for forecast in YYYY-MM-DD format or 'today'. Default: today",
			},
			"end_date": map[string]interface{}{
				"type":        "string",
				"description": "End date for forecast in YYYY-MM-DD format or relative like '+30d', '+3M'. Default: +30d",
			},
			"granularity": map[string]interface{}{
				"type":        "string",
				"description": "Time granularity: DAILY or MONTHLY. Default: MONTHLY",
				"enum":        []string{"DAILY", "MONTHLY"},
			},
			"metric": map[string]interface{}{
				"type":        "string",
				"description": "Metric to forecast. Default: UNBLENDED_COST",
				"enum":        []string{"UNBLENDED_COST", "BLENDED_COST", "AMORTIZED_COST"},
			},
			"prediction_interval": map[string]interface{}{
				"type":        "integer",
				"description": "Confidence interval percentage (80 or 95). Default: 80",
				"enum":        []int{80, 95},
			},
		},
	}
}

type GetCostForecastInput struct {
	StartDate          string `json:"start_date,omitempty"`
	EndDate            string `json:"end_date,omitempty"`
	Granularity        string `json:"granularity,omitempty"`
	Metric             string `json:"metric,omitempty"`
	PredictionInterval int    `json:"prediction_interval,omitempty"`
}

type ForecastDataPoint struct {
	TimePeriod         map[string]string `json:"time_period"`
	MeanValue          string            `json:"mean_value"`
	LowerBound         string            `json:"lower_bound,omitempty"`
	UpperBound         string            `json:"upper_bound,omitempty"`
	PredictionInterval int               `json:"prediction_interval,omitempty"`
}

type GetCostForecastResponse struct {
	ForecastedCost string              `json:"forecasted_cost"`
	Currency       string              `json:"currency"`
	StartDate      string              `json:"start_date"`
	EndDate        string              `json:"end_date"`
	Granularity    string              `json:"granularity"`
	Metric         string              `json:"metric"`
	ForecastData   []ForecastDataPoint `json:"forecast_data"`
}

func (t *GetCostForecastTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params GetCostForecastInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Set defaults
	now := time.Now()
	startDate := now
	if params.StartDate != "" && params.StartDate != "today" {
		var err error
		startDate, err = time.Parse("2006-01-02", params.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid start_date format: %w", err)
		}
	}

	// Default end date: +30 days
	endDate := now.AddDate(0, 0, 30)
	if params.EndDate != "" {
		if len(params.EndDate) > 1 && params.EndDate[0] == '+' {
			var err error
			endDate, err = parseFutureRelativeDate(params.EndDate[1:], now)
			if err != nil {
				return nil, fmt.Errorf("invalid end_date: %w", err)
			}
		} else {
			var err error
			endDate, err = time.Parse("2006-01-02", params.EndDate)
			if err != nil {
				return nil, fmt.Errorf("invalid end_date format: %w", err)
			}
		}
	}

	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Set granularity
	granularity := types.GranularityMonthly
	if params.Granularity == "DAILY" {
		granularity = types.GranularityDaily
	}

	// Set metric
	metric := types.MetricUnblendedCost
	metricStr := "UNBLENDED_COST"
	if params.Metric != "" {
		metricStr = params.Metric
		switch params.Metric {
		case "BLENDED_COST":
			metric = types.MetricBlendedCost
		case "AMORTIZED_COST":
			metric = types.MetricAmortizedCost
		}
	}

	// Set prediction interval
	predictionInterval := 80
	if params.PredictionInterval == 95 {
		predictionInterval = 95
	}

	// Build request
	forecastInput := &costexplorer.GetCostForecastInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startDateStr),
			End:   aws.String(endDateStr),
		},
		Granularity:             granularity,
		Metric:                  metric,
		PredictionIntervalLevel: aws.Int32(int32(predictionInterval)),
	}

	// Execute request
	result, err := t.client.GetCostForecast(ctx, forecastInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost forecast: %w", err)
	}

	// Parse results
	response := GetCostForecastResponse{
		ForecastedCost: aws.ToString(result.Total.Amount),
		Currency:       "USD",
		StartDate:      startDateStr,
		EndDate:        endDateStr,
		Granularity:    params.Granularity,
		Metric:         metricStr,
		ForecastData:   make([]ForecastDataPoint, 0, len(result.ForecastResultsByTime)),
	}

	for _, item := range result.ForecastResultsByTime {
		dataPoint := ForecastDataPoint{
			TimePeriod: map[string]string{
				"start": aws.ToString(item.TimePeriod.Start),
				"end":   aws.ToString(item.TimePeriod.End),
			},
			MeanValue:          aws.ToString(item.MeanValue),
			PredictionInterval: predictionInterval,
		}

		if item.PredictionIntervalLowerBound != nil {
			dataPoint.LowerBound = aws.ToString(item.PredictionIntervalLowerBound)
		}
		if item.PredictionIntervalUpperBound != nil {
			dataPoint.UpperBound = aws.ToString(item.PredictionIntervalUpperBound)
		}

		response.ForecastData = append(response.ForecastData, dataPoint)
	}

	return response, nil
}

func parseFutureRelativeDate(relative string, from time.Time) (time.Time, error) {
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
		return from.AddDate(0, 0, value), nil
	case 'M':
		return from.AddDate(0, value, 0), nil
	case 'y', 'Y':
		return from.AddDate(value, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid relative date unit, use 'd' (days), 'M' (months), or 'y' (years)")
	}
}
