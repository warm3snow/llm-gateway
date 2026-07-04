package main

// 这个文件用于为 /v1/* 代理端点生成 Swagger 文档
// 因为这些端点的处理函数不在 main.go 中，所以需要单独定义

import "github.com/gin-gonic/gin"

// ProxyHandlers 是一个虚拟类型，用于承载 Swagger 注解
type ProxyHandlers struct{}

// HandleChatCompletion 聊天补全
// @Summary Chat completion
// @Description Create a chat completion (OpenAI-compatible)
// @Tags proxy
// @Accept json
// @Produce json
// @Param request body types.ChatCompletionRequest true "Chat completion request"
// @Success 200 {object} types.ChatCompletionResponse "Chat completion response"
// @Failure 400 {object} types.ErrorResponse "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/chat/completions [post]
func (p *ProxyHandlers) HandleChatCompletion(c *gin.Context) {}

// HandleCompletion 文本补全
// @Summary Text completion
// @Description Create a text completion (legacy)
// @Tags proxy
// @Accept json
// @Produce json
// @Param request body object true "Completion request"
// @Success 200 {object} object "Completion response"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/completions [post]
func (p *ProxyHandlers) HandleCompletion(c *gin.Context) {}

// HandleEmbedding 嵌入
// @Summary Create embeddings
// @Description Create embeddings for text
// @Tags proxy
// @Accept json
// @Produce json
// @Param request body types.EmbeddingRequest true "Embedding request"
// @Success 200 {object} object "Embedding response"
// @Failure 400 {object} types.ErrorResponse "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/embeddings [post]
func (p *ProxyHandlers) HandleEmbedding(c *gin.Context) {}

// HandleModels 模型列表
// @Summary List models
// @Description List available models
// @Tags proxy
// @Accept json
// @Produce json
// @Success 200 {object} object "Models list" example({"object":"list","data":[{{"id":"gpt-3.5-turbo","object":"model","created":1234567890,"owned_by":"openai","permission":[],"root":"gpt-3.5-turbo","parent":null}},{{"id":"gpt-4","object":"model","created":1234567890,"owned_by":"openai","permission":[],"root":"gpt-4","parent":null}}]})
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/models [get]
func (p *ProxyHandlers) HandleModels(c *gin.Context) {}

// HandleImageGeneration 图像生成
// @Summary Generate images
// @Description Generate images from text (DALL-E)
// @Tags proxy
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Image generation request"
// @Success 200 {object} object "Image generation response"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/images/generations [post]
func (p *ProxyHandlers) HandleImageGeneration(c *gin.Context) {}

// HandleAudioSpeech 文本转语音
// @Summary Text to speech
// @Description Convert text to speech
// @Tags proxy
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "TTS request"
// @Success 200 {object} object "Audio response"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/audio/speech [post]
func (p *ProxyHandlers) HandleAudioSpeech(c *gin.Context) {}

// HandleAudioTranscription 音频转写
// @Summary Audio transcription
// @Description Transcribe audio to text (Whisper)
// @Tags proxy
// @Accept mpfd
// @Produce json
// @Param file formData file true "Audio file"
// @Param model formData string false "Model name"
// @Success 200 {object} object "Transcription response"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/audio/transcriptions [post]
func (p *ProxyHandlers) HandleAudioTranscription(c *gin.Context) {}

// HandleAudioTranslation 音频翻译
// @Summary Audio translation
// @Description Translate audio to English (Whisper)
// @Tags proxy
// @Accept mpfd
// @Produce json
// @Param file formData file true "Audio file"
// @Param model formData string false "Model name"
// @Success 200 {object} object "Translation response"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/audio/translations [post]
func (p *ProxyHandlers) HandleAudioTranslation(c *gin.Context) {}

// ProxyRequest 代理请求
// @Summary Proxy request
// @Description Proxy any request to upstream provider
// @Tags proxy
// @Accept json
// @Produce json
// @Param path path string true "Upstream path"
// @Success 200 {object} object "Proxy response"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/proxy/{path} [post]
func (p *ProxyHandlers) ProxyRequest(c *gin.Context) {}

// HandleStreamRequest 流式聊天补全
// @Summary Stream chat completion
// @Description Create a streaming chat completion (SSE)
// @Tags proxy
// @Accept json
// @Produce json
// @Param request body object true "Chat completion request"
// @Success 200 {object} object "Streaming response"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security VirtualKeyAuth
// @Router /v1/chat/completions/stream [post]
func (p *ProxyHandlers) HandleStreamRequest(c *gin.Context) {}
