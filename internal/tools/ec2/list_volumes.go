package ec2tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ListVolumesTool lists EBS volumes
type ListVolumesTool struct {
	ec2Client *ec2.Client
}

// NewListVolumesTool creates a new instance
func NewListVolumesTool(awsConfig aws.Config) *ListVolumesTool {
	return &ListVolumesTool{
		ec2Client: ec2.NewFromConfig(awsConfig),
	}
}

// Name returns the tool name
func (t *ListVolumesTool) Name() string {
	return "list_ec2_volumes"
}

// Description describes the tool
func (t *ListVolumesTool) Description() string {
	return "Lists EBS volumes with optional filters by state and availability zone. Returns attachment info, size, type, IOPS/throughput, and encryption."
}

// InputSchema returns JSON schema
func (t *ListVolumesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"state": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"creating", "available", "in-use", "deleting", "deleted", "error"},
				"description": "Filter by volume state. If omitted, all states are returned.",
			},
			"availability_zone": map[string]interface{}{
				"type":        "string",
				"description": "Optional availability zone filter (e.g., us-east-1a).",
			},
			"tag_key": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag key filter.",
			},
			"tag_value": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag value filter (used with tag_key).",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of volumes to return. Default: 100",
				"minimum":     1,
				"maximum":     500,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of volumes to skip for pagination. Default: 0",
				"minimum":     0,
				"default":     0,
			},
		},
		"required": []string{},
	}
}

// ListVolumesInput parameters
type ListVolumesInput struct {
	State            string `json:"state,omitempty"`
	AvailabilityZone string `json:"availability_zone,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	Offset           int    `json:"offset,omitempty"`
	TagKey           string `json:"tag_key,omitempty"`
	TagValue         string `json:"tag_value,omitempty"`
}

// VolumeInfo output
type VolumeInfo struct {
	VolumeID         string            `json:"volume_id"`
	State            *string           `json:"state,omitempty"`
	SizeGiB          int32             `json:"size_gib"`
	VolumeType       *string           `json:"volume_type,omitempty"`
	AvailabilityZone *string           `json:"availability_zone,omitempty"`
	IOPS             *int32            `json:"iops,omitempty"`
	Throughput       *int32            `json:"throughput,omitempty"`
	Encrypted        *bool             `json:"encrypted,omitempty"`
	Attachments      []string          `json:"attachments,omitempty"`
	CreateTime       *string           `json:"create_time,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
}

// ListVolumesOutput response
type ListVolumesOutput struct {
	Volumes    []VolumeInfo `json:"volumes"`
	Count      int          `json:"count"`
	TotalCount int          `json:"total_count"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
	HasMore    bool         `json:"has_more"`
}

// Execute lists volumes
func (t *ListVolumesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListVolumesInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	var filters []types.Filter
	if params.State != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("status"),
			Values: []string{strings.ToLower(params.State)},
		})
	}
	if params.AvailabilityZone != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("availability-zone"),
			Values: []string{params.AvailabilityZone},
		})
	}
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

	paginator := ec2.NewDescribeVolumesPaginator(t.ec2Client, &ec2.DescribeVolumesInput{
		Filters: filters,
	})

	var vols []types.Volume
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe volumes: %w", err)
		}
		vols = append(vols, page.Volumes...)
	}

	sort.Slice(vols, func(i, j int) bool {
		return aws.ToString(vols[i].VolumeId) < aws.ToString(vols[j].VolumeId)
	})

	total := len(vols)
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= total {
		return ListVolumesOutput{
			Volumes:    []VolumeInfo{},
			Count:      0,
			TotalCount: total,
			Offset:     params.Offset,
			Limit:      params.Limit,
			HasMore:    false,
		}, nil
	}
	if end > total {
		end = total
	}

	selected := vols[start:end]
	resp := make([]VolumeInfo, 0, len(selected))

	for _, vol := range selected {
		if vol.VolumeId == nil {
			continue
		}
		info := VolumeInfo{
			VolumeID:         *vol.VolumeId,
			SizeGiB:          aws.ToInt32(vol.Size),
			AvailabilityZone: vol.AvailabilityZone,
		}
		if vol.State != "" {
			st := string(vol.State)
			info.State = &st
		}
		if vol.VolumeType != "" {
			vt := string(vol.VolumeType)
			info.VolumeType = &vt
		}
		if vol.Iops != nil {
			info.IOPS = vol.Iops
		}
		if vol.Throughput != nil {
			info.Throughput = vol.Throughput
		}
		if vol.Encrypted != nil {
			info.Encrypted = vol.Encrypted
		}
		for _, att := range vol.Attachments {
			if att.InstanceId != nil {
				attach := fmt.Sprintf("%s:%s", aws.ToString(att.InstanceId), aws.ToString(att.Device))
				info.Attachments = append(info.Attachments, attach)
			}
		}
		if vol.CreateTime != nil {
			ct := vol.CreateTime.In(time.UTC).Format(time.RFC3339)
			info.CreateTime = &ct
		}
		if len(vol.Tags) > 0 {
			info.Tags = make(map[string]string, len(vol.Tags))
			for _, tag := range vol.Tags {
				if tag.Key != nil && tag.Value != nil {
					info.Tags[*tag.Key] = *tag.Value
				}
			}
		}

		resp = append(resp, info)
	}

	return ListVolumesOutput{
		Volumes:    resp,
		Count:      len(resp),
		TotalCount: total,
		Offset:     params.Offset,
		Limit:      params.Limit,
		HasMore:    end < total,
	}, nil
}
