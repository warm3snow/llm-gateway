package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/retry"
)

// ProxyHandler 代理处理器
type ProxyHandler struct {
	Config          *config.Config
	ProviderFactory *provider.ProviderFactory
	Retryer         *retry.Retryer
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler(cfg *config.Config) *ProxyHandler {
	return &ProxyHandler{
		Config:          cfg,
		ProviderFactory: provider.NewProviderFactory(),
		Retryer:         retry.NewRetryer(retry.DefaultRetryConfig()),
	}
}

// HandleChatCompletion 处理聊天补全请求
func (h *ProxyHandler) HandleChatCompletion(c *gin.Context) {
	var req types.ChatCompletionRequest

	// 解析请求
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}

	// 获取配置
	opts := h.getOptionsFromContext(c)

	// 创建 provider
	prov, err := provider.CreateProvider(opts.Provider, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Type:    "provider_error",
		})
		return
	}

	// 发送请求
	ctx := c.Request.Context()
	var resp *http.Response
	var respErr error

	if opts.Retry != nil {
		// 使用重试
		resp, respErr = h.Retryer.Do(ctx, func() (*http.Response, error) {
			return prov.ChatCompletion(ctx, &req, opts)
		})
	} else {
		resp, respErr = prov.ChatCompletion(ctx, &req, opts)
	}

	if respErr != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Request failed: %v", respErr),
			Type:    "request_error",
		})
		return
	}
	defer resp.Body.Close()

	// 处理响应
	h.handleResponse(c, resp)
}

// HandleCompletion 处理文本补全请求
func (h *ProxyHandler) HandleCompletion(c *gin.Context) {
	var req types.CompletionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}

	opts := h.getOptionsFromContext(c)
	prov, err := provider.CreateProvider(opts.Provider, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Type:    "provider_error",
		})
		return
	}

	ctx := c.Request.Context()
	resp, err := prov.Completion(ctx, &req, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Request failed: %v", err),
			Type:    "request_error",
		})
		return
	}
	defer resp.Body.Close()

	h.handleResponse(c, resp)
}

// HandleEmbedding 处理嵌入请求
func (h *ProxyHandler) HandleEmbedding(c *gin.Context) {
	var req types.EmbeddingRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}

	opts := h.getOptionsFromContext(c)
	prov, err := provider.CreateProvider(opts.Provider, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Type:    "provider_error",
		})
		return
	}

	ctx := c.Request.Context()
	resp, err := prov.Embedding(ctx, &req, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Request failed: %v", err),
			Type:    "request_error",
		})
		return
	}
	defer resp.Body.Close()

	h.handleResponse(c, resp)
}

// HandleModels 处理模型列表请求
func (h *ProxyHandler) HandleModels(c *gin.Context) {
	opts := h.getOptionsFromContext(c)
	prov, err := provider.CreateProvider(opts.Provider, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Type:    "provider_error",
		})
		return
	}

	ctx := c.Request.Context()
	resp, err := prov.Models(ctx, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Request failed: %v", err),
			Type:    "request_error",
		})
		return
	}
	defer resp.Body.Close()

	h.handleResponse(c, resp)
}

// handleResponse 处理响应
func (h *ProxyHandler) handleResponse(c *gin.Context, resp *http.Response) {
	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to read response: %v", err),
			Type:    "response_error",
		})
		return
	}

	// 设置响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 设置状态码
	c.Status(resp.StatusCode)

	// 写入响应体
	c.Writer.Write(body)
}

// getOptionsFromContext 从请求上下文获取选项
func (h *ProxyHandler) getOptionsFromContext(c *gin.Context) *types.Options {
	opts := &types.Options{
		Provider:       h.Config.Gateway.DefaultProvider,
		RequestTimeout: h.Config.Gateway.MaxRequestTimeout,
	}

	// 从请求头获取配置
	if provider := c.GetHeader("x-llm-provider"); provider != "" {
		opts.Provider = provider
	}

	if apiKey := c.GetHeader("x-llm-api-key"); apiKey != "" {
		opts.APIKey = apiKey
	}

	if virtualKey := c.GetHeader("x-llm-virtual-key"); virtualKey != "" {
		opts.VirtualKey = virtualKey
	}

	// 从配置中获取 provider 配置
	if providerConfig, ok := h.Config.Gateway.Providers[opts.Provider]; ok {
		// 合并配置
		if opts.APIKey == "" && providerConfig.APIKey != "" {
			opts.APIKey = providerConfig.APIKey
		}
		if opts.VirtualKey == "" && providerConfig.VirtualKey != "" {
			opts.VirtualKey = providerConfig.VirtualKey
		}
		opts.CustomHost = providerConfig.CustomHost
		opts.ForwardHeaders = providerConfig.ForwardHeaders
	}

	return opts
}

// ProxyRequest 代理请求
func (h *ProxyHandler) ProxyRequest(c *gin.Context) {
	// 获取目标 URL
	targetURL := h.getTargetURL(c)
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Target URL not found",
			Type:    "invalid_request_error",
		})
		return
	}

	// 读取请求体
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to read request body: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}

	// 创建新的请求
	ctx := c.Request.Context()
	req, err := http.NewRequestWithContext(ctx, c.Request.Method, targetURL, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create request: %v", err),
			Type:    "request_error",
		})
		return
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 发送请求
	client := &http.Client{
		Timeout: time.Duration(h.Config.Gateway.MaxRequestTimeout) * time.Millisecond,
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Request failed: %v", err),
			Type:    "request_error",
		})
		return
	}
	defer resp.Body.Close()

	// 处理响应
	h.handleResponse(c, resp)
}

// getTargetURL 获取目标 URL
func (h *ProxyHandler) getTargetURL(c *gin.Context) string {
	opts := h.getOptionsFromContext(c)

	// 如果有自定义 URL，使用它
	if opts.URLToFetch != "" {
		return opts.URLToFetch
	}

	// 根据 provider 构建 URL
	prov, err := provider.CreateProvider(opts.Provider, opts)
	if err != nil {
		return ""
	}

	baseURL := prov.GetBaseURL()
	path := c.Param("path")

	return baseURL + "/" + path
}

// HandleStreamRequest 处理流式请求
func (h *ProxyHandler) HandleStreamRequest(c *gin.Context) {
	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 获取 provider
	opts := h.getOptionsFromContext(c)
	prov, err := provider.CreateProvider(opts.Provider, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create provider: %v", err),
			Type:    "provider_error",
		})
		return
	}

	// 解析请求
	var req types.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}

	// 强制设置 stream 为 true
	req.Stream = true

	// 发送请求
	ctx := c.Request.Context()
	resp, err := prov.ChatCompletion(ctx, &req, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Request failed: %v", err),
			Type:    "request_error",
		})
		return
	}
	defer resp.Body.Close()

	// 流式传输响应
	h.streamResponse(c, resp)
}

// streamResponse 流式传输响应
func (h *ProxyHandler) streamResponse(c *gin.Context, resp *http.Response) {
	// 创建 flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Streaming not supported",
			Type:    "stream_error",
		})
		return
	}

	// 读取并转发流式数据
	buffer := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			// 写入数据
			_, writeErr := c.Writer.Write(buffer[:n])
			if writeErr != nil {
				return
			}
			flusher.Flush()
		}

		if err != nil {
			break
		}
	}
}

// RegisterRoutes 注册路由
func (h *ProxyHandler) RegisterRoutes(router *gin.Engine) {
	// 聊天补全
	router.POST("/v1/chat/completions", h.HandleChatCompletion)

	// 文本补全
	router.POST("/v1/completions", h.HandleCompletion)

	// 嵌入
	router.POST("/v1/embeddings", h.HandleEmbedding)

	// 模型列表
	router.GET("/v1/models", h.HandleModels)

	// 图像生成
	router.POST("/v1/images/generations", h.HandleImageGeneration)

	// 音频
	router.POST("/v1/audio/speech", h.HandleAudioSpeech)
	router.POST("/v1/audio/transcriptions", h.HandleAudioTranscription)
	router.POST("/v1/audio/translations", h.HandleAudioTranslation)

	// 代理
	router.Any("/v1/*path", h.ProxyRequest)

	// 流式请求
	router.POST("/v1/chat/completions/stream", h.HandleStreamRequest)
}

// HandleImageGeneration 处理图像生成请求
func (h *ProxyHandler) HandleImageGeneration(c *gin.Context) {
	// 实现图像生成逻辑
	c.JSON(http.StatusNotImplemented, types.ErrorResponse{
		Message: "Image generation not implemented yet",
		Type:    "not_implemented",
	})
}

// HandleAudioSpeech 处理文本转语音请求
func (h *ProxyHandler) HandleAudioSpeech(c *gin.Context) {
	// 实现文本转语音逻辑
	c.JSON(http.StatusNotImplemented, types.ErrorResponse{
		Message: "Audio speech not implemented yet",
		Type:    "not_implemented",
	})
}

// HandleAudioTranscription 处理语音转文本请求
func (h *ProxyHandler) HandleAudioTranscription(c *gin.Context) {
	// 实现语音转文本逻辑
	c.JSON(http.StatusNotImplemented, types.ErrorResponse{
		Message: "Audio transcription not implemented yet",
		Type:    "not_implemented",
	})
}

// HandleAudioTranslation 处理语音翻译请求
func (h *ProxyHandler) HandleAudioTranslation(c *gin.Context) {
	// 实现语音翻译逻辑
	c.JSON(http.StatusNotImplemented, types.ErrorResponse{
		Message: "Audio translation not implemented yet",
		Type:    "not_implemented",
	})
}
