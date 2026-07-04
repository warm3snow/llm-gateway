package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

// PortkeyPricingURL is the base URL for Portkey's free pricing JSON files.
const PortkeyPricingURL = "https://configs.portkey.ai/pricing"

// portkeyProviderName maps our internal provider names to Portkey's file names.
var portkeyProviderName = map[string]string{
	"openai":    "openai",
	"deepseek":  "deepseek",
	"anthropic": "anthropic",
	"gemini":    "google", // Portkey uses "google" for Gemini
	"glm":       "zhipu",  // Portkey uses "zhipu" for GLM
	"kimi":      "moonshot", // Portkey uses "moonshot" for Kimi
}

// AllPortkeyProviders returns the list of supported internal provider names.
func AllPortkeyProviders() []string {
	return []string{"openai", "deepseek", "anthropic", "gemini", "glm", "kimi"}
}

// PricingService fetches and stores model pricing from Portkey.
type PricingService struct {
	db         *gorm.DB
	httpClient *http.Client
}

// NewPricingService creates a PricingService.
func NewPricingService(db *gorm.DB) *PricingService {
	return &PricingService{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// portkeySchema mirrors the Portkey pricing JSON shape (only fields we use).
type portkeySchema map[string]struct {
	PricingConfig struct {
		PayAsYouGo struct {
			RequestToken        *struct{ Price float64 `json:"price"` } `json:"request_token"`
			ResponseToken       *struct{ Price float64 `json:"price"` } `json:"response_token"`
			CacheReadInputToken *struct{ Price float64 `json:"price"` } `json:"cache_read_input_token"`
		} `json:"pay_as_you_go"`
		Currency string `json:"currency"`
	} `json:"pricing_config"`
}

// FetchAndStore fetches pricing for the given providers (internal names) and
// upserts rows into model_pricings. If providers is empty, all supported
// providers are fetched. Returns the total number of rows parsed/stored.
// When dryRun is true, rows are parsed and counted but not written to the DB.
func (s *PricingService) FetchAndStore(providers []string, dryRun bool) (int, error) {
	if len(providers) == 0 {
		providers = AllPortkeyProviders()
	}

	total := 0
	var firstErr error

	for _, prov := range providers {
		portkeyName, ok := portkeyProviderName[prov]
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("unknown provider: %s", prov)
			}
			continue
		}

		n, err := s.fetchAndStoreOne(prov, portkeyName, dryRun)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("provider %s: %w", prov, err)
			}
			continue
		}
		total += n
	}

	return total, firstErr
}

func (s *PricingService) fetchAndStoreOne(provider, portkeyName string, dryRun bool) (int, error) {
	url := fmt.Sprintf("%s/%s.json", PortkeyPricingURL, portkeyName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("portkey returned %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var schema portkeySchema
	if err := json.Unmarshal(body, &schema); err != nil {
		return 0, fmt.Errorf("parse pricing JSON: %w", err)
	}

	count := 0
	for model, cfg := range schema {
		inputPrice, outputPrice, cacheReadPrice := 0.0, 0.0, 0.0
		if cfg.PricingConfig.PayAsYouGo.RequestToken != nil {
			inputPrice = cfg.PricingConfig.PayAsYouGo.RequestToken.Price
		}
		if cfg.PricingConfig.PayAsYouGo.ResponseToken != nil {
			outputPrice = cfg.PricingConfig.PayAsYouGo.ResponseToken.Price
		}
		if cfg.PricingConfig.PayAsYouGo.CacheReadInputToken != nil {
			cacheReadPrice = cfg.PricingConfig.PayAsYouGo.CacheReadInputToken.Price
		}
		currency := cfg.PricingConfig.Currency
		if currency == "" {
			currency = "USD"
		}

		// Skip models with all-zero prices (e.g. the "default" entry for some
		// providers is 0/0 — keep it though, as a fallback marker). Keep the
		// default row even at zero so GetModelPricing has a fallback target.
		if model != "default" && inputPrice == 0 && outputPrice == 0 {
			continue
		}

		if dryRun {
			count++
			continue
		}

		mp := &models.ModelPricing{
			Provider:       provider,
			Model:          model,
			InputPrice:     inputPrice,
			OutputPrice:    outputPrice,
			CacheReadPrice: cacheReadPrice,
			Currency:       currency,
			Source:         "portkey",
		}
		if err := models.UpsertModelPricing(s.db, mp); err != nil {
			return count, fmt.Errorf("upsert %s/%s: %w", provider, model, err)
		}
		count++
	}

	return count, nil
}

// FormatProvidersList returns a comma-separated string of provider names for logging.
func FormatProvidersList(providers []string) string {
	return strings.Join(providers, ", ")
}
