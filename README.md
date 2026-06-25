# LLM Gateway

一个用 Go 语言重写的 AI Gateway，基于 [Portkey Gateway](https://github.com/Portkey-AI/gateway) 架构设计。

## 功能特性

### 核心功能

- **多提供商支持**: 支持 12+ LLM 提供商（OpenAI, Anthropic, Google, Azure, Ollama 等）
- **统一 API**: 提供 OpenAI 兼容的 API 接口
- **智能路由**: 支持负载均衡、故障转移、条件路由
- **重试机制**: 自动重试失败的请求，支持指数退避
- **流式响应**: 支持 Server-Sent Events (SSE) 流式传输
- **请求转换**: 自动在不同提供商格式之间转换请求和响应

### 可靠性功能

- **自动重试**: 针对 429/500/502/503/504 等状态码自动重试
- **故障转移**: 主提供商失败时自动切换到备用提供商
- **负载均衡**: 在多个 API 密钥或提供商之间分配请求
- **请求超时**: 可配置请求超时时间
- **限流保护**: 防止过度使用 API 配额

### 安全功能

- **API 密钥管理**: 安全的 API 密钥存储和转发
- **CORS 支持**: 跨域资源共享配置
- **请求认证**: 支持 API 密钥认证
- **速率限制**: 防止滥用和过度请求

## 快速开始

### 1. 安装

```bash
# 克隆仓库
git clone https://github.com/warm3snow/llm-gateway.git
cd llm-gateway

# 安装依赖
go mod download

# 编译
go build -o llm-gateway cmd/server/main.go
```

### 2. 配置

编辑 `configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"

gateway:
  defaultProvider: "openai"
  guardrailsEnabled: true
  maxRequestTimeout: 120000
  providers:
    openai:
      provider: "openai"
      apiKey: "${OPENAI_API_KEY}"
    anthropic:
      provider: "anthropic"
      apiKey: "${ANTHROPIC_API_KEY}"
    ollama:
      provider: "ollama"
      customHost: "http://localhost:11434/v1"
```

### 3. 运行

```bash
# 设置环境变量
export OPENAI_API_KEY="your-openai-api-key"
export ANTHROPIC_API_KEY="your-anthropic-api-key"

# 运行服务器
./llm-gateway

# 或使用 Go 直接运行
go run cmd/server/main.go
```

服务器将在 `http://localhost:8080` 启动。

### 4. 测试

```bash
# 健康检查
curl http://localhost:8080/health

# 聊天补全
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-llm-provider: openai" \
  -H "x-llm-api-key: your-api-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## API 文档

### 端点

#### 聊天补全
```
POST /v1/chat/completions
```

请求体:
```json
{
  "model": "gpt-3.5-turbo",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 100,
  "stream": false
}
```

#### 文本补全
```
POST /v1/completions
```

#### 嵌入
```
POST /v1/embeddings
```

#### 模型列表
```
GET /v1/models
```

### 请求头

- `x-llm-provider`: 指定 LLM 提供商（openai, anthropic, google 等）
- `x-llm-api-key`: API 密钥
- `x-llm-virtual-key`: 虚拟密钥
- `x-llm-cache`: 缓存控制
- `x-llm-request-timeout`: 请求超时（毫秒）

## 配置

### 支持的提供商

- **OpenAI**: GPT-3.5, GPT-4, DALL-E, Whisper
- **Anthropic**: Claude 3 系列
- **Google**: Gemini Pro, Gemini Ultra
- **Azure OpenAI**: Azure 托管的 OpenAI 模型
- **Cohere**: Command, Embed
- **Mistral AI**: Mistral-7B, Mixtral
- **Together AI**: 开源模型托管
- **Ollama**: 本地运行的开源模型
- **Groq**: 高速 LLM 推理
- **DeepSeek**: DeepSeek Chat
- **AWS Bedrock**: Claude, Titan, Jurassic
- **Replicate**: 开源模型

### 路由策略

#### 单提供商
```yaml
strategy:
  mode: "single"
  options:
    - provider: "openai"
      apiKey: "${OPENAI_API_KEY}"
```

#### 负载均衡
```yaml
strategy:
  mode: "loadbalance"
  options:
    - provider: "openai"
      weight: 70
    - provider: "anthropic"
      weight: 30
```

#### 故障转移
```yaml
strategy:
  mode: "fallback"
  options:
    - provider: "openai"
    - provider: "anthropic"
```

#### 条件路由
```yaml
strategy:
  mode: "conditional"
  conditions:
    - query:
        max_tokens: "<= 100"
      then: "openai"
    - query:
        max_tokens: "> 100"
      then: "anthropic"
```

## 架构

### 项目结构

```
llm-gateway/
├── cmd/
│   └── server/          # 主服务器入口
├── internal/
│   ├── config/          # 配置管理
│   ├── handler/         # HTTP 处理器
│   ├── middleware/      # 中间件
│   ├── provider/        # LLM 提供商实现
│   ├── service/         # 业务逻辑服务
│   └── types/          # 数据模型
├── pkg/
│   ├── proxy/           # 代理逻辑
│   ├── retry/           # 重试机制
│   └── guardrail/      # 安全护栏
├── configs/            # 配置文件
├── docs/               # 文档
└── tests/              # 测试
```

### 核心组件

1. **Provider Interface**: 统一的 LLM 提供商接口
2. **Proxy Handler**: 请求代理和转发
3. **Retry Mechanism**: 智能重试和退避
4. **Config Manager**: 动态配置管理
5. **Middleware Chain**: 日志、认证、CORS 等

## 部署

### Docker

```bash
# 构建镜像
docker build -t llm-gateway:latest .

# 运行容器
docker run -p 8080:8080 \
  -e OPENAI_API_KEY=your-key \
  -v $(pwd)/configs:/app/configs \
  llm-gateway:latest
```

### Docker Compose

```bash
docker-compose up -d
```

### Kubernetes

```bash
kubectl apply -f deployment.yaml
```

## 开发

### 运行测试

```bash
go test ./...
```

### 代码格式化

```bash
go fmt ./...
go vet ./...
```

### 添加新提供商

1. 在 `internal/provider/` 创建新 provider
2. 实现 `Provider` 接口
3. 在 `init()` 中注册 provider
4. 更新配置文件

示例:

```go
// internal/provider/myprovider/myprovider.go
package myprovider

import "github.com/warm3snow/llm-gateway/internal/provider"

func init() {
	provider.RegisterGlobalProvider("myprovider", NewMyProvider)
}

type MyProvider struct {
	*provider.BaseProvider
}

func NewMyProvider(opts *types.Options) (provider.Provider, error) {
	return &MyProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    "myprovider",
			BaseURL: "https://api.myprovider.com/v1",
		},
	}, nil
}

func (p *MyProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	// 实现聊天补全逻辑
}
```

## 性能

- **低延迟**: < 1ms 额外延迟（不包括 LLM 推理时间）
- **高并发**: 支持数千个并发请求
- **内存高效**: 最小化内存分配和 GC 压力
- **流式处理**: 支持大文件和流式响应

## 对比原版 (Portkey Gateway)

| 特性 | Portkey (TypeScript) | LLM Gateway (Go) |
|------|----------------------|-------------------|
| 语言 | TypeScript/Node.js | Go |
| 性能 | 中等 | 高 |
| 内存占用 | ~150MB | ~30MB |
| 启动时间 | ~2s | ~0.5s |
| 并发处理 | 好 | 优秀 |
| 部署复杂度 | 中等 | 低（单一二进制） |

## 许可证

MIT License

## 贡献

欢迎贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

## 致谢

本项目基于 [Portkey Gateway](https://github.com/Portkey-AI/gateway) 的架构设计，感谢 Portkey 团队的出色工作。

## 联系方式

- 问题反馈: [GitHub Issues](https://github.com/warm3snow/llm-gateway/issues)
- 讨论: [GitHub Discussions](https://github.com/warm3snow/llm-gateway/discussions)
