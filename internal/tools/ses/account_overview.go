package sestools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

// AccountOverviewTool retrieves SES account-level sending information
type AccountOverviewTool struct {
	client *sesv2.Client
}

// NewAccountOverviewTool creates a new instance
func NewAccountOverviewTool(cfg aws.Config) *AccountOverviewTool {
	return &AccountOverviewTool{client: sesv2.NewFromConfig(cfg)}
}

func (t *AccountOverviewTool) Name() string {
	return "get_ses_account_overview"
}

func (t *AccountOverviewTool) Description() string {
	return "Shows SES account sending limits, production access, enforcement status, and sending-enabled flag."
}

func (t *AccountOverviewTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

type AccountOverview struct {
	ProductionAccessEnabled bool    `json:"production_access_enabled"`
	EnforcementStatus       *string `json:"enforcement_status,omitempty"`
	Max24HourSend           float64 `json:"max_24_hour_send,omitempty"`
	MaxSendRate             float64 `json:"max_send_rate,omitempty"`
	SentLast24Hours         float64 `json:"sent_last_24_hours,omitempty"`
}

func (t *AccountOverviewTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	out, err := t.client.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get SES account: %w", err)
	}

	ov := AccountOverview{
		ProductionAccessEnabled: out.ProductionAccessEnabled,
		EnforcementStatus:       out.EnforcementStatus,
	}
	if out.SendQuota != nil {
		ov.Max24HourSend = out.SendQuota.Max24HourSend
		ov.MaxSendRate = out.SendQuota.MaxSendRate
		ov.SentLast24Hours = out.SendQuota.SentLast24Hours
	}

	return ov, nil
}
