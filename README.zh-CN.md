# LLM Gateway

<p align="center">
  <a href="README.md">English</a> ·
  <strong>中文</strong> ·
  <a href="docs/api.md">API Reference</a> ·
  <a href="LICENSE">License</a>
</p>

LLM Gateway 是一个用 Go 构建的统一 LLM API 网关：用一套 OpenAI-compatible API 接入多个模型服务，并提供虚拟密钥、用量统计、缓存、路由和管理后台。

## 你可以用它做什么

- 用 `/v1/chat/completions` 等 OpenAI-compatible 接口统一调用不同 provider
- 给业务方发放虚拟密钥，并按 key 统计用量、预算和请求记录
- 在多个 provider / model 之间做负载均衡、故障转移和条件路由
- 开启内存或 Redis 缓存，降低重复请求成本
- 通过 Next.js 管理后台维护 provider、虚拟密钥、租户、用户、告警和统计报表

![alt text](assets/dashboard.png)

## 快速开始

### 1. 安装依赖

```bash
make deps
```

### 2. 配置 provider

默认配置文件在 `configs/config.yaml`。本地调试默认指向 Ollama：

```yaml
gateway:
  defaultProvider: ollama
  providers:
    ollama:
      provider: ollama
      customHost: http://localhost:11434/v1
```

如果使用云端 provider，按需替换 `provider`、`apiKey` 和 `customHost`。

### 3. 启动服务

```bash
make dev
```

- 后端 API：`http://localhost:8080`
- 前端控制台：`http://localhost:3000`
- 默认管理员：`admin / admin123`（生产环境请修改）

也可以后台启动：

```bash
make start
# 停止
make stop
```

### 4. 创建虚拟密钥并调用

登录控制台创建 Virtual Key 后，用它调用 OpenAI-compatible API：

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-llm-gateway-api-key: your-virtual-key" \
  -d '{
    "model": "llama3",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 架构设计图

<img width="2306" height="456" alt="Clipboard_Screenshot_1783826254" src="https://github.com/user-attachments/assets/a6e171b0-005c-4379-8c90-d64a3bdfa80f" />

### 请求链路

```text
Client
  -> /v1/*
  -> Virtual Key Auth
  -> Guardrail / Idempotency / Usage Record / Cache
  -> Proxy Handler
  -> Routing Engine
  -> Provider Adapter
  -> Upstream LLM API
```

## API 文档

- [API Reference](docs/api.md)（英文）
- OpenAI-compatible API 使用 `x-llm-gateway-api-key` 虚拟密钥认证
- Admin API 使用 `Authorization: Bearer <token>` JWT 认证

## 核心能力

| 能力 | 说明 |
|---|---|
| 多 Provider | 内置 OpenAI、Anthropic、Gemini、Azure、DeepSeek、Groq、Mistral、Kimi、GLM、Cohere、Ollama 等适配器 |
| 统一协议 | 对外提供 OpenAI-compatible API，减少业务侧适配成本 |
| 虚拟密钥 | 使用网关密钥隔离真实 provider key，支持预算和用量跟踪 |
| 路由策略 | 支持单 provider、负载均衡、故障转移、条件路由 |
| 缓存 | 支持内存缓存和 Redis 缓存 |
| 多租户 | 支持租户、用户、角色和租户级数据隔离 |
| 可观测性 | 提供请求日志、用量记录、统计分析和 Prometheus metrics |

## 常用命令

| 命令 | 说明 |
|---|---|
| `make deps` | 安装后端和前端依赖 |
| `make dev` | 前台启动后端和前端 |
| `make start` | 后台启动后端和前端 |
| `make stop` | 停止本地服务 |
| `make build` | 构建后端二进制和前端产物 |
| `make test` | 运行 Go 单元测试 |
| `make fmt` | 格式化 Go 代码 |
| `make seed-demo` | 生成演示数据 |
| `make traffic-demo` | 生成演示流量 |
| `make load-test` | 运行压测场景 |

## 项目结构

```text
llm-gateway/
├── cmd/server/              # 服务入口与路由注册
├── internal/
│   ├── config/              # 配置加载
│   ├── database/            # GORM 数据库层
│   ├── handler/             # HTTP handlers
│   ├── middleware/          # Gin middleware
│   ├── models/              # 数据模型
│   ├── provider/            # LLM provider adapters
│   ├── routing/             # 路由策略
│   └── service/             # 业务服务
├── pkg/
│   ├── cache/               # Memory / Redis cache
│   ├── guard-rail/          # 请求 / 响应护栏
│   ├── proxy/               # 核心代理逻辑
│   └── retry/               # 重试机制
├── web/frontend/            # Next.js 管理后台
├── configs/config.yaml      # 默认配置
└── Makefile                 # 本地开发命令
```

## 开源协作

欢迎提交 Issue 和 Pull Request。建议在提交前运行：

```bash
make test
make fmt
```

如果新增 provider、路由策略或管理接口，请同步更新相关文档和测试。

## 安全

- 不要提交真实 API Key、JWT Secret 或生产配置
- 生产环境请修改默认管理员密码和 `security.jwtSecret`
- 如果发现安全问题，请优先通过私有渠道联系维护者，避免公开披露可利用细节

## 许可证

MIT License. See [LICENSE](LICENSE).
