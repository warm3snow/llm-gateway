package guardrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// Guardrail 安全护栏接口
type Guardrail interface {
	// ValidateRequest 验证请求
	ValidateRequest(request interface{}) (*types.GuardrailResult, error)

	// ValidateResponse 验证响应
	ValidateResponse(response interface{}) (*types.GuardrailResult, error)

	// GetName 获取护栏名称
	GetName() string
}

// PIIType PII 类型
type PIIType string

const (
	PIITypeEmail    PIIType = "email"
	PIITypePhone    PIIType = "phone"
	PIITypeIDCard   PIIType = "id_card"
	PIITypeBankCard PIIType = "bank_card"
	PIITypeAddress  PIIType = "address"
	PIITypeName     PIIType = "name"
)

// PIDetector PII 检测器
type PIDetector struct {
	enabled     bool
	piiTypes    []PIIType
	patterns    map[PIIType]*regexp.Regexp
	maskEnabled bool
	maskChar    string
}

// NewPIDetector 创建 PII 检测器
func NewPIDetector() *PIDetector {
	detector := &PIDetector{
		enabled:     true,
		piiTypes:    []PIIType{PIITypeEmail, PIITypePhone, PIITypeIDCard, PIITypeBankCard},
		patterns:    make(map[PIIType]*regexp.Regexp),
		maskEnabled: true,
		maskChar:    "*",
	}

	// 编译正则表达式
	detector.patterns[PIITypeEmail] = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	detector.patterns[PIITypePhone] = regexp.MustCompile(`(1[3-9]\d{9}|\+86[1-3-9]\d{9})`)
	detector.patterns[PIITypeIDCard] = regexp.MustCompile(`[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`)
	detector.patterns[PIITypeBankCard] = regexp.MustCompile(`(4[0-9]{12,15}|5[1-5][0-9]{14}|3[47][0-9]{13}|3[0,6,8][0-9]{12}|6(?:011|5[0-9][0-9])[0-9]{12})`)

	return detector
}

// ValidateRequest 验证请求
func (d *PIDetector) ValidateRequest(request interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed:  true,
		Message: "",
		Reason:  "",
		Actions: []string{},
	}

	if !d.enabled {
		return result, nil
	}

	// 将请求转换为字符串
	requestStr := d.convertToString(request)

	// 检测 PII
	detectedPII := d.detectPII(requestStr)

	if len(detectedPII) > 0 {
		result.Passed = false
		result.Reason = fmt.Sprintf("Detected PII: %s", strings.Join(detectedPII, ", "))
		result.Actions = append(result.Actions, "deny")

		if d.maskEnabled {
			result.Actions = append(result.Actions, "mask")
			result.MaskedContent = d.maskContent(requestStr, detectedPII)
		}
	}

	return result, nil
}

// ValidateResponse 验证响应
func (d *PIDetector) ValidateResponse(response interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed:  true,
		Message: "",
		Reason:  "",
		Actions: []string{},
	}

	if !d.enabled {
		return result, nil
	}

	// 将响应转换为字符串
	responseStr := d.convertToString(response)

	// 检测 PII
	detectedPII := d.detectPII(responseStr)

	if len(detectedPII) > 0 {
		result.Passed = false
		result.Reason = fmt.Sprintf("Detected PII in response: %s", strings.Join(detectedPII, ", "))
		result.Actions = append(result.Actions, "deny")

		if d.maskEnabled {
			result.Actions = append(result.Actions, "mask")
			result.MaskedContent = d.maskContent(responseStr, detectedPII)
		}
	}

	return result, nil
}

// GetName 获取名称
func (d *PIDetector) GetName() string {
	return "pii_detector"
}

// detectPII 检测 PII
func (d *PIDetector) detectPII(content string) []string {
	detected := []string{}

	for _, piiType := range d.piiTypes {
		pattern, ok := d.patterns[piiType]
		if !ok {
			continue
		}

		if pattern.MatchString(content) {
			detected = append(detected, string(piiType))
		}
	}

	return detected
}

// maskContent 脱敏内容
func (d *PIDetector) maskContent(content string, detectedPII []string) string {
	masked := content

	for _, piiType := range detectedPII {
		pattern, ok := d.patterns[PIIType(piiType)]
		if !ok {
			continue
		}

		// 替换匹配的内容
		masked = pattern.ReplaceAllStringFunc(masked, func(match string) string {
			runes := []rune{}
			for i := 0; i < utf8.RuneCountInString(match); i++ {
				runes = append(runes, []rune(d.maskChar)[0])
			}
			return string(runes)
		})
	}

	return masked
}

// convertToString 转换为字符串
func (d *PIDetector) convertToString(data interface{}) string {
	switch v := data.(type) {
	case string:
		return v
	case map[string]interface{}:
		result := ""
		for _, value := range v {
			result += d.convertToString(value) + " "
		}
		return strings.TrimSpace(result)
	default:
		return fmt.Sprintf("%v", data)
	}
}

// KeywordFilter 关键词过滤器
type KeywordFilter struct {
	enabled       bool
	keywords      []string
	matchMode     string // contains, exact, regex
	caseSensitive bool
}

// NewKeywordFilter 创建关键词过滤器
func NewKeywordFilter(keywords []string, matchMode string, caseSensitive bool) *KeywordFilter {
	return &KeywordFilter{
		enabled:       true,
		keywords:      keywords,
		matchMode:     matchMode,
		caseSensitive: caseSensitive,
	}
}

// ValidateRequest 验证请求
func (f *KeywordFilter) ValidateRequest(request interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed:  true,
		Message: "",
		Reason:  "",
		Actions: []string{},
	}

	if !f.enabled {
		return result, nil
	}

	// 将请求转换为字符串
	requestStr := ""
	switch v := request.(type) {
	case string:
		requestStr = v
	case map[string]interface{}:
		// 提取消息内容
		if messages, ok := v["messages"].([]interface{}); ok {
			for _, msg := range messages {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					if content, ok := msgMap["content"].(string); ok {
						requestStr += content + " "
					}
				}
			}
		}
	}

	if !f.caseSensitive {
		requestStr = strings.ToLower(requestStr)
	}

	// 检查关键词
	matchedKeywords := []string{}
	for _, keyword := range f.keywords {
		if !f.caseSensitive {
			keyword = strings.ToLower(keyword)
		}

		matched := false
		switch f.matchMode {
		case "contains":
			matched = strings.Contains(requestStr, keyword)
		case "exact":
			matched = requestStr == keyword
		case "regex":
			matched = regexp.MustCompile(keyword).MatchString(requestStr)
		}

		if matched {
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}

	if len(matchedKeywords) > 0 {
		result.Passed = false
		result.Reason = fmt.Sprintf("Contains blocked keywords: %s", strings.Join(matchedKeywords, ", "))
		result.Actions = append(result.Actions, "deny")
	}

	return result, nil
}

// ValidateResponse 验证响应
func (f *KeywordFilter) ValidateResponse(response interface{}) (*types.GuardrailResult, error) {
	// 类似实现...
	result := &types.GuardrailResult{
		Passed: true,
	}
	return result, nil
}

// GetName 获取名称
func (f *KeywordFilter) GetName() string {
	return "keyword_filter"
}

// LengthLimiter 长度限制器
type LengthLimiter struct {
	enabled        bool
	maxRequestLen  int
	maxResponseLen int
}

// NewLengthLimiter 创建长度限制器
func NewLengthLimiter(maxRequestLen, maxResponseLen int) *LengthLimiter {
	return &LengthLimiter{
		enabled:        true,
		maxRequestLen:  maxRequestLen,
		maxResponseLen: maxResponseLen,
	}
}

// ValidateRequest 验证请求
func (l *LengthLimiter) ValidateRequest(request interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed:  true,
		Message: "",
		Reason:  "",
		Actions: []string{},
	}

	if !l.enabled {
		return result, nil
	}

	// 计算请求长度
	requestStr := ""
	switch v := request.(type) {
	case string:
		requestStr = v
	case map[string]interface{}:
		data, _ := json.Marshal(v)
		requestStr = string(data)
	}

	if utf8.RuneCountInString(requestStr) > l.maxRequestLen {
		result.Passed = false
		result.Reason = fmt.Sprintf("Request length exceeds limit: %d > %d", utf8.RuneCountInString(requestStr), l.maxRequestLen)
		result.Actions = append(result.Actions, "deny")
	}

	return result, nil
}

// ValidateResponse 验证响应
func (l *LengthLimiter) ValidateResponse(response interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed: true,
	}

	if !l.enabled {
		return result, nil
	}

	// 计算响应长度
	responseStr := ""
	switch v := response.(type) {
	case string:
		responseStr = v
	case map[string]interface{}:
		data, _ := json.Marshal(v)
		responseStr = string(data)
	}

	if utf8.RuneCountInString(responseStr) > l.maxResponseLen {
		result.Passed = false
		result.Reason = fmt.Sprintf("Response length exceeds limit: %d > %d", utf8.RuneCountInString(responseStr), l.maxResponseLen)
		result.Actions = append(result.Actions, "deny")
	}

	return result, nil
}

// GetName 获取名称
func (l *LengthLimiter) GetName() string {
	return "length_limiter"
}

// GuardrailManager 护栏管理器
type GuardrailManager struct {
	guardrails []Guardrail
}

// NewGuardrailManager 创建护栏管理器
func NewGuardrailManager() *GuardrailManager {
	return &GuardrailManager{
		guardrails: []Guardrail{},
	}
}

// RegisterGuardrail 注册护栏
func (m *GuardrailManager) RegisterGuardrail(guardrail Guardrail) {
	m.guardrails = append(m.guardrails, guardrail)
}

// ValidateRequest 验证请求（所有护栏）
func (m *GuardrailManager) ValidateRequest(request interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed:  true,
		Message: "",
		Reason:  "",
		Actions: []string{},
	}

	for _, guardrail := range m.guardrails {
		guardResult, err := guardrail.ValidateRequest(request)
		if err != nil {
			return nil, err
		}

		if !guardResult.Passed {
			result.Passed = false
			result.Reason += guardrail.GetName() + ": " + guardResult.Reason + "; "
			result.Actions = append(result.Actions, guardResult.Actions...)
		}
	}

	return result, nil
}

// ValidateResponse 验证响应（所有护栏）
func (m *GuardrailManager) ValidateResponse(response interface{}) (*types.GuardrailResult, error) {
	result := &types.GuardrailResult{
		Passed:  true,
		Message: "",
		Reason:  "",
		Actions: []string{},
	}

	for _, guardrail := range m.guardrails {
		guardResult, err := guardrail.ValidateResponse(response)
		if err != nil {
			return nil, err
		}

		if !guardResult.Passed {
			result.Passed = false
			result.Reason += guardrail.GetName() + ": " + guardResult.Reason + "; "
			result.Actions = append(result.Actions, guardResult.Actions...)
		}
	}

	return result, nil
}
