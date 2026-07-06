package types

import "time"

// Provider 定义 LLM 提供商类型
type Provider string

const (
	ProviderOpenAI      Provider = "openai"
	ProviderAnthropic   Provider = "anthropic"
	ProviderGoogle      Provider = "google"
	ProviderGemini      Provider = "gemini"
	ProviderAzureOpenAI Provider = "azure-openai"
	ProviderCohere      Provider = "cohere"
	ProviderMistral     Provider = "mistral-ai"
	ProviderTogetherAI  Provider = "together-ai"
	ProviderPerplexity  Provider = "perplexity-ai"
	ProviderOllama      Provider = "ollama"
	ProviderGroq        Provider = "groq"
	ProviderDeepSeek    Provider = "deepseek"
	ProviderGLM         Provider = "glm"
	ProviderKimi        Provider = "kimi"
	ProviderBedrock     Provider = "bedrock"
	ProviderReplicate   Provider = "replicate"
	ProviderHuggingFace Provider = "huggingface"
	ProviderVertexAI    Provider = "vertex-ai"
)

// StrategyMode 定义路由策略
type StrategyMode string

const (
	StrategyLoadBalance StrategyMode = "loadbalance"
	StrategyFallback    StrategyMode = "fallback"
	StrategySingle      StrategyMode = "single"
	StrategyConditional StrategyMode = "conditional"
)

// RetrySettings 重试配置
type RetrySettings struct {
	Attempts            int     `json:"attempts" yaml:"attempts"`
	OnStatusCodes       []int   `json:"onStatusCodes" yaml:"onStatusCodes"`
	UseRetryAfterHeader bool    `json:"useRetryAfterHeader" yaml:"useRetryAfterHeader"`
	BackoffMin          float64 `json:"backoffMin" yaml:"backoffMin"`
	BackoffMax          float64 `json:"backoffMax" yaml:"backoffMax"`
}

// CacheSettings 缓存配置
type CacheSettings struct {
	Mode    string        `json:"mode" yaml:"mode"`
	MaxAge  time.Duration `json:"maxAge" yaml:"maxAge"`
	Enabled bool          `json:"enabled" yaml:"enabled"`
}

// Options 提供商配置选项
type Options struct {
	Provider       string                 `json:"provider" yaml:"provider"`
	VirtualKey     string                 `json:"virtualKey,omitempty" yaml:"virtualKey,omitempty"`
	APIKey         string                 `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Weight         int                    `json:"weight,omitempty" yaml:"weight,omitempty"`
	Retry          *RetrySettings         `json:"retry,omitempty" yaml:"retry,omitempty"`
	OverrideParams map[string]interface{} `json:"overrideParams,omitempty" yaml:"overrideParams,omitempty"`
	URLToFetch     string                 `json:"urlToFetch,omitempty" yaml:"urlToFetch,omitempty"`

	// Azure specific
	ResourceName  string `json:"resourceName,omitempty" yaml:"resourceName,omitempty"`
	DeploymentID  string `json:"deploymentId,omitempty" yaml:"deploymentId,omitempty"`
	APIVersion    string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	AzureAuthMode string `json:"azureAuthMode,omitempty" yaml:"azureAuthMode,omitempty"`

	// AWS specific
	AWSSecretAccessKey string `json:"awsSecretAccessKey,omitempty" yaml:"awsSecretAccessKey,omitempty"`
	AWSAccessKeyID     string `json:"awsAccessKeyId,omitempty" yaml:"awsAccessKeyId,omitempty"`
	AWSRegion          string `json:"awsRegion,omitempty" yaml:"awsRegion,omitempty"`

	// Custom
	CustomHost     string            `json:"customHost,omitempty" yaml:"customHost,omitempty"`
	ForwardHeaders []string          `json:"forwardHeaders,omitempty" yaml:"forwardHeaders,omitempty"`
	RequestTimeout int               `json:"requestTimeout,omitempty" yaml:"requestTimeout,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Index int `json:"index,omitempty" yaml:"index,omitempty"`
}

// Strategy 路由策略
type Strategy struct {
	Mode          StrategyMode `json:"mode" yaml:"mode"`
	OnStatusCodes []int        `json:"onStatusCodes,omitempty" yaml:"onStatusCodes,omitempty"`
	Conditions    []Condition  `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Default       string       `json:"default,omitempty" yaml:"default,omitempty"`
}

// Condition 条件路由
type Condition struct {
	Query map[string]interface{} `json:"query" yaml:"query"`
	Then  string                 `json:"then" yaml:"then"`
}

// Config 网关配置
type Config struct {
	Mode           string            `json:"mode,omitempty" yaml:"mode,omitempty"`
	Strategy       Strategy          `json:"strategy" yaml:"strategy"`
	Options        []Options         `json:"options" yaml:"options"`
	Cache          *CacheSettings    `json:"cache,omitempty" yaml:"cache,omitempty"`
	Guardrails     []GuardrailConfig `json:"guardrails,omitempty" yaml:"guardrails,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	ForwardHeaders []string          `json:"forwardHeaders,omitempty" yaml:"forwardHeaders,omitempty"`
	RequestTimeout int               `json:"requestTimeout,omitempty" yaml:"requestTimeout,omitempty"`
}

// GuardrailConfig guardrail 配置
type GuardrailConfig struct {
	Type       string                 `json:"type" yaml:"type"`
	Parameters map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Deny       bool                   `json:"deny,omitempty" yaml:"deny,omitempty"`
	OnFailure  string                 `json:"onFailure,omitempty" yaml:"onFailure,omitempty"`
}

// Message 聊天消息
type Message struct {
	Role    string      `json:"role" binding:"required" example:"user"`
	Content interface{} `json:"content" binding:"required" example:"Hello, how are you?"`
	Name    string      `json:"name,omitempty" example:"John"`
}

// ChatCompletionRequest 聊天补全请求
type ChatCompletionRequest struct {
	Model            string                 `json:"model,omitempty" example:"gpt-3.5-turbo"`
	Messages         []Message              `json:"messages" binding:"required" example:"[{\"role\":\"user\",\"content\":\"Hello\"}]"`
	Temperature      float64                `json:"temperature,omitempty" example:"0.7"`
	TopP             float64                `json:"top_p,omitempty" example:"1.0"`
	N                int                    `json:"n,omitempty" example:"1"`
	Stream           bool                   `json:"stream,omitempty" example:"false"`
	Stop             []string               `json:"stop,omitempty" example:"\\n"`
	MaxTokens        int                    `json:"max_tokens,omitempty" example:"100"`
	PresencePenalty  float64                `json:"presence_penalty,omitempty" example:"0"`
	FrequencyPenalty float64                `json:"frequency_penalty,omitempty"`
	User             string                 `json:"user,omitempty"`
	Tools            []interface{}          `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"`
	ResponseFormat   map[string]interface{} `json:"response_format,omitempty"`
	StreamOptions    *StreamOptions         `json:"stream_options,omitempty"`
}

// StreamOptions controls streaming response behavior (OpenAI-compatible).
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// CompletionRequest 补全请求
type CompletionRequest struct {
	Model            string   `json:"model" binding:"required"`
	Prompt           string   `json:"prompt" binding:"required"`
	MaxTokens        int      `json:"max_tokens,omitempty"`
	Temperature      float64  `json:"temperature,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	N                int      `json:"n,omitempty"`
	Stream           bool     `json:"stream,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	User             string   `json:"user,omitempty"`
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Model string      `json:"model" binding:"required" example:"text-embedding-ada-002"`
	Input interface{} `json:"input" binding:"required"`
	User  string      `json:"user,omitempty" example:"user-123"`
}

// ResponseFormat 响应格式
type ResponseFormat struct {
	ObjectType string         `json:"objectType"`
	Data       interface{}    `json:"data"`
	Error      *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ProviderResponse 提供商响应
type ProviderResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Error      error
}

// ChatCompletionResponse 聊天补全响应
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 选择
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage 使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamResponse 流式响应
type StreamResponse struct {
	Data  []byte
	Error error
	Done  bool
}

// GuardrailResult 护栏检查结果
type GuardrailResult struct {
	Passed        bool     `json:"passed"`
	Message       string   `json:"message,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	Actions       []string `json:"actions,omitempty"`
	MaskedContent string   `json:"maskedContent,omitempty"`
}
