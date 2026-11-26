package service

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/pkg/redact"
)

// RedactRegistry holds the live masking engine and the configuration it was
// built from. The proxy pipeline reads the engine per request; the admin API
// swaps configuration atomically.
type RedactRegistry struct {
	mu     sync.RWMutex
	engine *redact.Engine
	opts   redact.Options
	config config.RedactConfig
}

var redactRegistry = &RedactRegistry{engine: redact.New()}

// RedactEngine returns the process-wide masking engine. Never nil.
func RedactEngine() *redact.Engine {
	return redactRegistry.engine
}

// RedactOptions returns the currently applied options and whether masking is
// enabled.
func RedactOptions() (redact.Options, bool) {
	redactRegistry.mu.RLock()
	defer redactRegistry.mu.RUnlock()
	return redactRegistry.opts, redactRegistry.config.Enabled
}

// RedactOptionsFor validates a candidate configuration into options without
// installing it (used by the preview endpoint).
func RedactOptionsFor(cfg config.RedactConfig) (redact.Options, error) {
	return redactOptionsFromConfig(cfg)
}

// ApplyRedactConfig validates and installs a masking configuration. Returns
// an error when a custom rule is invalid.
func ApplyRedactConfig(cfg config.RedactConfig) error {
	opts, err := redactOptionsFromConfig(cfg)
	if err != nil {
		return err
	}
	redactRegistry.mu.Lock()
	defer redactRegistry.mu.Unlock()
	redactRegistry.config = cfg
	redactRegistry.opts = opts
	return nil
}

// RedactConfig returns the currently installed configuration.
func RedactConfig() config.RedactConfig {
	redactRegistry.mu.RLock()
	defer redactRegistry.mu.RUnlock()
	return redactRegistry.config
}

func redactOptionsFromConfig(cfg config.RedactConfig) (redact.Options, error) {
	rules := make([]redact.Rule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		// Regex rules are validated here so a bad pattern fails the request
		// instead of silently disabling the rule.
		if r.Prefix == "" && r.Regex != "" {
			if _, err := regexp.Compile(r.Regex); err != nil {
				return redact.Options{}, fmt.Errorf("rule %q: %w", r.Kind, err)
			}
		}
		rules = append(rules, redact.Rule{Kind: r.Kind, Prefix: r.Prefix, Regex: r.Regex})
	}
	actions := make(map[string]string, len(cfg.Actions))
	for kind, action := range cfg.Actions {
		actions[kind] = action
	}
	return redact.Options{
		Secrets:      cfg.Secrets,
		Personal:     cfg.Personal,
		Kinds:        cfg.Kinds,
		Words:        cfg.Words,
		WordsAnyCase: cfg.WordsAnyCase,
		WordsWhole:   cfg.WordsWhole,
		Rules:        rules,
		Actions:      actions,
		Allow:        cfg.Allow,
	}, nil
}
