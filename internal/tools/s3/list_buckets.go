package s3tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ListBucketsTool is a tool that lists all S3 buckets in the AWS account
type ListBucketsTool struct {
	s3Client *s3.Client
}

// NewListBucketsTool creates a new ListBucketsTool instance
func NewListBucketsTool(awsConfig aws.Config) *ListBucketsTool {
	return &ListBucketsTool{
		s3Client: s3.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListBucketsTool) Name() string {
	return "list_s3_buckets"
}

// Description returns a description of what the tool does
func (t *ListBucketsTool) Description() string {
	return "Lists S3 buckets in the AWS account with configurable fields and pagination. You can choose which information to retrieve: names, creation dates, regions, versioning status, encryption, public access settings, and object counts."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListBucketsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"fields": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"name", "creation_date", "region", "versioning", "encryption", "public_access", "object_count"},
				},
				"description": "List of fields to include in the response. Available: name (always included), creation_date, region, versioning, encryption, public_access, object_count. Default: [name, creation_date]",
				"default":     []string{"name", "creation_date"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of buckets to return. Default: 100",
				"minimum":     1,
				"maximum":     1000,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of buckets to skip (for pagination). Default: 0",
				"minimum":     0,
				"default":     0,
			},
		},
		"required": []string{},
	}
}

// ListBucketsInput represents the input parameters for listing buckets
type ListBucketsInput struct {
	Fields []string `json:"fields,omitempty"`
	Limit  int      `json:"limit,omitempty"`
	Offset int      `json:"offset,omitempty"`
}

// BucketInfo represents information about an S3 bucket
type BucketInfo struct {
	Name              string  `json:"name"`
	CreationDate      *string `json:"creation_date,omitempty"`
	Region            *string `json:"region,omitempty"`
	VersioningEnabled *bool   `json:"versioning_enabled,omitempty"`
	EncryptionEnabled *bool   `json:"encryption_enabled,omitempty"`
	PublicAccess      *string `json:"public_access,omitempty"`
	ObjectCount       *int64  `json:"object_count,omitempty"`
}

// ListBucketsOutput represents the output of the list buckets operation
type ListBucketsOutput struct {
	Buckets    []BucketInfo `json:"buckets"`
	Count      int          `json:"count"`
	TotalCount int          `json:"total_count"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
	HasMore    bool         `json:"has_more"`
}

// Execute runs the tool with the given input and returns the result
func (t *ListBucketsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	// Parse input parameters
	var params ListBucketsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	// Set default values
	if params.Limit == 0 {
		params.Limit = 100
	}
	if len(params.Fields) == 0 {
		params.Fields = []string{"name", "creation_date"}
	}

	// Normalize fields to lowercase and create a set for quick lookup
	fieldSet := make(map[string]bool)
	for _, field := range params.Fields {
		fieldSet[strings.ToLower(field)] = true
	}
	fieldSet["name"] = true // Name is always included

	// Call S3 ListBuckets API
	result, err := t.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %w", err)
	}

	totalCount := len(result.Buckets)

	// Apply pagination
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= totalCount {
		return ListBucketsOutput{
			Buckets:    []BucketInfo{},
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

	paginatedBuckets := result.Buckets[start:end]

	// Convert the result to our output format
	buckets := make([]BucketInfo, 0, len(paginatedBuckets))
	for _, bucket := range paginatedBuckets {
		if bucket.Name == nil {
			continue
		}

		bucketInfo := BucketInfo{
			Name: *bucket.Name,
		}

		// Add creation date if requested
		if fieldSet["creation_date"] && bucket.CreationDate != nil {
			dateStr := bucket.CreationDate.Format("2006-01-02 15:04:05 MST")
			bucketInfo.CreationDate = &dateStr
		}

		// Add additional fields if requested
		if fieldSet["region"] {
			region, err := t.getBucketRegion(ctx, *bucket.Name)
			if err == nil {
				bucketInfo.Region = &region
			}
		}

		if fieldSet["versioning"] {
			versioning, err := t.getBucketVersioning(ctx, *bucket.Name)
			if err == nil {
				bucketInfo.VersioningEnabled = &versioning
			}
		}

		if fieldSet["encryption"] {
			encryption, err := t.getBucketEncryption(ctx, *bucket.Name)
			if err == nil {
				bucketInfo.EncryptionEnabled = &encryption
			}
		}

		if fieldSet["public_access"] {
			publicAccess, err := t.getBucketPublicAccess(ctx, *bucket.Name)
			if err == nil {
				bucketInfo.PublicAccess = &publicAccess
			}
		}

		if fieldSet["object_count"] {
			objectCount, err := t.getBucketObjectCount(ctx, *bucket.Name)
			if err == nil {
				bucketInfo.ObjectCount = &objectCount
			}
		}

		buckets = append(buckets, bucketInfo)
	}

	return ListBucketsOutput{
		Buckets:    buckets,
		Count:      len(buckets),
		TotalCount: totalCount,
		Offset:     params.Offset,
		Limit:      params.Limit,
		HasMore:    end < totalCount,
	}, nil
}

// getBucketRegion gets the region of a bucket
func (t *ListBucketsTool) getBucketRegion(ctx context.Context, bucketName string) (string, error) {
	result, err := t.s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "", err
	}

	// Empty location constraint means eu-west-1
	if result.LocationConstraint == "" {
		return "eu-west-1", nil
	}
	return string(result.LocationConstraint), nil
}

// getBucketVersioning checks if versioning is enabled
func (t *ListBucketsTool) getBucketVersioning(ctx context.Context, bucketName string) (bool, error) {
	result, err := t.s3Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return false, err
	}
	return result.Status == types.BucketVersioningStatusEnabled, nil
}

// getBucketEncryption checks if encryption is enabled
func (t *ListBucketsTool) getBucketEncryption(ctx context.Context, bucketName string) (bool, error) {
	_, err := t.s3Client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucketName),
	})
	// If no error, encryption is configured
	if err != nil {
		// Check if the error is because encryption is not set
		if strings.Contains(err.Error(), "ServerSideEncryptionConfigurationNotFoundError") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// getBucketPublicAccess gets the public access block status
func (t *ListBucketsTool) getBucketPublicAccess(ctx context.Context, bucketName string) (string, error) {
	result, err := t.s3Client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchPublicAccessBlockConfiguration") {
			return "not_configured", nil
		}
		return "", err
	}

	config := result.PublicAccessBlockConfiguration
	if config.BlockPublicAcls != nil && *config.BlockPublicAcls &&
		config.BlockPublicPolicy != nil && *config.BlockPublicPolicy &&
		config.IgnorePublicAcls != nil && *config.IgnorePublicAcls &&
		config.RestrictPublicBuckets != nil && *config.RestrictPublicBuckets {
		return "fully_blocked", nil
	} else if config.BlockPublicAcls != nil && *config.BlockPublicAcls {
		return "partially_blocked", nil
	}
	return "public", nil
}

// getBucketObjectCount gets approximate object count using CloudWatch metrics or ListObjectsV2
func (t *ListBucketsTool) getBucketObjectCount(ctx context.Context, bucketName string) (int64, error) {
	// Use ListObjectsV2 with max-keys=1 to get approximate count from metadata
	// Note: This is an approximation and may not be accurate for large buckets
	result, err := t.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		MaxKeys: aws.Int32(1000), // Sample first 1000 to get a sense
	})
	if err != nil {
		return 0, err
	}

	// Return the key count if available, otherwise return counted objects
	if result.KeyCount != nil {
		return int64(*result.KeyCount), nil
	}
	return int64(len(result.Contents)), nil
}
