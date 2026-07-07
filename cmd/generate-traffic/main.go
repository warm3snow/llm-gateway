package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	seedmanifest "github.com/warm3snow/llm-gateway/internal/seed"
)

var prompts = []string{
	"Say hello in one short sentence.",
	"Explain API gateways in one sentence.",
	"Write a short haiku about software.",
	"Summarize this phrase: reliable systems need observability.",
	"What is 2+2? Answer briefly.",
	"Translate 'hello' to French.",
}

type options struct {
	Profile      string
	ManifestPath string
	BaseURL      string
	Provider     string
	Model        string
	Requests     int
	Concurrency  int
	MaxTokens    int
	Timeout      time.Duration
}

type result struct {
	Status int
	Err    error
}

func main() {
	opts := options{}
	flag.StringVar(&opts.Profile, "profile", "demo", "seed profile used to resolve the default manifest path")
	flag.StringVar(&opts.ManifestPath, "manifest", "", "seed manifest path")
	flag.StringVar(&opts.BaseURL, "base-url", "", "gateway base URL override")
	flag.StringVar(&opts.Provider, "provider", "", "provider name to route through")
	flag.StringVar(&opts.Model, "model", "", "chat model to use; auto-detected from /v1/models when empty")
	flag.IntVar(&opts.Requests, "requests", 20, "number of chat completion requests to send")
	flag.IntVar(&opts.Concurrency, "concurrency", 2, "number of concurrent workers")
	flag.IntVar(&opts.MaxTokens, "max-tokens", 20, "max_tokens for each request")
	flag.DurationVar(&opts.Timeout, "timeout", 60*time.Second, "HTTP client timeout")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.Requests <= 0 {
		return fmt.Errorf("requests must be > 0")
	}
	if opts.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be > 0")
	}
	if opts.MaxTokens <= 0 {
		return fmt.Errorf("max-tokens must be > 0")
	}
	if opts.ManifestPath == "" {
		opts.ManifestPath = seedmanifest.DefaultManifestPath(opts.Profile)
	}

	manifest, err := seedmanifest.LoadManifest(opts.ManifestPath)
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(firstNonEmpty(opts.BaseURL, manifest.BaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("base URL is empty; set --base-url or rerun seed with --base-url")
	}
	provider := firstNonEmpty(opts.Provider, firstProvider(manifest))
	if provider == "" {
		return fmt.Errorf("provider is empty; set --provider")
	}
	keys := keysForProvider(manifest, provider)
	if len(keys) == 0 {
		return fmt.Errorf("no virtual keys in manifest allow provider %q", provider)
	}

	client := &http.Client{Timeout: opts.Timeout}
	model := opts.Model
	if model == "" {
		model, err = detectModel(context.Background(), client, baseURL, provider, keys[0].Key)
		if err != nil {
			return err
		}
	}

	var ok atomic.Int64
	var failed atomic.Int64
	jobs := make(chan int)
	results := make(chan result, opts.Requests)
	var wg sync.WaitGroup
	for worker := 0; worker < opts.Concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for n := range jobs {
				key := keys[n%len(keys)].Key
				res := sendChat(context.Background(), client, baseURL, provider, key, model, prompts[n%len(prompts)], opts.MaxTokens)
				if res.Err != nil || res.Status < 200 || res.Status >= 300 {
					failed.Add(1)
				} else {
					ok.Add(1)
				}
				results <- res
			}
		}(worker)
	}
	for n := 0; n < opts.Requests; n++ {
		jobs <- n
	}
	close(jobs)
	wg.Wait()
	close(results)

	var sampleErr error
	for res := range results {
		if sampleErr == nil && (res.Err != nil || res.Status < 200 || res.Status >= 300) {
			if res.Err != nil {
				sampleErr = res.Err
			} else {
				sampleErr = fmt.Errorf("status=%d", res.Status)
			}
		}
	}

	log.Printf("generated traffic provider=%s model=%s requests=%d success=%d failed=%d", provider, model, opts.Requests, ok.Load(), failed.Load())
	if ok.Load() == 0 {
		return fmt.Errorf("all traffic requests failed: %w", sampleErr)
	}
	if failed.Load() > 0 {
		return fmt.Errorf("%d/%d traffic requests failed; sample: %w", failed.Load(), opts.Requests, sampleErr)
	}
	return nil
}

func detectModel(ctx context.Context, client *http.Client, baseURL, provider, key string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-llm-gateway-api-key", key)
	req.Header.Set("x-llm-provider", provider)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET /v1/models: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET /v1/models returned status=%d body=%s", resp.StatusCode, truncate(string(body), 200))
	}
	model := firstChatModel(body)
	if model == "" {
		return "", errors.New("could not detect a chat model from /v1/models; set --model")
	}
	return model, nil
}

func sendChat(ctx context.Context, client *http.Client, baseURL, provider, key, model, prompt string, maxTokens int) result {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return result{Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return result{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-gateway-api-key", key)
	req.Header.Set("x-llm-provider", provider)
	resp, err := client.Do(req)
	if err != nil {
		return result{Err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return result{Status: resp.StatusCode}
}

func firstChatModel(data []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	for _, key := range []string{"data", "models"} {
		if model := firstModelFromValue(payload[key]); model != "" {
			return model
		}
	}
	return ""
}

func firstModelFromValue(value interface{}) string {
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if model := modelName(item); model != "" && !looksLikeEmbeddingModel(model) {
				return model
			}
		}
	case map[string]interface{}:
		for _, item := range v {
			if model := modelName(item); model != "" && !looksLikeEmbeddingModel(model) {
				return model
			}
		}
	}
	return ""
}

func modelName(item interface{}) string {
	switch v := item.(type) {
	case string:
		return v
	case map[string]interface{}:
		for _, key := range []string{"id", "name", "model"} {
			if value, ok := v[key].(string); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

func looksLikeEmbeddingModel(model string) bool {
	name := strings.ToLower(model)
	for _, marker := range []string{"embed", "bge", "nomic", "mxbai", "e5", "minilm"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func keysForProvider(manifest *seedmanifest.Manifest, provider string) []seedmanifest.ManifestKey {
	keys := make([]seedmanifest.ManifestKey, 0, len(manifest.VirtualKeys))
	for _, key := range manifest.VirtualKeys {
		if len(key.Providers) == 0 || contains(key.Providers, provider) {
			keys = append(keys, key)
		}
	}
	return keys
}

func firstProvider(manifest *seedmanifest.Manifest) string {
	if len(manifest.Providers) > 0 {
		return manifest.Providers[0].Name
	}
	if len(manifest.VirtualKeys) > 0 && len(manifest.VirtualKeys[0].Providers) > 0 {
		return manifest.VirtualKeys[0].Providers[0]
	}
	return ""
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
