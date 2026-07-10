package models

import "time"

const (
	IdempotencyStatusProcessing = "processing"
	IdempotencyStatusCompleted  = "completed"
)

// IdempotencyRecord stores the first completed response for a virtual-key scoped
// Idempotency-Key so client retries do not invoke upstream providers or bill twice.
type IdempotencyRecord struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	VirtualKeyID        uint       `gorm:"not null;uniqueIndex:idx_idempotency_vk_key" json:"virtual_key_id"`
	IdempotencyKey      string     `gorm:"column:idempotency_key;size:255;not null;uniqueIndex:idx_idempotency_vk_key" json:"idempotency_key"`
	RequestHash         string     `gorm:"size:64;not null" json:"request_hash"`
	Status              string     `gorm:"size:20;not null;index" json:"status"`
	ResponseStatus      int        `json:"response_status"`
	ResponseContentType string     `gorm:"size:255" json:"response_content_type,omitempty"`
	ResponseBody        []byte     `gorm:"type:blob" json:"-"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	ExpiresAt           time.Time  `gorm:"index" json:"expires_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (IdempotencyRecord) TableName() string {
	return "idempotency_records"
}
