package sestools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

// ListConfigurationSetsTool lists SES configuration sets
type ListConfigurationSetsTool struct {
	client *sesv2.Client
}

// NewListConfigurationSetsTool creates a new instance
func NewListConfigurationSetsTool(cfg aws.Config) *ListConfigurationSetsTool {
	return &ListConfigurationSetsTool{client: sesv2.NewFromConfig(cfg)}
}

func (t *ListConfigurationSetsTool) Name() string {
	return "list_ses_configuration_sets"
}

func (t *ListConfigurationSetsTool) Description() string {
	return "Lists SES configuration sets with optional pagination."
}

func (t *ListConfigurationSetsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum configuration sets to return. Default 100.",
				"minimum":     1,
				"maximum":     1000,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of items to skip (pagination). Default 0.",
				"minimum":     0,
				"default":     0,
			},
		},
	}
}

type ListConfigurationSetsInput struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type ConfigurationSetInfo struct {
	Name string `json:"name"`
}

type ListConfigurationSetsOutput struct {
	ConfigurationSets []ConfigurationSetInfo `json:"configuration_sets"`
	Count             int                    `json:"count"`
	TotalCount        int                    `json:"total_count"`
	Offset            int                    `json:"offset"`
	Limit             int                    `json:"limit"`
	HasMore           bool                   `json:"has_more"`
}

func (t *ListConfigurationSetsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListConfigurationSetsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	paginator := sesv2.NewListConfigurationSetsPaginator(t.client, &sesv2.ListConfigurationSetsInput{})

	var names []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list configuration sets: %w", err)
		}
		names = append(names, page.ConfigurationSets...)
	}

	sort.Slice(names, func(i, j int) bool { return strings.Compare(names[i], names[j]) < 0 })

	total := len(names)
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= total {
		return ListConfigurationSetsOutput{
			ConfigurationSets: []ConfigurationSetInfo{},
			Count:             0,
			TotalCount:        total,
			Offset:            params.Offset,
			Limit:             params.Limit,
			HasMore:           false,
		}, nil
	}
	if end > total {
		end = total
	}

	sel := names[start:end]
	resp := make([]ConfigurationSetInfo, 0, len(sel))
	for _, n := range sel {
		resp = append(resp, ConfigurationSetInfo{Name: n})
	}

	return ListConfigurationSetsOutput{
		ConfigurationSets: resp,
		Count:             len(resp),
		TotalCount:        total,
		Offset:            params.Offset,
		Limit:             params.Limit,
		HasMore:           end < total,
	}, nil
}
