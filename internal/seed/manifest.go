package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Manifest records the seed output needed by follow-up local tooling.
type Manifest struct {
	Profile     string             `json:"profile"`
	BaseURL     string             `json:"base_url"`
	Tenants     []ManifestTenant   `json:"tenants"`
	Users       []ManifestUser     `json:"users"`
	Providers   []ManifestProvider `json:"providers"`
	VirtualKeys []ManifestKey      `json:"virtual_keys"`
}

type ManifestTenant struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ManifestUser struct {
	ID       uint   `json:"id"`
	TenantID uint   `json:"tenant_id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type ManifestProvider struct {
	Name         string   `json:"name"`
	ProviderType string   `json:"provider_type"`
	Models       []string `json:"models,omitempty"`
}

type ManifestKey struct {
	ID        uint     `json:"id"`
	TenantID  uint     `json:"tenant_id"`
	Tenant    string   `json:"tenant"`
	Name      string   `json:"name"`
	Key       string   `json:"key"`
	Providers []string `json:"providers"`
}

func DefaultManifestPath(profile string) string {
	return filepath.Join(".tmp", "seed", profile+"-manifest.json")
}

func SaveManifest(path string, manifest *Manifest) error {
	if path == "" {
		return fmt.Errorf("manifest path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &manifest, nil
}
