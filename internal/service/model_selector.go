package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/types"
)

const (
	DefaultAutoRecentWindowSeconds  = 300
	DefaultAutoMaxConcurrency       = 20
	DefaultAutoOutputTokens         = 1024
	DefaultAutoCostWeight           = 0.50
	DefaultAutoConcurrencyWeight    = 0.25
	DefaultAutoRecentUsageWeight    = 0.15
	DefaultAutoErrorWeight          = 0.05
	DefaultAutoProviderWeightWeight = 0.05
)

var ErrNoAutoModelCandidates = errors.New("no auto-mode candidate provider/model available")

type SelectionHint struct {
	ProviderName     string
	AllowedProviders []string
}

type Selection struct {
	ProviderName string
	ProviderType string
	Options      types.Options
	Model        string
}

type ModelSelector struct {
	cfg     *config.Config
	tracker *ModelConcurrencyTracker
}

type ModelConcurrencyTracker struct {
	counts sync.Map
}

type candidate struct {
	providerName   string
	providerType   string
	options        types.Options
	model          string
	inputPrice     float64
	outputPrice    float64
	cost           float64
	inflight       int64
	maxConcurrency int
	recentCount    int64
	errorRate      float64
	weight         int
	score          float64
}

type autoModeSettings struct {
	CostWeight                  float64
	ConcurrencyWeight           float64
	RecentUsageWeight           float64
	ErrorWeight                 float64
	ProviderWeightPenaltyWeight float64
	RecentWindowSeconds         int
	DefaultMaxConcurrency       int
	DefaultOutputTokens         int
}

func NewModelSelector(cfg *config.Config, tracker *ModelConcurrencyTracker) *ModelSelector {
	if tracker == nil {
		tracker = NewModelConcurrencyTracker()
	}
	return &ModelSelector{cfg: cfg, tracker: tracker}
}

func (s *ModelSelector) settings() autoModeSettings {
	settings := autoModeSettings{
		CostWeight:                  DefaultAutoCostWeight,
		ConcurrencyWeight:           DefaultAutoConcurrencyWeight,
		RecentUsageWeight:           DefaultAutoRecentUsageWeight,
		ErrorWeight:                 DefaultAutoErrorWeight,
		ProviderWeightPenaltyWeight: DefaultAutoProviderWeightWeight,
		RecentWindowSeconds:         DefaultAutoRecentWindowSeconds,
		DefaultMaxConcurrency:       DefaultAutoMaxConcurrency,
		DefaultOutputTokens:         DefaultAutoOutputTokens,
	}
	if s == nil || s.cfg == nil {
		return settings
	}
	autoMode := s.cfg.Gateway.AutoMode
	if autoMode.CostWeight > 0 {
		settings.CostWeight = autoMode.CostWeight
	}
	if autoMode.ConcurrencyWeight > 0 {
		settings.ConcurrencyWeight = autoMode.ConcurrencyWeight
	}
	if autoMode.RecentUsageWeight > 0 {
		settings.RecentUsageWeight = autoMode.RecentUsageWeight
	}
	if autoMode.ErrorWeight > 0 {
		settings.ErrorWeight = autoMode.ErrorWeight
	}
	if autoMode.ProviderWeightPenaltyWeight > 0 {
		settings.ProviderWeightPenaltyWeight = autoMode.ProviderWeightPenaltyWeight
	}
	if autoMode.RecentWindowSeconds > 0 {
		settings.RecentWindowSeconds = autoMode.RecentWindowSeconds
	}
	if autoMode.DefaultMaxConcurrency > 0 {
		settings.DefaultMaxConcurrency = autoMode.DefaultMaxConcurrency
	}
	if autoMode.DefaultOutputTokens > 0 {
		settings.DefaultOutputTokens = autoMode.DefaultOutputTokens
	}
	return settings
}

func NewModelConcurrencyTracker() *ModelConcurrencyTracker {
	return &ModelConcurrencyTracker{}
}

func (t *ModelConcurrencyTracker) Begin(providerName, model string) func() {
	if t == nil {
		return func() {}
	}
	counter := t.counter(providerName, model)
	counter.Add(1)
	return func() {
		counter.Add(-1)
	}
}

func (t *ModelConcurrencyTracker) Snapshot(providerName, model string) int64 {
	if t == nil {
		return 0
	}
	counter := t.counter(providerName, model)
	return counter.Load()
}

func (t *ModelConcurrencyTracker) TryBegin(providerName, model string, maxConcurrency int) (func(), bool) {
	if t == nil {
		return func() {}, true
	}
	counter := t.counter(providerName, model)
	for {
		current := counter.Load()
		if maxConcurrency > 0 && current >= int64(maxConcurrency) {
			return nil, false
		}
		if counter.CompareAndSwap(current, current+1) {
			return func() {
				counter.Add(-1)
			}, true
		}
	}
}

func (t *ModelConcurrencyTracker) counter(providerName, model string) *atomic.Int64 {
	key := providerName + "\x00" + model
	if existing, ok := t.counts.Load(key); ok {
		return existing.(*atomic.Int64)
	}
	counter := &atomic.Int64{}
	actual, _ := t.counts.LoadOrStore(key, counter)
	return actual.(*atomic.Int64)
}

func (s *ModelSelector) Select(ctx context.Context, req *types.ChatCompletionRequest, hint SelectionHint) (*Selection, error) {
	candidates, err := s.rankedCandidates(ctx, req, hint)
	if err != nil {
		return nil, err
	}
	return selectionFromCandidate(candidates[0]), nil
}

func (s *ModelSelector) SelectAndReserve(ctx context.Context, req *types.ChatCompletionRequest, hint SelectionHint) (*Selection, func(), error) {
	candidates, err := s.rankedCandidates(ctx, req, hint)
	if err != nil {
		return nil, nil, err
	}
	for _, candidate := range candidates {
		done, ok := s.tracker.TryBegin(candidate.providerName, candidate.model, candidate.maxConcurrency)
		if ok {
			return selectionFromCandidate(candidate), done, nil
		}
	}
	return nil, nil, ErrNoAutoModelCandidates
}

func (s *ModelSelector) rankedCandidates(ctx context.Context, req *types.ChatCompletionRequest, hint SelectionHint) ([]candidate, error) {
	if s == nil || s.cfg == nil {
		return nil, fmt.Errorf("auto-mode selector is not configured")
	}
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database is not connected")
	}

	pricingRows, err := models.GetAllModelPricings(db)
	if err != nil {
		return nil, err
	}
	pricingByProvider := groupPricingByProvider(pricingRows)
	allowedProviders := setFromStrings(hint.AllowedProviders)
	settings := s.settings()
	inputTokens := estimateInputTokens(req)
	outputTokens := estimateOutputTokens(req, settings.DefaultOutputTokens)
	recentStats := loadRecentStats(ctx, settings.RecentWindowSeconds)

	s.cfg.Gateway.ProvidersMu.RLock()
	providers := make(map[string]types.Options, len(s.cfg.Gateway.Providers))
	for name, opts := range s.cfg.Gateway.Providers {
		providers[name] = opts
	}
	s.cfg.Gateway.ProvidersMu.RUnlock()

	candidates := make([]candidate, 0)
	for providerName, opts := range providers {
		if hint.ProviderName != "" && providerName != hint.ProviderName {
			continue
		}
		if len(allowedProviders) > 0 && !allowedProviders[providerName] {
			continue
		}
		if !autoEnabled(opts.Metadata) {
			continue
		}
		providerType := opts.Provider
		if providerType == "" {
			providerType = providerName
		}
		rows := pricingByProvider[providerType]
		if len(rows) == 0 {
			continue
		}
		autoModels := setFromStrings(splitList(opts.Metadata["auto_models"]))
		maxConcurrency := metadataInt(opts.Metadata, "auto_max_concurrency", settings.DefaultMaxConcurrency)
		if maxConcurrency <= 0 {
			maxConcurrency = DefaultAutoMaxConcurrency
		}
		for _, row := range rows {
			if row.Model == "default" || (row.InputPrice == 0 && row.OutputPrice == 0) {
				continue
			}
			if len(autoModels) > 0 && !autoModels[row.Model] {
				continue
			}
			inflight := s.tracker.Snapshot(providerName, row.Model)
			if inflight >= int64(maxConcurrency) {
				continue
			}
			stats := recentStats[statsKey(providerType, row.Model)]
			candidates = append(candidates, candidate{
				providerName:   providerName,
				providerType:   providerType,
				options:        opts,
				model:          row.Model,
				inputPrice:     row.InputPrice,
				outputPrice:    row.OutputPrice,
				cost:           float64(inputTokens)*row.InputPrice + float64(outputTokens)*row.OutputPrice,
				inflight:       inflight,
				maxConcurrency: maxConcurrency,
				recentCount:    stats.total,
				errorRate:      stats.errorRate(),
				weight:         opts.Weight,
				score:          0,
			})
		}
	}

	if len(candidates) == 0 {
		return nil, ErrNoAutoModelCandidates
	}

	scoreCandidates(candidates, settings)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			if candidates[i].providerName == candidates[j].providerName {
				return candidates[i].model < candidates[j].model
			}
			return candidates[i].providerName < candidates[j].providerName
		}
		return candidates[i].score < candidates[j].score
	})
	return candidates, nil
}

func selectionFromCandidate(candidate candidate) *Selection {
	return &Selection{
		ProviderName: candidate.providerName,
		ProviderType: candidate.providerType,
		Options:      candidate.options,
		Model:        candidate.model,
	}
}

func groupPricingByProvider(rows []models.ModelPricing) map[string][]models.ModelPricing {
	grouped := make(map[string][]models.ModelPricing)
	for _, row := range rows {
		grouped[row.Provider] = append(grouped[row.Provider], row)
	}
	return grouped
}

func scoreCandidates(candidates []candidate, settings autoModeSettings) {
	maxCost := maxFloat(candidates, func(c candidate) float64 { return c.cost })
	maxInflight := maxFloat(candidates, func(c candidate) float64 { return c.inflightLoad() })
	maxRecent := maxFloat(candidates, func(c candidate) float64 { return float64(c.recentCount) })
	maxWeight := maxFloat(candidates, func(c candidate) float64 { return float64(c.weight) })
	if maxWeight <= 0 {
		maxWeight = 1
	}
	for i := range candidates {
		normCost := normalize(candidates[i].cost, maxCost)
		normInflight := normalize(candidates[i].inflightLoad(), maxInflight)
		normRecent := normalize(float64(candidates[i].recentCount), maxRecent)
		providerWeightPenalty := 1 - normalize(float64(candidates[i].weight), maxWeight)
		candidates[i].score = settings.CostWeight*normCost +
			settings.ConcurrencyWeight*normInflight +
			settings.RecentUsageWeight*normRecent +
			settings.ErrorWeight*candidates[i].errorRate +
			settings.ProviderWeightPenaltyWeight*providerWeightPenalty
	}
}

func (c candidate) inflightLoad() float64 {
	if c.maxConcurrency <= 0 {
		return float64(c.inflight)
	}
	return float64(c.inflight) / float64(c.maxConcurrency)
}

func maxFloat(candidates []candidate, value func(candidate) float64) float64 {
	maxValue := 0.0
	for _, c := range candidates {
		maxValue = math.Max(maxValue, value(c))
	}
	return maxValue
}

func normalize(value, maxValue float64) float64 {
	if maxValue <= 0 {
		return 0
	}
	return value / maxValue
}

type recentUsageStats struct {
	total  int64
	errors int64
}

func (s recentUsageStats) errorRate() float64 {
	if s.total == 0 {
		return 0
	}
	return float64(s.errors) / float64(s.total)
}

func loadRecentStats(ctx context.Context, windowSeconds int) map[string]recentUsageStats {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	if windowSeconds <= 0 {
		windowSeconds = DefaultAutoRecentWindowSeconds
	}
	since := time.Now().Add(-time.Duration(windowSeconds) * time.Second)
	var rows []struct {
		Provider string
		Model    string
		Total    int64
		Errors   int64
	}
	db.WithContext(ctx).Model(&models.UsageRecord{}).
		Select("provider, model, COUNT(*) AS total, SUM(CASE WHEN status_code = ? OR status_code >= ? THEN 1 ELSE 0 END) AS errors", 429, 500).
		Where("created_at >= ?", since).
		Group("provider, model").
		Scan(&rows)
	stats := make(map[string]recentUsageStats, len(rows))
	for _, row := range rows {
		stats[statsKey(row.Provider, row.Model)] = recentUsageStats{total: row.Total, errors: row.Errors}
	}
	return stats
}

func statsKey(provider, model string) string {
	return provider + "\x00" + model
}

func estimateInputTokens(req *types.ChatCompletionRequest) int {
	if req == nil {
		return 1
	}
	chars := 0
	for _, msg := range req.Messages {
		chars += len(fmt.Sprint(msg.Content))
	}
	if chars == 0 {
		return 1
	}
	return maxInt(1, chars/4)
}

func estimateOutputTokens(req *types.ChatCompletionRequest, defaultOutputTokens int) int {
	if req != nil && req.MaxTokens > 0 {
		return req.MaxTokens
	}
	if defaultOutputTokens > 0 {
		return defaultOutputTokens
	}
	return DefaultAutoOutputTokens
}

func autoEnabled(metadata map[string]string) bool {
	if metadata == nil {
		return true
	}
	value := strings.TrimSpace(strings.ToLower(metadata["auto_enabled"]))
	return value != "false" && value != "0" && value != "no"
}

func metadataInt(metadata map[string]string, key string, fallback int) int {
	if metadata == nil {
		return fallback
	}
	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func setFromStrings(values []string) map[string]bool {
	set := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	return set
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
