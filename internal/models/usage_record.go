package models

import (
	"time"
)

// UsageRecord represents a single model-invocation usage entry. A record is
// written only when a model-invocation route is hit AND either token usage was
// reported by the upstream or the call failed (status >= 400).
//
// Each record is a snapshot of one token→budget conversion:
//
//	cost = (input_tokens * input_price + output_tokens * output_price) / 100
//
// where prices come from the model_pricings table (cents per token). That cost
// is accumulated onto the owning virtual key's budget_used, establishing the
// concrete relationship between a virtual key's budget and consumed tokens.
type UsageRecord struct {
	ID                          uint      `gorm:"primaryKey" json:"id"`
	TenantID                    uint      `gorm:"index;not null;default:1" json:"tenant_id"`
	RequestID                   string    `gorm:"size:64;index" json:"request_id"`
	VirtualKeyID                *uint     `gorm:"index" json:"virtual_key_id,omitempty"`
	VirtualKeyName              string    `gorm:"size:100" json:"virtual_key_name,omitempty"`
	VirtualKeyCreatedByUserID   *uint     `gorm:"index" json:"virtual_key_created_by_user_id,omitempty"`
	VirtualKeyCreatedByUsername string    `gorm:"size:100" json:"virtual_key_created_by_username,omitempty"`
	Provider                    string    `gorm:"size:50;index" json:"provider"`
	Model                       string    `gorm:"size:100;index" json:"model"`
	Endpoint                    string    `gorm:"size:100;index" json:"endpoint"`
	StatusCode                  int       `gorm:"index" json:"status_code"`
	ErrorMessage                string    `gorm:"type:text" json:"error_message,omitempty"`
	ModelInputKind              string    `gorm:"size:50" json:"model_input_kind,omitempty"`
	ModelInputPreview           string    `gorm:"type:text" json:"model_input_preview,omitempty"`
	ModelInputTruncated         bool      `json:"model_input_truncated"`
	ModelInputBytes             int       `json:"model_input_bytes"`
	ModelInputPreviewBytes      int       `json:"model_input_preview_bytes"`
	InputTokens                 int       `json:"input_tokens"`
	OutputTokens                int       `json:"output_tokens"`
	Cost                        float64   `gorm:"type:decimal(10,6)" json:"cost"`
	CreatedAt                   time.Time `gorm:"index" json:"created_at"`
}

// TableName specifies the table name.
func (UsageRecord) TableName() string {
	return "usage_records"
}
