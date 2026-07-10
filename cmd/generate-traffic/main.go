package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	seedmanifest "github.com/warm3snow/llm-gateway/internal/seed"
)

const maxResponseDrainBytes int64 = 16 << 20

var prompts = []string{
	"Say hello in one short sentence.",
	"Explain API gateways in one sentence.",
	"Write a short haiku about software.",
	"Summarize this phrase: reliable systems need observability.",
	"What is 2+2? Answer briefly.",
	"Translate 'hello' to French.",
}

type options struct {
	Profile         string        `json:"profile"`
	ManifestPath    string        `json:"manifest_path"`
	BaseURL         string        `json:"base_url"`
	Provider        string        `json:"provider"`
	Model           string        `json:"model"`
	Requests        int           `json:"requests"`
	Concurrency     int           `json:"concurrency"`
	MaxTokens       int           `json:"max_tokens"`
	Timeout         time.Duration `json:"timeout"`
	Duration        time.Duration `json:"duration"`
	Warmup          time.Duration `json:"warmup"`
	RPS             float64       `json:"rps"`
	Scenario        string        `json:"scenario"`
	Stream          bool          `json:"stream"`
	PromptSize      string        `json:"prompt_size"`
	PromptFile      string        `json:"prompt_file"`
	KeyStrategy     string        `json:"key_strategy"`
	Output          string        `json:"output"`
	OutputFile      string        `json:"output_file"`
	MetricsURL      string        `json:"metrics_url"`
	MetricsInterval time.Duration `json:"metrics_interval"`
	FailThreshold   float64       `json:"fail_threshold"`
	P95Threshold    time.Duration `json:"p95_threshold"`
}

type result struct {
	Status        int           `json:"status"`
	Err           error         `json:"-"`
	Error         string        `json:"error,omitempty"`
	Latency       time.Duration `json:"latency"`
	TTFT          time.Duration `json:"ttft,omitempty"`
	RequestBytes  int           `json:"request_bytes"`
	ResponseBytes int64         `json:"response_bytes"`
	KeyName       string        `json:"key_name,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	EndedAt       time.Time     `json:"ended_at"`
}

type durationSummary struct {
	Min float64 `json:"min"`
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
}

type metricsSnapshot struct {
	Timestamp time.Time          `json:"timestamp"`
	Values    map[string]float64 `json:"values"`
	Error     string             `json:"error,omitempty"`
}

type summary struct {
	Scenario           string            `json:"scenario"`
	BaseURL            string            `json:"base_url"`
	Provider           string            `json:"provider"`
	Model              string            `json:"model"`
	StartedAt          time.Time         `json:"started_at"`
	EndedAt            time.Time         `json:"ended_at"`
	DurationSeconds    float64           `json:"duration_seconds"`
	WarmupSeconds      float64           `json:"warmup_seconds"`
	Requests           int               `json:"requests"`
	Success            int               `json:"success"`
	Failed             int               `json:"failed"`
	ErrorRate          float64           `json:"error_rate"`
	ThroughputRPS      float64           `json:"throughput_rps"`
	LatencyMS          durationSummary   `json:"latency_ms"`
	TTFTMS             *durationSummary  `json:"ttft_ms,omitempty"`
	StatusCodes        map[string]int    `json:"status_codes"`
	RequestBytesTotal  int64             `json:"request_bytes_total"`
	ResponseBytesTotal int64             `json:"response_bytes_total"`
	Options            options           `json:"options"`
	Metrics            []metricsSnapshot `json:"metrics,omitempty"`
}

func main() {
	opts := options{}
	flag.StringVar(&opts.Profile, "profile", "demo", "seed profile used to resolve the default manifest path")
	flag.StringVar(&opts.ManifestPath, "manifest", "", "seed manifest path")
	flag.StringVar(&opts.BaseURL, "base-url", "", "gateway base URL override")
	flag.StringVar(&opts.Provider, "provider", "", "provider name to route through")
	flag.StringVar(&opts.Model, "model", "", "chat model to use; auto-detected from /v1/models when empty")
	flag.IntVar(&opts.Requests, "requests", 20, "number of chat completion requests to send; ignored when --duration is set")
	flag.IntVar(&opts.Concurrency, "concurrency", 2, "number of concurrent workers")
	flag.IntVar(&opts.MaxTokens, "max-tokens", 20, "max_tokens for each request")
	flag.DurationVar(&opts.Timeout, "timeout", 60*time.Second, "HTTP client timeout")
	flag.DurationVar(&opts.Duration, "duration", 0, "fixed benchmark duration; when set, --requests is ignored")
	flag.DurationVar(&opts.Warmup, "warmup", 0, "warmup duration excluded from final stats")
	flag.Float64Var(&opts.RPS, "rps", 0, "target requests per second; 0 means run as fast as concurrency allows")
	flag.StringVar(&opts.Scenario, "scenario", "", "scenario preset: baseline-nonstream, streaming-ttft, large-prompt, cache-repeat, many-keys, soak")
	flag.BoolVar(&opts.Stream, "stream", false, "use /v1/chat/completions/stream and measure time to first streamed byte")
	flag.StringVar(&opts.PromptSize, "prompt-size", "short", "prompt size: short, medium, large")
	flag.StringVar(&opts.PromptFile, "prompt-file", "", "newline-delimited prompt corpus")
	flag.StringVar(&opts.KeyStrategy, "key-strategy", "round-robin", "virtual key strategy: single, round-robin, random")
	flag.StringVar(&opts.Output, "output", "text", "output format: text or json")
	flag.StringVar(&opts.OutputFile, "output-file", "", "write benchmark summary JSON to this path")
	flag.StringVar(&opts.MetricsURL, "metrics-url", "", "gateway /metrics URL to snapshot before, during, and after the run")
	flag.DurationVar(&opts.MetricsInterval, "metrics-interval", 0, "metrics snapshot interval during duration runs")
	flag.Float64Var(&opts.FailThreshold, "fail-threshold", 0, "maximum allowed failure percentage; 0 preserves legacy fail-on-any-error behavior")
	flag.DurationVar(&opts.P95Threshold, "p95-threshold", 0, "optional maximum allowed p95 latency")
	flag.Parse()

	applyScenarioDefaults(&opts)

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func applyScenarioDefaults(opts *options) {
	switch strings.TrimSpace(opts.Scenario) {
	case "baseline-nonstream":
		opts.Stream = false
		if opts.MaxTokens == 20 {
			opts.MaxTokens = 64
		}
	case "streaming-ttft":
		opts.Stream = true
		if opts.MaxTokens == 20 {
			opts.MaxTokens = 128
		}
	case "large-prompt":
		opts.PromptSize = "large"
		if opts.MaxTokens == 20 {
			opts.MaxTokens = 256
		}
	case "cache-repeat":
		opts.Stream = false
		if opts.MaxTokens == 20 {
			opts.MaxTokens = 64
		}
	case "many-keys":
		if opts.KeyStrategy == "" || opts.KeyStrategy == "round-robin" {
			opts.KeyStrategy = "random"
		}
		if opts.MaxTokens == 20 {
			opts.MaxTokens = 64
		}
	case "soak":
		if opts.Duration == 0 {
			opts.Duration = 30 * time.Minute
		}
		if opts.Warmup == 0 {
			opts.Warmup = 30 * time.Second
		}
		if opts.MaxTokens == 20 {
			opts.MaxTokens = 64
		}
	}
}

func validateOptions(opts options) error {
	if !contains([]string{"", "baseline-nonstream", "streaming-ttft", "large-prompt", "cache-repeat", "many-keys", "soak"}, opts.Scenario) {
		return fmt.Errorf("unknown scenario %q", opts.Scenario)
	}
	if !contains([]string{"text", "json"}, opts.Output) {
		return fmt.Errorf("unknown output %q", opts.Output)
	}
	if !contains([]string{"single", "round-robin", "random"}, opts.KeyStrategy) {
		return fmt.Errorf("unknown key-strategy %q", opts.KeyStrategy)
	}
	if !contains([]string{"short", "medium", "large"}, opts.PromptSize) {
		return fmt.Errorf("unknown prompt-size %q", opts.PromptSize)
	}
	return nil
}

func run(opts options) error {
	if opts.Duration <= 0 && opts.Requests <= 0 {
		return fmt.Errorf("requests must be > 0 when duration is not set")
	}
	if opts.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be > 0")
	}
	if opts.MaxTokens <= 0 {
		return fmt.Errorf("max-tokens must be > 0")
	}
	if opts.RPS < 0 {
		return fmt.Errorf("rps must be >= 0")
	}
	if opts.ManifestPath == "" {
		opts.ManifestPath = seedmanifest.DefaultManifestPath(opts.Profile)
	}
	if opts.Output == "" {
		opts.Output = "text"
	}
	if opts.KeyStrategy == "" {
		opts.KeyStrategy = "round-robin"
	}
	if opts.PromptSize == "" {
		opts.PromptSize = "short"
	}
	if err := validateOptions(opts); err != nil {
		return err
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
	promptSet, err := promptsForOptions(opts)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: opts.Timeout}
	model := opts.Model
	if model == "" {
		model, err = detectModel(context.Background(), client, baseURL, provider, keys[0].Key)
		if err != nil {
			return err
		}
	}

	log.Printf("benchmark scenario=%s provider=%s model=%s duration=%s requests=%d concurrency=%d rps=%.2f stream=%t prompt_size=%s max_tokens=%d key_strategy=%s", scenarioName(opts), provider, model, opts.Duration, opts.Requests, opts.Concurrency, opts.RPS, opts.Stream, opts.PromptSize, opts.MaxTokens, opts.KeyStrategy)

	started := time.Now()
	metricCtx, cancelMetrics := context.WithCancel(context.Background())
	defer cancelMetrics()
	metricsCh := startMetricsSnapshots(metricCtx, client, opts.MetricsURL, opts.MetricsInterval)

	results := executeLoad(opts, client, baseURL, provider, model, keys, promptSet)
	ended := time.Now()
	cancelMetrics()
	metricSnapshots := <-metricsCh

	s := buildSummary(opts, baseURL, provider, model, started, ended, results, metricSnapshots)
	if err := emitSummary(s, opts); err != nil {
		return err
	}
	if s.Requests == 0 {
		return fmt.Errorf("no benchmark samples collected")
	}
	if s.Success == 0 {
		return fmt.Errorf("all traffic requests failed")
	}
	if opts.FailThreshold == 0 && s.Failed > 0 {
		return fmt.Errorf("%d/%d traffic requests failed", s.Failed, s.Requests)
	}
	if opts.FailThreshold > 0 && s.ErrorRate*100 > opts.FailThreshold {
		return fmt.Errorf("failure rate %.2f%% exceeded threshold %.2f%%", s.ErrorRate*100, opts.FailThreshold)
	}
	if opts.P95Threshold > 0 && time.Duration(s.LatencyMS.P95*float64(time.Millisecond)) > opts.P95Threshold {
		return fmt.Errorf("p95 latency %.0fms exceeded threshold %s", s.LatencyMS.P95, opts.P95Threshold)
	}
	return nil
}

func executeLoad(opts options, client *http.Client, baseURL, provider, model string, keys []seedmanifest.ManifestKey, promptSet []string) []result {
	scheduleCtx := context.Background()
	if opts.Duration > 0 {
		var cancel context.CancelFunc
		scheduleCtx, cancel = context.WithTimeout(scheduleCtx, opts.Duration)
		defer cancel()
	}

	jobs := make(chan int)
	results := make(chan result, maxInt(1024, opts.Concurrency*2))
	var wg sync.WaitGroup
	for worker := 0; worker < opts.Concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				key := keyForRequest(keys, opts.KeyStrategy, n)
				prompt := promptForRequest(promptSet, opts.Scenario, n)
				res := sendChat(context.Background(), client, baseURL, provider, key, model, prompt, opts.MaxTokens, opts.Stream)
				results <- res
			}
		}()
	}

	var collected []result
	collectedDone := make(chan struct{})
	go func() {
		defer close(collectedDone)
		for res := range results {
			collected = append(collected, res)
		}
	}()

	produceJobs(scheduleCtx, opts, jobs)
	close(jobs)
	wg.Wait()
	close(results)
	<-collectedDone
	return collected
}

func produceJobs(ctx context.Context, opts options, jobs chan<- int) {
	send := func(n int) bool {
		select {
		case jobs <- n:
			return true
		case <-ctx.Done():
			return false
		}
	}

	if opts.RPS <= 0 {
		for n := 0; opts.Duration > 0 || n < opts.Requests; n++ {
			if !send(n) {
				return
			}
		}
		return
	}

	interval := time.Duration(float64(time.Second) / opts.RPS)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for n := 0; opts.Duration > 0 || n < opts.Requests; n++ {
		if n > 0 {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
		if !send(n) {
			return
		}
	}
}

func buildSummary(opts options, baseURL, provider, model string, started, ended time.Time, results []result, snapshots []metricsSnapshot) summary {
	warmupCutoff := started.Add(opts.Warmup)
	filtered := make([]result, 0, len(results))
	for _, res := range results {
		if opts.Warmup > 0 && res.StartedAt.Before(warmupCutoff) {
			continue
		}
		filtered = append(filtered, res)
	}

	latencies := make([]time.Duration, 0, len(filtered))
	ttfts := make([]time.Duration, 0, len(filtered))
	statusCodes := make(map[string]int)
	var success, failed int
	var requestBytes, responseBytes int64
	for _, res := range filtered {
		requestBytes += int64(res.RequestBytes)
		responseBytes += res.ResponseBytes
		code := strconv.Itoa(res.Status)
		if res.Status == 0 {
			code = "error"
		}
		statusCodes[code]++
		if res.Err != nil || res.Status < 200 || res.Status >= 300 {
			failed++
			continue
		}
		success++
		latencies = append(latencies, res.Latency)
		if res.TTFT > 0 {
			ttfts = append(ttfts, res.TTFT)
		}
	}

	requests := len(filtered)
	durationSeconds := ended.Sub(started.Add(opts.Warmup)).Seconds()
	if opts.Duration > 0 {
		durationSeconds = (opts.Duration - opts.Warmup).Seconds()
	}
	if durationSeconds <= 0 {
		durationSeconds = ended.Sub(started).Seconds()
	}
	if durationSeconds <= 0 {
		durationSeconds = 1
	}
	var ttftSummary *durationSummary
	if len(ttfts) > 0 {
		d := summarizeDurations(ttfts)
		ttftSummary = &d
	}
	var errorRate float64
	if requests > 0 {
		errorRate = float64(failed) / float64(requests)
	}

	return summary{
		Scenario:           scenarioName(opts),
		BaseURL:            baseURL,
		Provider:           provider,
		Model:              model,
		StartedAt:          started,
		EndedAt:            ended,
		DurationSeconds:    durationSeconds,
		WarmupSeconds:      opts.Warmup.Seconds(),
		Requests:           requests,
		Success:            success,
		Failed:             failed,
		ErrorRate:          errorRate,
		ThroughputRPS:      float64(requests) / durationSeconds,
		LatencyMS:          summarizeDurations(latencies),
		TTFTMS:             ttftSummary,
		StatusCodes:        statusCodes,
		RequestBytesTotal:  requestBytes,
		ResponseBytesTotal: responseBytes,
		Options:            opts,
		Metrics:            snapshots,
	}
}

func summarizeDurations(values []time.Duration) durationSummary {
	if len(values) == 0 {
		return durationSummary{}
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return durationSummary{
		Min: ms(sorted[0]),
		P50: ms(percentileDuration(sorted, 50)),
		P90: ms(percentileDuration(sorted, 90)),
		P95: ms(percentileDuration(sorted, 95)),
		P99: ms(percentileDuration(sorted, 99)),
		Max: ms(sorted[len(sorted)-1]),
	}
}

func percentileDuration(sorted []time.Duration, percentile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil((percentile/100)*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func emitSummary(s summary, opts options) error {
	if opts.OutputFile != "" {
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(opts.OutputFile, append(data, '\n'), 0600); err != nil {
			return err
		}
	}
	if opts.Output == "json" {
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	printTextSummary(s)
	return nil
}

func printTextSummary(s summary) {
	fmt.Printf("scenario=%s provider=%s model=%s requests=%d success=%d failed=%d error_rate=%.2f%% throughput=%.2f rps\n", s.Scenario, s.Provider, s.Model, s.Requests, s.Success, s.Failed, s.ErrorRate*100, s.ThroughputRPS)
	fmt.Printf("latency_ms min=%.0f p50=%.0f p90=%.0f p95=%.0f p99=%.0f max=%.0f\n", s.LatencyMS.Min, s.LatencyMS.P50, s.LatencyMS.P90, s.LatencyMS.P95, s.LatencyMS.P99, s.LatencyMS.Max)
	if s.TTFTMS != nil {
		fmt.Printf("ttft_ms min=%.0f p50=%.0f p90=%.0f p95=%.0f p99=%.0f max=%.0f\n", s.TTFTMS.Min, s.TTFTMS.P50, s.TTFTMS.P90, s.TTFTMS.P95, s.TTFTMS.P99, s.TTFTMS.Max)
	}
	fmt.Printf("status_codes=%v request_bytes=%d response_bytes=%d\n", s.StatusCodes, s.RequestBytesTotal, s.ResponseBytesTotal)
	if len(s.Metrics) > 0 {
		fmt.Printf("metrics_snapshots=%d\n", len(s.Metrics))
	}
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

func sendChat(ctx context.Context, client *http.Client, baseURL, provider string, key seedmanifest.ManifestKey, model, prompt string, maxTokens int, stream bool) result {
	started := time.Now()
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
	}
	if stream {
		payload["stream"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return failedResult(err, started, key.Name)
	}
	endpoint := "/v1/chat/completions"
	if stream {
		endpoint = "/v1/chat/completions/stream"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return failedResult(err, started, key.Name)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-llm-gateway-api-key", key.Key)
	req.Header.Set("x-llm-provider", provider)
	resp, err := client.Do(req)
	if err != nil {
		return failedResult(err, started, key.Name)
	}
	defer resp.Body.Close()

	res := result{Status: resp.StatusCode, RequestBytes: len(body), KeyName: key.Name, StartedAt: started}
	var readErr error
	if stream {
		res.ResponseBytes, res.TTFT, readErr = readStreamingBody(resp.Body, started)
	} else {
		res.ResponseBytes, readErr = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseDrainBytes))
	}
	if readErr != nil {
		res.Err = readErr
		res.Error = readErr.Error()
	}
	res.EndedAt = time.Now()
	res.Latency = res.EndedAt.Sub(started)
	return res
}

func failedResult(err error, started time.Time, keyName string) result {
	ended := time.Now()
	return result{Err: err, Error: err.Error(), StartedAt: started, EndedAt: ended, Latency: ended.Sub(started), KeyName: keyName}
}

func readStreamingBody(body io.Reader, started time.Time) (int64, time.Duration, error) {
	reader := bufio.NewReader(body)
	buf := make([]byte, 32*1024)
	var total int64
	var ttft time.Duration
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if ttft == 0 {
				ttft = time.Since(started)
			}
			total += int64(n)
		}
		if errors.Is(err, io.EOF) {
			return total, ttft, nil
		}
		if err != nil {
			return total, ttft, err
		}
	}
}

func startMetricsSnapshots(ctx context.Context, client *http.Client, metricsURL string, interval time.Duration) <-chan []metricsSnapshot {
	ch := make(chan []metricsSnapshot, 1)
	go func() {
		defer close(ch)
		if strings.TrimSpace(metricsURL) == "" {
			ch <- nil
			return
		}
		snapshots := []metricsSnapshot{fetchMetricsSnapshot(ctx, client, metricsURL)}
		if interval > 0 {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					snapshots = append(snapshots, fetchMetricsSnapshot(ctx, client, metricsURL))
				case <-ctx.Done():
					snapshots = append(snapshots, fetchMetricsSnapshot(context.Background(), client, metricsURL))
					ch <- snapshots
					return
				}
			}
		}
		<-ctx.Done()
		snapshots = append(snapshots, fetchMetricsSnapshot(context.Background(), client, metricsURL))
		ch <- snapshots
	}()
	return ch
}

func fetchMetricsSnapshot(ctx context.Context, client *http.Client, metricsURL string) metricsSnapshot {
	snap := metricsSnapshot{Timestamp: time.Now(), Values: map[string]float64{}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	resp, err := client.Do(req)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snap.Error = fmt.Sprintf("status=%d", resp.StatusCode)
		return snap
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "llmgw_") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		snap.Values[fields[0]] = value
	}
	return snap
}

func promptsForOptions(opts options) ([]string, error) {
	if opts.PromptFile != "" {
		data, err := os.ReadFile(opts.PromptFile)
		if err != nil {
			return nil, err
		}
		var values []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				values = append(values, line)
			}
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("prompt file %s has no non-empty prompts", opts.PromptFile)
		}
		return values, nil
	}
	switch opts.PromptSize {
	case "short", "":
		return prompts, nil
	case "medium":
		return []string{
			"Explain how an API gateway improves reliability, observability, and provider abstraction in a production LLM platform. Keep the answer under five sentences.",
			"Summarize the tradeoffs between response caching, provider fallback, and tenant-level rate limiting for a multi-tenant LLM gateway.",
		}, nil
	case "large":
		base := "You are benchmarking an LLM gateway. Describe a production incident involving authentication, routing, provider failover, streaming responses, usage accounting, and observability. Include concrete symptoms, likely bottlenecks, and mitigation steps. "
		return []string{strings.Repeat(base, 24), strings.Repeat(base, 32)}, nil
	default:
		return nil, fmt.Errorf("unknown prompt-size %q", opts.PromptSize)
	}
}

func promptForRequest(promptSet []string, scenario string, n int) string {
	if scenario == "cache-repeat" && len(promptSet) > 0 {
		return promptSet[0]
	}
	return promptSet[n%len(promptSet)]
}

func keyForRequest(keys []seedmanifest.ManifestKey, strategy string, n int) seedmanifest.ManifestKey {
	switch strategy {
	case "single":
		return keys[0]
	case "random":
		return keys[rand.Intn(len(keys))]
	case "round-robin", "":
		return keys[n%len(keys)]
	default:
		return keys[n%len(keys)]
	}
}

func scenarioName(opts options) string {
	if strings.TrimSpace(opts.Scenario) != "" {
		return strings.TrimSpace(opts.Scenario)
	}
	return "custom"
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
