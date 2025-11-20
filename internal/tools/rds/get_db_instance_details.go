package rdstools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type GetDBInstanceDetailsTool struct {
	client *rds.Client
}

func NewGetDBInstanceDetailsTool(cfg aws.Config) *GetDBInstanceDetailsTool {
	return &GetDBInstanceDetailsTool{client: rds.NewFromConfig(cfg)}
}

func (t *GetDBInstanceDetailsTool) Name() string { return "get_rds_instance_details" }

func (t *GetDBInstanceDetailsTool) Description() string {
	return "Get detailed information for specific RDS DB instance"
}

func (t *GetDBInstanceDetailsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"identifier": map[string]interface{}{
				"type":        "string",
				"description": "DB instance identifier",
			},
		},
		"required": []string{"identifier"},
	}
}

type getDBInstanceDetailsInput struct {
	Identifier string `json:"identifier"`
}

type dbInstanceDetails struct {
	Identifier                 string            `json:"identifier"`
	ARN                        string            `json:"arn"`
	Engine                     string            `json:"engine"`
	EngineVersion              string            `json:"engine_version"`
	Status                     string            `json:"status"`
	Class                      string            `json:"instance_class"`
	Storage                    int32             `json:"storage_gb"`
	StorageType                string            `json:"storage_type"`
	MultiAZ                    bool              `json:"multi_az"`
	VpcID                      string            `json:"vpc_id"`
	AvailabilityZone           string            `json:"az"`
	Endpoint                   string            `json:"endpoint"`
	Port                       int32             `json:"port"`
	MasterUsername             string            `json:"master_username"`
	BackupRetention            int32             `json:"backup_retention_days"`
	PreferredBackupWindow      string            `json:"preferred_backup_window"`
	PreferredMaintenanceWindow string            `json:"preferred_maintenance_window"`
	PubliclyAccessible         bool              `json:"publicly_accessible"`
	StorageEncrypted           bool              `json:"storage_encrypted"`
	CreatedTime                string            `json:"created_time"`
	SecurityGroups             []string          `json:"security_groups,omitempty"`
	Tags                       map[string]string `json:"tags,omitempty"`
}

func (t *GetDBInstanceDetailsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params getDBInstanceDetailsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	resp, err := t.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(params.Identifier),
	})
	if err != nil {
		return nil, fmt.Errorf("describe DB instance failed: %w", err)
	}
	if len(resp.DBInstances) == 0 {
		return nil, fmt.Errorf("db instance not found: %s", params.Identifier)
	}

	db := resp.DBInstances[0]
	details := dbInstanceDetails{
		Identifier:                 aws.ToString(db.DBInstanceIdentifier),
		ARN:                        aws.ToString(db.DBInstanceArn),
		Engine:                     aws.ToString(db.Engine),
		EngineVersion:              aws.ToString(db.EngineVersion),
		Status:                     aws.ToString(db.DBInstanceStatus),
		Class:                      aws.ToString(db.DBInstanceClass),
		Storage:                    aws.ToInt32(db.AllocatedStorage),
		StorageType:                aws.ToString(db.StorageType),
		MultiAZ:                    aws.ToBool(db.MultiAZ),
		MasterUsername:             aws.ToString(db.MasterUsername),
		BackupRetention:            aws.ToInt32(db.BackupRetentionPeriod),
		PreferredBackupWindow:      aws.ToString(db.PreferredBackupWindow),
		PreferredMaintenanceWindow: aws.ToString(db.PreferredMaintenanceWindow),
		PubliclyAccessible:         aws.ToBool(db.PubliclyAccessible),
		StorageEncrypted:           aws.ToBool(db.StorageEncrypted),
	}

	if db.DBSubnetGroup != nil {
		details.VpcID = aws.ToString(db.DBSubnetGroup.VpcId)
	}
	if db.Endpoint != nil {
		details.Endpoint = aws.ToString(db.Endpoint.Address)
		details.Port = aws.ToInt32(db.Endpoint.Port)
	}
	if db.AvailabilityZone != nil {
		details.AvailabilityZone = aws.ToString(db.AvailabilityZone)
	}
	if db.InstanceCreateTime != nil {
		details.CreatedTime = db.InstanceCreateTime.Format("2006-01-02 15:04:05 MST")
	}
	for _, sg := range db.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil {
			details.SecurityGroups = append(details.SecurityGroups, aws.ToString(sg.VpcSecurityGroupId))
		}
	}

	if len(db.TagList) > 0 {
		details.Tags = make(map[string]string)
		for _, tag := range db.TagList {
			details.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
		}
	}

	return details, nil
}
