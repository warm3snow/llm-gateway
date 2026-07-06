package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/encryption"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrProviderAlreadyExists    = errors.New("provider already exists")
	ErrProviderNotFound         = errors.New("provider not found")
	ErrProviderStoreUnavailable = errors.New("provider store unavailable")
)

// ProviderConfigService handles database-backed provider configuration.
type ProviderConfigService struct {
	db *gorm.DB
}

// NewProviderConfigService creates a new ProviderConfigService.
func NewProviderConfigService() *ProviderConfigService {
	return &ProviderConfigService{db: database.GetDB()}
}

// List returns all provider configs ordered by name.
func (s *ProviderConfigService) List() ([]models.ProviderConfig, error) {
	if s.db == nil {
		return nil, ErrProviderStoreUnavailable
	}
	var providers []models.ProviderConfig
	if err := s.db.Order("name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// GetByName returns a provider config by name.
func (s *ProviderConfigService) GetByName(name string) (*models.ProviderConfig, error) {
	if s.db == nil {
		return nil, ErrProviderStoreUnavailable
	}
	var provider models.ProviderConfig
	if err := s.db.Where("name = ?", name).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return &provider, nil
}

// Create inserts a new provider config.
func (s *ProviderConfigService) Create(name string, opts types.Options) (*models.ProviderConfig, error) {
	if s.db == nil {
		return nil, ErrProviderStoreUnavailable
	}
	if _, err := s.GetByName(name); err == nil {
		return nil, ErrProviderAlreadyExists
	} else if !errors.Is(err, ErrProviderNotFound) {
		return nil, err
	}

	provider, err := optionsToModel(name, opts)
	if err != nil {
		return nil, err
	}
	provider.Enabled = true
	if err := s.db.Create(provider).Error; err != nil {
		return nil, fmt.Errorf("failed to create provider config: %w", err)
	}
	return provider, nil
}

// Update updates an existing provider config. Empty APIKey keeps the stored key.
func (s *ProviderConfigService) Update(name string, opts types.Options) (*models.ProviderConfig, error) {
	if s.db == nil {
		return nil, ErrProviderStoreUnavailable
	}
	current, err := s.GetByName(name)
	if err != nil {
		return nil, err
	}

	provider, err := optionsToModel(name, opts)
	if err != nil {
		return nil, err
	}
	provider.ID = current.ID
	provider.CreatedAt = current.CreatedAt
	provider.Enabled = current.Enabled
	if opts.APIKey == "" {
		provider.APIKeyEncrypted = current.APIKeyEncrypted
	}
	if provider.Weight == 0 {
		provider.Weight = current.Weight
	}

	if err := s.db.Save(provider).Error; err != nil {
		return nil, fmt.Errorf("failed to update provider config: %w", err)
	}
	return provider, nil
}

// Delete soft-deletes a provider config by name.
func (s *ProviderConfigService) Delete(name string) error {
	if s.db == nil {
		return ErrProviderStoreUnavailable
	}
	res := s.db.Where("name = ?", name).Delete(&models.ProviderConfig{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete provider config: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrProviderNotFound
	}
	return nil
}

// Upsert inserts or updates a provider config by name.
func (s *ProviderConfigService) Upsert(name string, opts types.Options) error {
	provider, err := optionsToModel(name, opts)
	if err != nil {
		return err
	}
	provider.Enabled = true
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"provider_type":     provider.ProviderType,
			"api_key_encrypted": provider.APIKeyEncrypted,
			"base_url":          provider.BaseURL,
			"weight":            provider.Weight,
			"enabled":           provider.Enabled,
			"config":            provider.Config,
			"updated_at":        provider.UpdatedAt,
		}),
	}).Create(provider).Error
}

// ToOptions converts a ProviderConfig row to runtime provider options.
func (s *ProviderConfigService) ToOptions(provider *models.ProviderConfig) (types.Options, error) {
	return modelToOptions(provider)
}

func optionsToModel(name string, opts types.Options) (*models.ProviderConfig, error) {
	apiKeyEncrypted := ""
	if opts.APIKey != "" {
		ciphertext, err := encryption.Encrypt(opts.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt provider api key: %w", err)
		}
		apiKeyEncrypted = ciphertext
	}

	extra := providerExtraConfig{
		VirtualKey:         opts.VirtualKey,
		Retry:              opts.Retry,
		OverrideParams:     opts.OverrideParams,
		URLToFetch:         opts.URLToFetch,
		ResourceName:       opts.ResourceName,
		DeploymentID:       opts.DeploymentID,
		APIVersion:         opts.APIVersion,
		AzureAuthMode:      opts.AzureAuthMode,
		AWSSecretAccessKey: opts.AWSSecretAccessKey,
		AWSAccessKeyID:     opts.AWSAccessKeyID,
		AWSRegion:          opts.AWSRegion,
		ForwardHeaders:     opts.ForwardHeaders,
		RequestTimeout:     opts.RequestTimeout,
		Metadata:           opts.Metadata,
		Index:              opts.Index,
	}
	configBytes, err := json.Marshal(extra)
	if err != nil {
		return nil, fmt.Errorf("failed to encode provider config: %w", err)
	}
	extraConfigEncrypted, err := encryption.Encrypt(string(configBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt provider config: %w", err)
	}

	weight := opts.Weight
	if weight == 0 {
		weight = 1
	}

	return &models.ProviderConfig{
		Name:            name,
		ProviderType:    opts.Provider,
		APIKeyEncrypted: apiKeyEncrypted,
		BaseURL:         opts.CustomHost,
		Weight:          weight,
		Enabled:         true,
		Config:          extraConfigEncrypted,
	}, nil
}

func modelToOptions(provider *models.ProviderConfig) (types.Options, error) {
	extra := providerExtraConfig{}
	if provider.Config != "" {
		configJSON, err := encryption.Decrypt(provider.Config)
		if err != nil {
			// Backward compatibility for rows written before Config was encrypted.
			configJSON = provider.Config
		}
		if err := json.Unmarshal([]byte(configJSON), &extra); err != nil {
			return types.Options{}, fmt.Errorf("failed to decode provider config: %w", err)
		}
	}

	apiKey := ""
	if provider.APIKeyEncrypted != "" {
		plaintext, err := encryption.Decrypt(provider.APIKeyEncrypted)
		if err != nil {
			return types.Options{}, fmt.Errorf("failed to decrypt provider api key: %w", err)
		}
		apiKey = plaintext
	}

	return types.Options{
		Provider:           provider.ProviderType,
		VirtualKey:         extra.VirtualKey,
		APIKey:             apiKey,
		Weight:             provider.Weight,
		Retry:              extra.Retry,
		OverrideParams:     extra.OverrideParams,
		URLToFetch:         extra.URLToFetch,
		ResourceName:       extra.ResourceName,
		DeploymentID:       extra.DeploymentID,
		APIVersion:         extra.APIVersion,
		AzureAuthMode:      extra.AzureAuthMode,
		AWSSecretAccessKey: extra.AWSSecretAccessKey,
		AWSAccessKeyID:     extra.AWSAccessKeyID,
		AWSRegion:          extra.AWSRegion,
		CustomHost:         provider.BaseURL,
		ForwardHeaders:     extra.ForwardHeaders,
		RequestTimeout:     extra.RequestTimeout,
		Metadata:           extra.Metadata,
		Index:              extra.Index,
	}, nil
}

type providerExtraConfig struct {
	VirtualKey         string                 `json:"virtualKey,omitempty"`
	Retry              *types.RetrySettings   `json:"retry,omitempty"`
	OverrideParams     map[string]interface{} `json:"overrideParams,omitempty"`
	URLToFetch         string                 `json:"urlToFetch,omitempty"`
	ResourceName       string                 `json:"resourceName,omitempty"`
	DeploymentID       string                 `json:"deploymentId,omitempty"`
	APIVersion         string                 `json:"apiVersion,omitempty"`
	AzureAuthMode      string                 `json:"azureAuthMode,omitempty"`
	AWSSecretAccessKey string                 `json:"awsSecretAccessKey,omitempty"`
	AWSAccessKeyID     string                 `json:"awsAccessKeyId,omitempty"`
	AWSRegion          string                 `json:"awsRegion,omitempty"`
	ForwardHeaders     []string               `json:"forwardHeaders,omitempty"`
	RequestTimeout     int                    `json:"requestTimeout,omitempty"`
	Metadata           map[string]string      `json:"metadata,omitempty"`
	Index              int                    `json:"index,omitempty"`
}
