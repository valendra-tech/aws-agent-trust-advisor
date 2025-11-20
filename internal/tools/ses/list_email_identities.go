package sestools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// ListEmailIdentitiesTool lists SES identities (domains/emails) with verification status
type ListEmailIdentitiesTool struct {
	client *sesv2.Client
}

// NewListEmailIdentitiesTool creates a new instance
func NewListEmailIdentitiesTool(cfg aws.Config) *ListEmailIdentitiesTool {
	return &ListEmailIdentitiesTool{client: sesv2.NewFromConfig(cfg)}
}

func (t *ListEmailIdentitiesTool) Name() string {
	return "list_ses_email_identities"
}

func (t *ListEmailIdentitiesTool) Description() string {
	return "Lists SES email identities (domains/emails) with verification and sending status. Supports filtering by verification status and identity type."
}

func (t *ListEmailIdentitiesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"verification_status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"NOT_STARTED", "PENDING", "TEMPORARY_FAILURE", "SUCCESS", "FAILED"},
				"description": "Optional filter by verification status.",
			},
			"identity_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"EMAIL_ADDRESS", "DOMAIN"},
				"description": "Optional filter by identity type.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum identities to return. Default 100.",
				"minimum":     1,
				"maximum":     1000,
				"default":     100,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Number of identities to skip (pagination). Default 0.",
				"minimum":     0,
				"default":     0,
			},
		},
	}
}

type ListEmailIdentitiesInput struct {
	VerificationStatus string `json:"verification_status,omitempty"`
	IdentityType       string `json:"identity_type,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Offset             int    `json:"offset,omitempty"`
}

type EmailIdentityInfo struct {
	IdentityName       string `json:"identity_name"`
	IdentityType       string `json:"identity_type"`
	VerificationStatus string `json:"verification_status"`
	SendingEnabled     bool   `json:"sending_enabled"`
}

type ListEmailIdentitiesOutput struct {
	Identities []EmailIdentityInfo `json:"identities"`
	Count      int                 `json:"count"`
	TotalCount int                 `json:"total_count"`
	Offset     int                 `json:"offset"`
	Limit      int                 `json:"limit"`
	HasMore    bool                `json:"has_more"`
}

func (t *ListEmailIdentitiesTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ListEmailIdentitiesInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}
	if params.Limit == 0 {
		params.Limit = 100
	}

	paginator := sesv2.NewListEmailIdentitiesPaginator(t.client, &sesv2.ListEmailIdentitiesInput{})

	var all []types.IdentityInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list email identities: %w", err)
		}
		all = append(all, page.EmailIdentities...)
	}

	// Filter
	filtered := make([]types.IdentityInfo, 0, len(all))
	for _, id := range all {
		if params.IdentityType != "" && string(id.IdentityType) != params.IdentityType {
			continue
		}
		if params.VerificationStatus != "" && string(id.VerificationStatus) != params.VerificationStatus {
			continue
		}
		filtered = append(filtered, id)
	}

	// Sort by name
	sort.Slice(filtered, func(i, j int) bool {
		return strings.Compare(aws.ToString(filtered[i].IdentityName), aws.ToString(filtered[j].IdentityName)) < 0
	})

	total := len(filtered)
	start := params.Offset
	end := params.Offset + params.Limit
	if start >= total {
		return ListEmailIdentitiesOutput{
			Identities: []EmailIdentityInfo{},
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

	selected := filtered[start:end]

	resp := make([]EmailIdentityInfo, 0, len(selected))
	for _, id := range selected {
		info := EmailIdentityInfo{
			IdentityName:       aws.ToString(id.IdentityName),
			IdentityType:       string(id.IdentityType),
			VerificationStatus: string(id.VerificationStatus),
			SendingEnabled:     id.SendingEnabled,
		}
		resp = append(resp, info)
	}

	return ListEmailIdentitiesOutput{
		Identities: resp,
		Count:      len(resp),
		TotalCount: total,
		Offset:     params.Offset,
		Limit:      params.Limit,
		HasMore:    end < total,
	}, nil
}
