package models

import (
	"time"
)

// CacheEntry represents a cached response
type CacheEntry struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CacheKey     string    `gorm:"size:64;uniqueIndex;not null" json:"-"` // Hash of request
	CacheKeyShort string    `gorm:"size:16" json:"cache_key_short"`  // First 8 chars for identification
	RequestText  string    `gorm:"type:text" json:"request_text,omitempty"`
	ResponseText string    `gorm:"type:text" json:"-"` // Response body, excluded from list queries
	Provider     string    `gorm:"size:50;index" json:"provider"`
	Model        string    `gorm:"size:100;index" json:"model"`
	Embedding    []byte    `gorm:"type:blob" json:"-"` // For semantic caching
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	ExpiresAt    time.Time `gorm:"index" json:"expires_at"`
	AccessCount  int       `gorm:"default:0" json:"access_count"`
	LastAccessAt time.Time `json:"last_access_at"`
}

// TableName specifies the table name
func (CacheEntry) TableName() string {
	return "cache_entries"
}

// CacheEntryResponse is used for API responses (excludes sensitive fields)
type CacheEntryResponse struct {
	ID            uint      `json:"id"`
	CacheKeyShort string    `json:"cache_key_short"`
	RequestText   string    `json:"request_text,omitempty"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	AccessCount   int       `json:"access_count"`
	LastAccessAt  time.Time `json:"last_access_at"`
}
