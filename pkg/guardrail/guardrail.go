package guardrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// Guardrail 安全护栏接口
type Guardrail interface {
	// GetName 获取护栏名称
	GetName() string

	// ValidateRequest 验证请求
	ValidateRequest(req interface{}) (*types.GuardrailResult, error)

	// ValidateResponse 验证响应
	ValidateResponse(resp interface{}) (*types.GuardrailResult, error)
}

// GuardrailResult 护栏验证结果
type GuardrailResult struct {
	Passed  bool     `json:"passed"`
	Message string   `json:"message,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Actions []string `json:"actions,omitempty"`
}

// GuardrailConfig 护栏配置
type GuardrailConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Enabled    bool                   `json:"enabled"`
	Parameters map[string]interface{} `json:"parameters"`
	Deny       bool                   `json:"deny"`
	OnFailure  string                 `json:"onFailure"`
}

// GuardrailManager 护栏管理器
type GuardrailManager struct {
	Guardrails []Guardrail       `json:"guardrails"`
	Configs    []GuardrailConfig `json:"configs"`
}

// NewGuardrailManager 创建护栏管理器
func NewGuardrailManager() *GuardrailManager {
	return &GuardrailManager{
		Guardrails: make([]Guardrail, 0),
		Configs:    make([]GuardrailConfig, 0),
	}
}

// RegisterGuardrail 注册护栏
func (m *GuardrailManager) RegisterGuardrail(g Guardrail) {
	m.Guardrails = append(m.Guardrails, g)
}

// AddConfig 添加配置
func (m *GuardrailManager) AddConfig(cfg GuardrailConfig) {
	m.Configs = append(m.Configs, cfg)
}

// ValidateRequest 验证请求
func (m *GuardrailManager) ValidateRequest(req interface{}) (*types.GuardrailResult, error) {
	for _, g := range m.Guardrails {
		result, err := g.ValidateRequest(req)
		if err != nil {
			return nil, err
		}

		if !result.Passed {
			return result, nil
		}
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// ValidateResponse 验证响应
func (m *GuardrailManager) ValidateResponse(resp interface{}) (*types.GuardrailResult, error) {
	for _, g := range m.Guardrails {
		result, err := g.ValidateResponse(resp)
		if err != nil {
			return nil, err
		}

		if !result.Passed {
			return result, nil
		}
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// PIIDetector PII 检测器
type PIIDetector struct {
	Name     string
	Patterns map[string]*regexp.Regexp
}

// NewPIIDetector 创建 PII 检测器
func NewPIIDetector() *PIIDetector {
	return &PIIDetector{
		Name: "pii-detector",
		Patterns: map[string]*regexp.Regexp{
			"email":     regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			"phone":     regexp.MustCompile(`(\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3,4}[-.\s]?\d{4}`),
			"id_card":   regexp.MustCompile(`\d{17}[\dXx]`),
			"passport":  regexp.MustCompile(`[EGDSMP]\d{8}`),
			"bank_card": regexp.MustCompile(`\d{16,19}`),
		},
	}
}

// GetName 获取名称
func (d *PIIDetector) GetName() string {
	return d.Name
}

// ValidateRequest 验证请求
func (d *PIIDetector) ValidateRequest(req interface{}) (*types.GuardrailResult, error) {
	// 将请求转换为字符串
	reqStr, err := toJSONString(req)
	if err != nil {
		return nil, err
	}

	// 检测 PII
	detected := d.detectPII(reqStr)

	if len(detected) > 0 {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "PII detected in request",
			Reason:  fmt.Sprintf("Detected PII types: %s", strings.Join(detected, ", ")),
			Actions: []string{"mask", "deny"},
		}, nil
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// ValidateResponse 验证响应
func (d *PIIDetector) ValidateResponse(resp interface{}) (*types.GuardrailResult, error) {
	respStr, err := toJSONString(resp)
	if err != nil {
		return nil, err
	}

	detected := d.detectPII(respStr)

	if len(detected) > 0 {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "PII detected in response",
			Reason:  fmt.Sprintf("Detected PII types: %s", strings.Join(detected, ", ")),
			Actions: []string{"mask", "deny"},
		}, nil
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// detectPII 检测 PII
func (d *PIIDetector) detectPII(text string) []string {
	detected := make([]string, 0)

	for piiType, pattern := range d.Patterns {
		if pattern.MatchString(text) {
			detected = append(detected, piiType)
		}
	}

	return detected
}

// KeywordFilter 关键词过滤器
type KeywordFilter struct {
	Name          string
	Keywords      []string
	MatchMode     string // contains, exact, regex
	CaseSensitive bool
}

// NewKeywordFilter 创建关键词过滤器
func NewKeywordFilter(keywords []string, matchMode string, caseSensitive bool) *KeywordFilter {
	return &KeywordFilter{
		Name:          "keyword-filter",
		Keywords:      keywords,
		MatchMode:     matchMode,
		CaseSensitive: caseSensitive,
	}
}

// GetName 获取名称
func (f *KeywordFilter) GetName() string {
	return f.Name
}

// ValidateRequest 验证请求
func (f *KeywordFilter) ValidateRequest(req interface{}) (*types.GuardrailResult, error) {
	reqStr, err := toJSONString(req)
	if err != nil {
		return nil, err
	}

	matched := f.matchKeywords(reqStr)

	if matched {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "Blocked keywords detected in request",
			Reason:  "Request contains blocked keywords",
			Actions: []string{"deny"},
		}, nil
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// ValidateResponse 验证响应
func (f *KeywordFilter) ValidateResponse(resp interface{}) (*types.GuardrailResult, error) {
	respStr, err := toJSONString(resp)
	if err != nil {
		return nil, err
	}

	matched := f.matchKeywords(respStr)

	if matched {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "Blocked keywords detected in response",
			Reason:  "Response contains blocked keywords",
			Actions: []string{"deny", "replace"},
		}, nil
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// matchKeywords 匹配关键词
func (f *KeywordFilter) matchKeywords(text string) bool {
	searchText := text
	if !f.CaseSensitive {
		searchText = strings.ToLower(text)
	}

	for _, keyword := range f.Keywords {
		searchKeyword := keyword
		if !f.CaseSensitive {
			searchKeyword = strings.ToLower(keyword)
		}

		switch f.MatchMode {
		case "contains":
			if strings.Contains(searchText, searchKeyword) {
				return true
			}
		case "exact":
			if searchText == searchKeyword {
				return true
			}
		case "regex":
			if matched, _ := regexp.MatchString(searchKeyword, searchText); matched {
				return true
			}
		}
	}

	return false
}

// LengthLimiter 长度限制器
type LengthLimiter struct {
	Name            string
	MaxInputLength  int
	MaxOutputLength int
}

// NewLengthLimiter 创建长度限制器
func NewLengthLimiter(maxInput, maxOutput int) *LengthLimiter {
	return &LengthLimiter{
		Name:            "length-limiter",
		MaxInputLength:  maxInput,
		MaxOutputLength: maxOutput,
	}
}

// GetName 获取名称
func (l *LengthLimiter) GetName() string {
	return l.Name
}

// ValidateRequest 验证请求
func (l *LengthLimiter) ValidateRequest(req interface{}) (*types.GuardrailResult, error) {
	reqStr, err := toJSONString(req)
	if err != nil {
		return nil, err
	}

	if len(reqStr) > l.MaxInputLength {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "Request too long",
			Reason:  fmt.Sprintf("Request length %d exceeds maximum %d", len(reqStr), l.MaxInputLength),
			Actions: []string{"truncate", "deny"},
		}, nil
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// ValidateResponse 验证响应
func (l *LengthLimiter) ValidateResponse(resp interface{}) (*types.GuardrailResult, error) {
	respStr, err := toJSONString(resp)
	if err != nil {
		return nil, err
	}

	if len(respStr) > l.MaxOutputLength {
		return &types.GuardrailResult{
			Passed:  false,
			Message: "Response too long",
			Reason:  fmt.Sprintf("Response length %d exceeds maximum %d", len(respStr), l.MaxOutputLength),
			Actions: []string{"truncate", "deny"},
		}, nil
	}

	return &types.GuardrailResult{Passed: true}, nil
}

// toJSONString 转换为 JSON 字符串
func toJSONString(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

// 注册默认护栏
func init() {
	// 可以在这里注册全局默认的护栏
}
