package organizationstools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
)

// ListAccountsTool lists all accounts in the AWS Organization
type ListAccountsTool struct {
	client *organizations.Client
}

// NewListAccountsTool creates a new ListAccountsTool instance
func NewListAccountsTool(awsConfig aws.Config) *ListAccountsTool {
	return &ListAccountsTool{
		client: organizations.NewFromConfig(awsConfig),
	}
}

// Name returns the name of the tool
func (t *ListAccountsTool) Name() string {
	return "list_organization_accounts"
}

// Description returns a description of what the tool does
func (t *ListAccountsTool) Description() string {
	return "Lists all AWS accounts that belong to the organization, including their ID, ARN, name, email, status, and join method."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *ListAccountsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of accounts to return. Default: 100",
				"minimum":     1,
				"maximum":     1000,
				"default":     100,
			},
		},
	}
}

// ListAccountsInput represents the input parameters
type ListAccountsInput struct {
	Limit int `json:"limit,omitempty"`
}

// AccountInfo represents an AWS account in the organization
type AccountInfo struct {
	ID         string  `json:"id"`
	ARN        string  `json:"arn"`
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Status     string  `json:"status"`
	JoinedMethod string `json:"joined_method"`
	JoinedTimestamp *string `json:"joined_timestamp,omitempty"`
}

// ListAccountsOutput represents the output of the list accounts operation
type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts"`
	Count    int           `json:"count"`
}

// Execute runs the tool and returns a list of organization accounts
func (t *ListAccountsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListAccountsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	var accounts []AccountInfo
	paginator := organizations.NewListAccountsPaginator(t.client, &organizations.ListAccountsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list organization accounts: %w", err)
		}

		for _, acct := range page.Accounts {
			info := AccountInfo{
				ID:           aws.ToString(acct.Id),
				ARN:          aws.ToString(acct.Arn),
				Name:         aws.ToString(acct.Name),
				Email:        aws.ToString(acct.Email),
				Status:       string(acct.Status),
				JoinedMethod: string(acct.JoinedMethod),
			}
			if acct.JoinedTimestamp != nil {
				ts := acct.JoinedTimestamp.Format("2006-01-02T15:04:05Z07:00")
				info.JoinedTimestamp = &ts
			}
			accounts = append(accounts, info)

			if len(accounts) >= params.Limit {
				return ListAccountsOutput{
					Accounts: accounts,
					Count:    len(accounts),
				}, nil
			}
		}
	}

	return ListAccountsOutput{
		Accounts: accounts,
		Count:    len(accounts),
	}, nil
}
