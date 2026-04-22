package organizationstools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"
)

// ListPoliciesTool lists policies in the AWS Organization
type ListPoliciesTool struct {
	client *organizations.Client
}

// NewListPoliciesTool creates a new ListPoliciesTool instance
func NewListPoliciesTool(awsConfig aws.Config) *ListPoliciesTool {
	return &ListPoliciesTool{
		client: organizations.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListPoliciesTool) Name() string {
	return "list_organization_policies"
}

// Description returns a description of what the tool does
func (t *ListPoliciesTool) Description() string {
	return "Lists policies in the AWS Organization for a given policy type. Supported types: SERVICE_CONTROL_POLICY, TAG_POLICY, BACKUP_POLICY, AISERVICES_OPT_OUT_POLICY."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListPoliciesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "The type of policies to list. Default: SERVICE_CONTROL_POLICY",
				"enum":        []string{"SERVICE_CONTROL_POLICY", "TAG_POLICY", "BACKUP_POLICY", "AISERVICES_OPT_OUT_POLICY"},
				"default":     "SERVICE_CONTROL_POLICY",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of policies to return. Default: 100",
				"minimum":     1,
				"maximum":     1000,
				"default":     100,
			},
		},
	}
}

// ListPoliciesInput represents the input parameters
type ListPoliciesInput struct {
	Filter string `json:"filter,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// PolicyInfo represents an organization policy
type PolicyInfo struct {
	ID          string `json:"id"`
	ARN         string `json:"arn"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	AwsManaged  bool   `json:"aws_managed"`
}

// ListPoliciesOutput represents the output of the list policies operation
type ListPoliciesOutput struct {
	Policies []PolicyInfo `json:"policies"`
	Count    int          `json:"count"`
	Filter   string       `json:"filter"`
}

// Execute runs the tool and returns a list of organization policies
func (t *ListPoliciesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListPoliciesInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	if params.Filter == "" {
		params.Filter = "SERVICE_CONTROL_POLICY"
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	filter, err := parsePolicyType(params.Filter)
	if err != nil {
		return nil, err
	}

	var policies []PolicyInfo
	paginator := organizations.NewListPoliciesPaginator(t.client, &organizations.ListPoliciesInput{
		Filter: filter,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list organization policies: %w", err)
		}

		for _, p := range page.Policies {
			info := PolicyInfo{
				ID:          aws.ToString(p.Id),
				ARN:         aws.ToString(p.Arn),
				Name:        aws.ToString(p.Name),
				Description: aws.ToString(p.Description),
				Type:        string(p.Type),
				AwsManaged:  p.AwsManaged,
			}
			policies = append(policies, info)

			if len(policies) >= params.Limit {
				return ListPoliciesOutput{
					Policies: policies,
					Count:    len(policies),
					Filter:   params.Filter,
				}, nil
			}
		}
	}

	return ListPoliciesOutput{
		Policies: policies,
		Count:    len(policies),
		Filter:   params.Filter,
	}, nil
}

func parsePolicyType(s string) (types.PolicyType, error) {
	switch s {
	case "SERVICE_CONTROL_POLICY":
		return types.PolicyTypeServiceControlPolicy, nil
	case "TAG_POLICY":
		return types.PolicyTypeTagPolicy, nil
	case "BACKUP_POLICY":
		return types.PolicyTypeBackupPolicy, nil
	case "AISERVICES_OPT_OUT_POLICY":
		return types.PolicyTypeAiservicesOptOutPolicy, nil
	default:
		return "", fmt.Errorf("unsupported policy type: %s", s)
	}
}
