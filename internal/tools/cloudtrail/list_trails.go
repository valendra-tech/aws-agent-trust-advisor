package cloudtrailtools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
)

// ListTrailsTool lists CloudTrail trails with status
type ListTrailsTool struct {
	client *cloudtrail.Client
}

// NewListTrailsTool creates a new instance
func NewListTrailsTool(awsConfig aws.Config) *ListTrailsTool {
	return &ListTrailsTool{
		client: cloudtrail.NewFromConfig(awsConfig),
	}
}

// Name returns tool name
func (t *ListTrailsTool) Name() string {
	return "list_cloudtrail_trails"
}

// Description describes the tool
func (t *ListTrailsTool) Description() string {
	return "Lists CloudTrail trails with status (logging, latest delivery time), S3 bucket, and multi-region flag."
}

// InputSchema returns JSON schema
func (t *ListTrailsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// TrailInfo output
type TrailInfo struct {
	Name                string  `json:"name"`
	TrailARN            string  `json:"trail_arn"`
	HomeRegion          string  `json:"home_region"`
	S3Bucket            *string `json:"s3_bucket,omitempty"`
	IncludeGlobal       *bool   `json:"include_global_services,omitempty"`
	MultiRegion         *bool   `json:"multi_region,omitempty"`
	Logging             *bool   `json:"logging,omitempty"`
	LastDelivery        *string `json:"last_delivery_time,omitempty"`
	LatestCloudWatch    *string `json:"latest_cloudwatch_delivery_time,omitempty"`
	HasInsightSelectors bool    `json:"has_insight_selectors"`
}

// ListTrailsOutput response
type ListTrailsOutput struct {
	Trails []TrailInfo `json:"trails"`
	Count  int         `json:"count"`
}

// Execute lists trails
func (t *ListTrailsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	out, err := t.client.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{
		IncludeShadowTrails: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe trails: %w", err)
	}

	var trails []TrailInfo
	for _, tr := range out.TrailList {
		if tr.Name == nil || tr.TrailARN == nil {
			continue
		}

		info := TrailInfo{
			Name:                aws.ToString(tr.Name),
			TrailARN:            aws.ToString(tr.TrailARN),
			HomeRegion:          aws.ToString(tr.HomeRegion),
			S3Bucket:            tr.S3BucketName,
			IncludeGlobal:       tr.IncludeGlobalServiceEvents,
			MultiRegion:         tr.IsMultiRegionTrail,
			HasInsightSelectors: false,
		}

		// Get status
		status, err := t.client.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
			Name: tr.TrailARN,
		})
		if err == nil {
			if status.IsLogging != nil {
				info.Logging = status.IsLogging
			}
			if status.LatestDeliveryTime != nil {
				str := status.LatestDeliveryTime.Format("2006-01-02T15:04:05Z07:00")
				info.LastDelivery = &str
			}
			if status.LatestCloudWatchLogsDeliveryTime != nil {
				str := status.LatestCloudWatchLogsDeliveryTime.Format("2006-01-02T15:04:05Z07:00")
				info.LatestCloudWatch = &str
			}
		}

		// Insight selectors presence
		sel, err := t.client.GetInsightSelectors(ctx, &cloudtrail.GetInsightSelectorsInput{
			TrailName: tr.TrailARN,
		})
		if err == nil && len(sel.InsightSelectors) > 0 {
			info.HasInsightSelectors = true
		}

		trails = append(trails, info)
	}

	return ListTrailsOutput{
		Trails: trails,
		Count:  len(trails),
	}, nil
}
