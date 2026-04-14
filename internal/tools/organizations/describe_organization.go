package organizationstools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
)

// DescribeOrganizationTool returns details about the AWS Organization
type DescribeOrganizationTool struct {
	client *organizations.Client
}

// NewDescribeOrganizationTool creates a new DescribeOrganizationTool instance
func NewDescribeOrganizationTool(awsConfig aws.Config) *DescribeOrganizationTool {
	return &DescribeOrganizationTool{
		client: organizations.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *DescribeOrganizationTool) Name() string {
	return "describe_organization"
}

// Description returns a description of what the tool does
func (t *DescribeOrganizationTool) Description() string {
	return "Returns details about the AWS Organization, including its ID, ARN, master account, feature set, and available policy types."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *DescribeOrganizationTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// OrganizationInfo represents details about an AWS Organization
type OrganizationInfo struct {
	ID                 string        `json:"id"`
	ARN                string        `json:"arn"`
	FeatureSet         string        `json:"feature_set"`
	MasterAccountID    string        `json:"master_account_id"`
	MasterAccountARN   string        `json:"master_account_arn"`
	MasterAccountEmail string        `json:"master_account_email"`
	AvailablePolicies  []PolicyType  `json:"available_policy_types"`
}

// PolicyType represents a policy type available in the organization
type PolicyType struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// Execute runs the tool and returns organization details
func (t *DescribeOrganizationTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	out, err := t.client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe organization: %w", err)
	}

	org := out.Organization
	if org == nil {
		return nil, fmt.Errorf("no organization data returned")
	}

	info := OrganizationInfo{
		ID:                 aws.ToString(org.Id),
		ARN:                aws.ToString(org.Arn),
		FeatureSet:         string(org.FeatureSet),
		MasterAccountID:    aws.ToString(org.MasterAccountId),
		MasterAccountARN:   aws.ToString(org.MasterAccountArn),
		MasterAccountEmail: aws.ToString(org.MasterAccountEmail),
	}

	for _, pt := range org.AvailablePolicyTypes {
		info.AvailablePolicies = append(info.AvailablePolicies, PolicyType{
			Type:   string(pt.Type),
			Status: string(pt.Status),
		})
	}

	return info, nil
}
