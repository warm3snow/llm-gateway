package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ModelPricing stores per-model token pricing (cents per token).
// Source is typically "portkey" (synced from Portkey-AI/models).
type ModelPricing struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Provider       string    `gorm:"size:32;index:idx_provider_model,unique" json:"provider"`
	Model          string    `gorm:"size:128;index:idx_provider_model,unique" json:"model"`
	InputPrice     float64   `gorm:"type:decimal(12,8)" json:"input_price"`       // cents per token
	OutputPrice    float64   `gorm:"type:decimal(12,8)" json:"output_price"`     // cents per token
	CacheReadPrice float64   `gorm:"type:decimal(12,8);default:0" json:"cache_read_price"` // cents per token
	Currency       string    `gorm:"size:8;default:USD" json:"currency"`
	Source         string    `gorm:"size:32;default:portkey" json:"source"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName specifies the table name.
func (ModelPricing) TableName() string {
	return "model_pricings"
}

// UpsertModelPricing inserts or updates a ModelPricing row keyed on (provider, model).
func UpsertModelPricing(db *gorm.DB, mp *ModelPricing) error {
	mp.UpdatedAt = time.Now()
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "provider"}, {Name: "model"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"input_price":      mp.InputPrice,
			"output_price":     mp.OutputPrice,
			"cache_read_price": mp.CacheReadPrice,
			"currency":         mp.Currency,
			"source":           mp.Source,
			"updated_at":       mp.UpdatedAt,
		}),
	}).Create(mp).Error
}

// GetModelPricing returns pricing for (provider, model). Falls back to the
// provider's "default" row when an exact match is missing. Returns nil if
// neither exists.
func GetModelPricing(db *gorm.DB, provider, model string) (*ModelPricing, error) {
	if db == nil {
		return nil, nil
	}
	var mp ModelPricing
	err := db.Where("provider = ? AND model = ?", provider, model).First(&mp).Error
	if err == nil {
		return &mp, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// Fall back to default row for the provider.
	err = db.Where("provider = ? AND model = ?", provider, "default").First(&mp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &mp, nil
}

// GetAllModelPricings returns all pricing rows.
func GetAllModelPricings(db *gorm.DB) ([]ModelPricing, error) {
	var rows []ModelPricing
	if db == nil {
		return rows, nil
	}
	err := db.Find(&rows).Error
	return rows, err
}
