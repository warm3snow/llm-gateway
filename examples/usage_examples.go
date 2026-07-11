package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 示例：如何使用 LLM Gateway

const GatewayBaseURL = "http://localhost:8080/v1"

// ChatCompletionExample 聊天补全示例
func ChatCompletionExample() {
	// 构造请求
	requestBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello! Can you tell me a joke?"},
		},
		"temperature": 0.7,
		"max_tokens":  100,
		"stream":      false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", GatewayBaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-provider", "openai")
	req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		return
	}

	// 打印结果
	fmt.Printf("Response: %+v\n", response)
}

// StreamChatExample 流式聊天示例
func StreamChatExample() {
	requestBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": "Count from 1 to 5 slowly."},
		},
		"stream": true,
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", GatewayBaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-provider", "openai")
	req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 处理 SSE 流
	buffer := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			fmt.Print(string(buffer[:n]))
		}
		if err != nil {
			break
		}
	}
}

// EmbeddingExample 嵌入示例
func EmbeddingExample() {
	requestBody := map[string]interface{}{
		"model": "text-embedding-ada-002",
		"input": "The quick brown fox jumped over the lazy dog",
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", GatewayBaseURL+"/embeddings", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-provider", "openai")
	req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Embedding response: %s\n", body)
}

// ModelsExample 获取模型列表示例
func ModelsExample() {
	req, _ := http.NewRequest("GET", GatewayBaseURL+"/models", nil)
	req.Header.Set("x-llm-provider", "openai")
	req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Models: %s\n", body)
}

// AnthropicExample 使用 Anthropic Claude 示例
func AnthropicExample() {
	requestBody := map[string]interface{}{
		"model": "claude-3-opus-20240229",
		"messages": []map[string]string{
			{"role": "user", "content": "Explain quantum computing in simple terms."},
		},
		"max_tokens": 500,
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", GatewayBaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-provider", "anthropic")
	req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Claude response: %s\n", body)
}

// OllamaExample 使用本地 Ollama 示例
func OllamaExample() {
	requestBody := map[string]interface{}{
		"model": "llama2",
		"messages": []map[string]string{
			{"role": "user", "content": "What is Go programming language?"},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", GatewayBaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))
	req.Header.Set("x-llm-provider", "ollama")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Ollama response: %s\n", body)
}

// LoadBalanceExample 负载均衡示例
func LoadBalanceExample() {
	// 通过配置实现负载均衡（需要在 config.yaml 中配置）
	// 这里演示如何发送请求到配置了负载均衡的网关

	requestBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": "Load balance test"},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	// 多次请求以观察负载均衡效果
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("POST", GatewayBaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-llm-gateway-api-key", os.Getenv("LLM_GATEWAY_API_KEY"))

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request %d failed: %v\n", i+1, err)
			continue
		}

		fmt.Printf("Request %d: Status %s\n", i+1, resp.Status)
		resp.Body.Close()

		time.Sleep(1 * time.Second)
	}
}

// Main 主函数
func main() {
	// 检查网关是否运行
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Println("Error: LLM Gateway is not running!")
		fmt.Println("Please start the gateway first: go run cmd/server/main.go")
		os.Exit(1)
	}
	resp.Body.Close()

	fmt.Println("=== LLM Gateway Usage Examples ===")

	// 运行示例
	fmt.Println("1. Chat Completion Example:")
	ChatCompletionExample()

	fmt.Println("\n2. Embedding Example:")
	EmbeddingExample()

	fmt.Println("\n3. Models Example:")
	ModelsExample()

	// 注意：以下示例需要相应的 API key
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		fmt.Println("\n4. Anthropic Claude Example:")
		AnthropicExample()
	}

	// 检查 Ollama 是否运行
	_, err = http.Get("http://localhost:11434")
	if err == nil {
		fmt.Println("\n5. Ollama Local Example:")
		OllamaExample()
	}

	fmt.Println("\n=== Examples Complete ===")
}
