package guardrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// Guardrail validates model requests and responses.
type Guardrail interface {
	GetName() string
	ValidateRequest(req any) (*types.GuardrailResult, error)
	ValidateResponse(resp any) (*types.GuardrailResult, error)
}

type GuardrailManager struct {
	enabled    bool
	guardrails []Guardrail
}

func NewGuardrailManager(enabled ...bool) *GuardrailManager {
	isEnabled := true
	if len(enabled) > 0 {
		isEnabled = enabled[0]
	}
	return &GuardrailManager{enabled: isEnabled, guardrails: []Guardrail{}}
}

func NewManagerFromConfig(enabled bool, configs []types.GuardrailConfig) (*GuardrailManager, error) {
	manager := NewGuardrailManager(enabled)
	if !enabled {
		return manager, nil
	}

	for _, cfg := range configs {
		if strings.EqualFold(cfg.OnFailure, "allow") || strings.EqualFold(cfg.OnFailure, "warn") {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
		case "pii", "pii_detector":
			manager.RegisterGuardrail(NewPIIDetector())
		case "keyword", "keyword_filter":
			keywords := stringSliceParam(cfg.Parameters, "keywords")
			matchMode := stringParam(cfg.Parameters, "matchMode", "contains")
			caseSensitive := boolParam(cfg.Parameters, "caseSensitive", false)
			manager.RegisterGuardrail(NewKeywordFilter(keywords, matchMode, caseSensitive))
		case "length", "length_limiter":
			maxInput := intParam(cfg.Parameters, "maxInputLength", 0)
			maxOutput := intParam(cfg.Parameters, "maxOutputLength", 0)
			manager.RegisterGuardrail(NewLengthLimiter(maxInput, maxOutput))
		case "":
			return nil, fmt.Errorf("guardrail type is required")
		default:
			return nil, fmt.Errorf("unsupported guardrail type: %s", cfg.Type)
		}
	}

	return manager, nil
}

func (m *GuardrailManager) Enabled() bool {
	return m != nil && m.enabled
}

func (m *GuardrailManager) Empty() bool {
	return m == nil || len(m.guardrails) == 0
}

func (m *GuardrailManager) RegisterGuardrail(g Guardrail) {
	if g == nil {
		return
	}
	m.guardrails = append(m.guardrails, g)
}

func (m *GuardrailManager) ValidateRequest(req any) (*types.GuardrailResult, error) {
	if !m.Enabled() || m.Empty() {
		return &types.GuardrailResult{Passed: true}, nil
	}
	for _, g := range m.guardrails {
		result, err := g.ValidateRequest(req)
		if err != nil {
			return nil, err
		}
		if result != nil && !result.Passed {
			if result.Message == "" {
				result.Message = "Request blocked by guardrail"
			}
			if result.Reason != "" {
				result.Reason = g.GetName() + ": " + result.Reason
			}
			return result, nil
		}
	}
	return &types.GuardrailResult{Passed: true}, nil
}

func (m *GuardrailManager) ValidateResponse(resp any) (*types.GuardrailResult, error) {
	if !m.Enabled() || m.Empty() {
		return &types.GuardrailResult{Passed: true}, nil
	}
	for _, g := range m.guardrails {
		result, err := g.ValidateResponse(resp)
		if err != nil {
			return nil, err
		}
		if result != nil && !result.Passed {
			if result.Message == "" {
				result.Message = "Response blocked by guardrail"
			}
			if result.Reason != "" {
				result.Reason = g.GetName() + ": " + result.Reason
			}
			return result, nil
		}
	}
	return &types.GuardrailResult{Passed: true}, nil
}

type PIIDetector struct {
	Name        string
	patterns    map[string]*regexp.Regexp
	maskEnabled bool
	maskChar    rune
}

func NewPIIDetector() *PIIDetector {
	return &PIIDetector{
		Name: "pii_detector",
		patterns: map[string]*regexp.Regexp{
			"email":     regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			"phone":     regexp.MustCompile(`(1[3-9]\d{9}|\+86[1-9]\d{10})`),
			"id_card":   regexp.MustCompile(`[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
			"bank_card": regexp.MustCompile(`(4[0-9]{12,15}|5[1-5][0-9]{14}|3[47][0-9]{13}|3[0,6,8][0-9]{12}|6(?:011|5[0-9][0-9])[0-9]{12})`),
		},
		maskEnabled: true,
		maskChar:    '*',
	}
}

func (d *PIIDetector) GetName() string { return d.Name }

func (d *PIIDetector) ValidateRequest(req any) (*types.GuardrailResult, error) {
	return d.validate(req, "PII detected in request")
}

func (d *PIIDetector) ValidateResponse(resp any) (*types.GuardrailResult, error) {
	return d.validate(resp, "PII detected in response")
}

func (d *PIIDetector) validate(value any, message string) (*types.GuardrailResult, error) {
	text, err := toJSONString(value)
	if err != nil {
		return nil, err
	}
	detected := make([]string, 0)
	for name, pattern := range d.patterns {
		if pattern.MatchString(text) {
			detected = append(detected, name)
		}
	}
	if len(detected) == 0 {
		return &types.GuardrailResult{Passed: true}, nil
	}
	result := &types.GuardrailResult{
		Passed:  false,
		Message: message,
		Reason:  fmt.Sprintf("detected PII types: %s", strings.Join(detected, ", ")),
		Actions: []string{"deny"},
	}
	if d.maskEnabled {
		result.Actions = append(result.Actions, "mask")
		result.MaskedContent = d.maskContent(text, detected)
	}
	return result, nil
}

func (d *PIIDetector) maskContent(text string, detected []string) string {
	masked := text
	for _, name := range detected {
		pattern := d.patterns[name]
		masked = pattern.ReplaceAllStringFunc(masked, func(match string) string {
			return strings.Repeat(string(d.maskChar), utf8.RuneCountInString(match))
		})
	}
	return masked
}

type KeywordFilter struct {
	Name          string
	Keywords      []string
	MatchMode     string
	CaseSensitive bool
}

func NewKeywordFilter(keywords []string, matchMode string, caseSensitive bool) *KeywordFilter {
	matchMode = strings.ToLower(strings.TrimSpace(matchMode))
	if matchMode == "" {
		matchMode = "contains"
	}
	return &KeywordFilter{
		Name:          "keyword_filter",
		Keywords:      keywords,
		MatchMode:     matchMode,
		CaseSensitive: caseSensitive,
	}
}

func (f *KeywordFilter) GetName() string { return f.Name }

func (f *KeywordFilter) ValidateRequest(req any) (*types.GuardrailResult, error) {
	return f.validate(req, "Blocked keywords detected in request")
}

func (f *KeywordFilter) ValidateResponse(resp any) (*types.GuardrailResult, error) {
	return f.validate(resp, "Blocked keywords detected in response")
}

func (f *KeywordFilter) validate(value any, message string) (*types.GuardrailResult, error) {
	text, err := toJSONString(value)
	if err != nil {
		return nil, err
	}
	matched, err := f.matchKeywords(text)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return &types.GuardrailResult{Passed: true}, nil
	}
	return &types.GuardrailResult{
		Passed:  false,
		Message: message,
		Reason:  fmt.Sprintf("matched blocked keywords: %s", strings.Join(matched, ", ")),
		Actions: []string{"deny"},
	}, nil
}

func (f *KeywordFilter) matchKeywords(text string) ([]string, error) {
	searchText := text
	if !f.CaseSensitive {
		searchText = strings.ToLower(text)
	}

	matched := make([]string, 0)
	for _, keyword := range f.Keywords {
		searchKeyword := keyword
		if !f.CaseSensitive {
			searchKeyword = strings.ToLower(keyword)
		}

		switch f.MatchMode {
		case "contains":
			if strings.Contains(searchText, searchKeyword) {
				matched = append(matched, keyword)
			}
		case "exact":
			if searchText == searchKeyword {
				matched = append(matched, keyword)
			}
		case "regex":
			ok, err := regexp.MatchString(searchKeyword, searchText)
			if err != nil {
				return nil, err
			}
			if ok {
				matched = append(matched, keyword)
			}
		default:
			return nil, fmt.Errorf("unsupported keyword match mode: %s", f.MatchMode)
		}
	}
	return matched, nil
}

type LengthLimiter struct {
	Name            string
	MaxInputLength  int
	MaxOutputLength int
}

func NewLengthLimiter(maxInput, maxOutput int) *LengthLimiter {
	return &LengthLimiter{Name: "length_limiter", MaxInputLength: maxInput, MaxOutputLength: maxOutput}
}

func (l *LengthLimiter) GetName() string { return l.Name }

func (l *LengthLimiter) ValidateRequest(req any) (*types.GuardrailResult, error) {
	text, err := toJSONString(req)
	if err != nil {
		return nil, err
	}
	if l.MaxInputLength > 0 && utf8.RuneCountInString(text) > l.MaxInputLength {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "Request too long",
			Reason:  fmt.Sprintf("request length %d exceeds maximum %d", utf8.RuneCountInString(text), l.MaxInputLength),
			Actions: []string{"deny"},
		}, nil
	}
	return &types.GuardrailResult{Passed: true}, nil
}

func (l *LengthLimiter) ValidateResponse(resp any) (*types.GuardrailResult, error) {
	text, err := toJSONString(resp)
	if err != nil {
		return nil, err
	}
	if l.MaxOutputLength > 0 && utf8.RuneCountInString(text) > l.MaxOutputLength {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "Response too long",
			Reason:  fmt.Sprintf("response length %d exceeds maximum %d", utf8.RuneCountInString(text), l.MaxOutputLength),
			Actions: []string{"deny"},
		}, nil
	}
	return &types.GuardrailResult{Passed: true}, nil
}

func toJSONString(v any) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case []byte:
		return string(val), nil
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

func stringParam(params map[string]any, key, fallback string) string {
	if params == nil {
		return fallback
	}
	if value, ok := params[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func boolParam(params map[string]any, key string, fallback bool) bool {
	if params == nil {
		return fallback
	}
	if value, ok := params[key].(bool); ok {
		return value
	}
	return fallback
}

func intParam(params map[string]any, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	switch value := params[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func stringSliceParam(params map[string]any, key string) []string {
	if params == nil {
		return nil
	}
	switch values := params[key].(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, item := range values {
			if value, ok := item.(string); ok {
				out = append(out, value)
			}
		}
		return out
	case string:
		if values == "" {
			return nil
		}
		parts := strings.Split(values, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	default:
		return nil
	}
}
