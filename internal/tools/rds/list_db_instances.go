package rdstools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type ListDBInstancesTool struct {
	client *rds.Client
}

func NewListDBInstancesTool(cfg aws.Config) *ListDBInstancesTool {
	return &ListDBInstancesTool{client: rds.NewFromConfig(cfg)}
}

func (t *ListDBInstancesTool) Name() string {
	return "list_rds_instances"
}

func (t *ListDBInstancesTool) Description() string {
	return "List RDS DB instances with optional field selection and pagination"
}

func (t *ListDBInstancesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"fields": map[string]interface{}{
				"type":        "array",
				"description": "Fields: identifier, engine, version, status, class, storage, multi_az, endpoint, port, vpc_id, az, created_time, security_groups",
				"items":       map[string]interface{}{"type": "string"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max items to return (default 100)",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Items to skip (default 0)",
			},
			"include_tags": map[string]interface{}{
				"type":        "boolean",
				"description": "Include tags for each DB instance (extra API calls).",
				"default":     false,
			},
		},
	}
}

type listDBInstancesInput struct {
	Fields      []string `json:"fields"`
	Limit       int      `json:"limit"`
	Offset      int      `json:"offset"`
	IncludeTags bool     `json:"include_tags,omitempty"`
}

type dbInstanceInfo struct {
	Identifier       string            `json:"identifier"`
	Engine           string            `json:"engine,omitempty"`
	EngineVersion    string            `json:"version,omitempty"`
	Status           string            `json:"status,omitempty"`
	InstanceClass    string            `json:"class,omitempty"`
	Storage          int32             `json:"storage_gb,omitempty"`
	MultiAZ          bool              `json:"multi_az,omitempty"`
	Endpoint         string            `json:"endpoint,omitempty"`
	Port             int32             `json:"port,omitempty"`
	VpcID            string            `json:"vpc_id,omitempty"`
	AvailabilityZone string            `json:"az,omitempty"`
	CreatedTime      string            `json:"created_time,omitempty"`
	SecurityGroups   []string          `json:"security_groups,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
}

func (t *ListDBInstancesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params listDBInstancesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Limit == 0 {
		params.Limit = 100
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	fieldSet := make(map[string]bool)
	for _, f := range params.Fields {
		fieldSet[f] = true
	}
	includeAll := len(fieldSet) == 0

	resp, err := t.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe DB instances failed: %w", err)
	}

	instances := resp.DBInstances
	sort.Slice(instances, func(i, j int) bool {
		return aws.ToString(instances[i].DBInstanceIdentifier) < aws.ToString(instances[j].DBInstanceIdentifier)
	})

	total := len(instances)
	start := params.Offset
	if start > total {
		start = total
	}
	end := start + params.Limit
	if end > total {
		end = total
	}

	infos := make([]dbInstanceInfo, 0, end-start)
	for _, db := range instances[start:end] {
		info := dbInstanceInfo{Identifier: aws.ToString(db.DBInstanceIdentifier)}
		if includeAll || fieldSet["engine"] {
			info.Engine = aws.ToString(db.Engine)
		}
		if includeAll || fieldSet["version"] {
			info.EngineVersion = aws.ToString(db.EngineVersion)
		}
		if includeAll || fieldSet["status"] {
			info.Status = aws.ToString(db.DBInstanceStatus)
		}
		if includeAll || fieldSet["class"] {
			info.InstanceClass = aws.ToString(db.DBInstanceClass)
		}
		if includeAll || fieldSet["storage"] {
			info.Storage = aws.ToInt32(db.AllocatedStorage)
		}
		if includeAll || fieldSet["multi_az"] {
			info.MultiAZ = aws.ToBool(db.MultiAZ)
		}
		if includeAll || fieldSet["endpoint"] {
			if db.Endpoint != nil {
				info.Endpoint = aws.ToString(db.Endpoint.Address)
			}
		}
		if includeAll || fieldSet["port"] {
			if db.Endpoint != nil {
				info.Port = aws.ToInt32(db.Endpoint.Port)
			}
		}
		if includeAll || fieldSet["vpc_id"] {
			if db.DBSubnetGroup != nil {
				info.VpcID = aws.ToString(db.DBSubnetGroup.VpcId)
			}
		}
		if includeAll || fieldSet["az"] {
			info.AvailabilityZone = aws.ToString(db.AvailabilityZone)
		}
		if includeAll || fieldSet["created_time"] {
			if db.InstanceCreateTime != nil {
				info.CreatedTime = db.InstanceCreateTime.Format("2006-01-02 15:04:05 MST")
			}
		}
		if includeAll || fieldSet["security_groups"] {
			for _, sg := range db.VpcSecurityGroups {
				if sg.VpcSecurityGroupId != nil {
					info.SecurityGroups = append(info.SecurityGroups, aws.ToString(sg.VpcSecurityGroupId))
				}
			}
		}
		if params.IncludeTags {
			tags, err := t.client.ListTagsForResource(ctx, &rds.ListTagsForResourceInput{
				ResourceName: db.DBInstanceArn,
			})
			if err == nil && len(tags.TagList) > 0 {
				info.Tags = make(map[string]string, len(tags.TagList))
				for _, tag := range tags.TagList {
					if tag.Key != nil && tag.Value != nil {
						info.Tags[*tag.Key] = *tag.Value
					}
				}
			}
		}
		infos = append(infos, info)
	}

	return map[string]interface{}{
		"instances":   infos,
		"count":       len(infos),
		"total_count": total,
		"offset":      params.Offset,
		"limit":       params.Limit,
		"has_more":    end < total,
	}, nil
}
