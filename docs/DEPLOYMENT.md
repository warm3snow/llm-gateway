# LLM Gateway 部署指南

本文档介绍如何部署 LLM Gateway。

## 目录

- [环境要求](#环境要求)
- [本地部署](#本地部署)
- [Docker 部署](#docker-部署)
- [Kubernetes 部署](#kubernetes-部署)
- [生产环境建议](#生产环境建议)
- [监控和日志](#监控和日志)

## 环境要求

### 最低要求

- Go 1.21+
- 2GB RAM
- 1 CPU 核心
- 100MB 磁盘空间

### 推荐配置（生产环境）

- Go 1.21+
- 4GB+ RAM
- 2+ CPU 核心
- 1GB+ 磁盘空间
- Redis（用于分布式缓存）

## 本地部署

### 1. 编译

```bash
# 克隆仓库
git clone https://github.com/warm3snow/llm-gateway.git
cd llm-gateway

# 下载依赖
go mod download

# 编译二进制文件
go build -o llm-gateway cmd/server/main.go
```

### 2. 配置

编辑 `configs/config.yaml`：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"  # release, debug, test

gateway:
  defaultProvider: "openai"
  guardrailsEnabled: true
  providers:
    openai:
      provider: "openai"
      apiKey: "${OPENAI_API_KEY}"
    ollama:
      provider: "ollama"
      customHost: "http://localhost:11434/v1"
```

### 3. 设置环境变量

```bash
# OpenAI
export OPENAI_API_KEY="sk-your-openai-key"

# Anthropic (可选)
export ANTHROPIC_API_KEY="sk-ant-your-anthropic-key"

# Google (可选)
export GOOGLE_API_KEY="your-google-key"
```

### 4. 运行

```bash
# 直接运行
./llm-gateway

# 或使用 Go 运行
go run cmd/server/main.go

# 后台运行
nohup ./llm-gateway > logs/gateway.log 2>&1 &
```

### 5. 验证

```bash
# 健康检查
curl http://localhost:8080/health

# 测试聊天补全
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-llm-provider: openai" \
  -H "x-llm-api-key: $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Docker 部署

### 1. 使用 Dockerfile

```bash
# 构建镜像
docker build -t llm-gateway:latest .

# 运行容器
docker run -d \
  --name llm-gateway \
  -p 8080:8080 \
  -e OPENAI_API_KEY="$OPENAI_API_KEY" \
  -e ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY" \
  -v $(pwd)/configs:/app/configs \
  llm-gateway:latest
```

### 2. 使用 Docker Compose

创建 `docker-compose.yml`（已提供），然后运行：

```bash
# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

### 3. Docker Compose 配置

`docker-compose.yml` 包含以下服务：

- **llm-gateway**: 主网关服务
- **redis**: 用于缓存（可选）
- **nginx**: 反向代理和 SSL 终止（可选）

## Kubernetes 部署

### 1. 创建 ConfigMap

```yaml
# k8s-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: llm-gateway-config
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      mode: "release"
    gateway:
      defaultProvider: "openai"
      guardrailsEnabled: true
```

### 2. 创建 Secret

```bash
# 从环境变量创建 secret
kubectl create secret generic llm-gateway-secrets \
  --from-literal=openai-api-key="$OPENAI_API_KEY" \
  --from-literal=anthropic-api-key="$ANTHROPIC_API_KEY"
```

### 3. 创建 Deployment

```yaml
# k8s-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llm-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: llm-gateway
  template:
    metadata:
      labels:
        app: llm-gateway
    spec:
      containers:
      - name: llm-gateway
        image: llm-gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: llm-gateway-secrets
              key: openai-api-key
        - name: LLM_GATEWAY_CONFIG_PATH
          value: "/app/configs/config.yaml"
        volumeMounts:
        - name: config
          mountPath: "/app/configs"
      volumes:
      - name: config
        configMap:
          name: llm-gateway-config
```

### 4. 创建 Service

```yaml
# k8s-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: llm-gateway
spec:
  selector:
    app: llm-gateway
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
```

### 5. 部署

```bash
# 应用配置
kubectl apply -f k8s-configmap.yaml
kubectl apply -f k8s-deployment.yaml
kubectl apply -f k8s-service.yaml

# 查看状态
kubectl get pods -l app=llm-gateway
kubectl get svc llm-gateway
```

## 生产环境建议

### 1. 安全

- **使用 HTTPS**: 在生产环境使用 Nginx 或 Traefik 作为反向代理，配置 SSL/TLS
- **API 密钥认证**: 启用网关认证（`security.apiKeyHeader`）
- **速率限制**: 启用速率限制防止滥用
- **CORS 配置**: 不要使用 `*` 作为 allowedOrigins，指定具体的域名

### 2. 性能

- **启用缓存**: 配置 Redis 缓存以减少 API 调用和成本
- **连接池**: 配置 HTTP 客户端连接池
- **超时设置**: 根据需求调整请求超时
- **负载均衡**: 在多个网关实例间进行负载均衡

### 3. 高可用

- **多副本**: 运行多个网关实例
- **健康检查**: 配置 Kubernetes liveness 和 readiness probe
- **优雅关闭**: 网关已支持优雅关闭
- **监控告警**: 设置监控和告警

### 4. 配置管理

- **环境变量**: 使用环境变量管理敏感信息
- **配置分离**: 为不同环境（dev/staging/prod）使用不同的配置文件
- **动态配置**: 支持热加载配置（开发中）

## 监控和日志

### 1. 日志

网关支持多种日志级别和格式：

```yaml
logging:
  level: "info"  # debug, info, warn, error
  format: "json"  # json, text
  outputPath: "stdout"  # stdout, /path/to/log/file.log
```

### 2. 指标

（开发中）将支持 Prometheus 指标导出：

- 请求计数
- 请求延迟
- 错误率
- 提供商可用性

### 3. 追踪

（开发中）将支持 OpenTelemetry 分布式追踪：

- 请求追踪 ID
- 跨服务追踪
- 性能分析

## 故障排查

### 网关无法启动

1. 检查配置文件格式是否正确
2. 检查端口是否被占用
3. 查看日志输出

### 请求失败

1. 检查 API 密钥是否正确
2. 检查网络连接
3. 查看提供商 API 状态
4. 检查请求格式是否符合 OpenAI API 规范

### 性能问题

1. 启用缓存
2. 增加实例数量
3. 优化提供商配置
4. 使用更高效的模型

## 支持

如有问题，请：

- 查看 [GitHub Issues](https://github.com/warm3snow/llm-gateway/issues)
- 加入 [Discord 社区](https://discord.gg/xxx)（待创建）
- 查看 [Portkey 文档](https://docs.portkey.ai)（原版网关文档）

## 许可证

MIT License
