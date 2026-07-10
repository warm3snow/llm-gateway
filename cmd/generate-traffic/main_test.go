package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestApplyScenarioDefaults(t *testing.T) {
	opts := options{Scenario: "streaming-ttft", MaxTokens: 20, PromptSize: "short"}
	applyScenarioDefaults(&opts)

	if !opts.Stream {
		t.Fatalf("streaming-ttft should enable streaming")
	}
	if opts.MaxTokens != 128 {
		t.Fatalf("streaming-ttft max tokens = %d, want 128", opts.MaxTokens)
	}
}

func TestDurationSummaryPercentiles(t *testing.T) {
	summary := summarizeDurations([]time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	})

	if summary.P50 != 300 {
		t.Fatalf("p50 = %.0f, want 300", summary.P50)
	}
	if summary.P90 != 500 {
		t.Fatalf("p90 = %.0f, want 500", summary.P90)
	}
	if summary.P99 != 500 {
		t.Fatalf("p99 = %.0f, want 500", summary.P99)
	}
}

func TestBuildSummaryExcludesWarmupSamples(t *testing.T) {
	started := time.Unix(100, 0)
	results := []result{
		{Status: 200, Latency: 5 * time.Second, StartedAt: started.Add(500 * time.Millisecond), EndedAt: started.Add(6 * time.Second)},
		{Status: 200, Latency: 100 * time.Millisecond, StartedAt: started.Add(2 * time.Second), EndedAt: started.Add(2100 * time.Millisecond)},
		{Status: 500, Latency: 200 * time.Millisecond, StartedAt: started.Add(3 * time.Second), EndedAt: started.Add(3200 * time.Millisecond)},
	}

	summary := buildSummary(options{Scenario: "baseline-nonstream", Warmup: time.Second}, "http://gateway", "openai", "gpt-test", started, started.Add(4*time.Second), results, nil)

	if summary.Requests != 2 {
		t.Fatalf("requests = %d, want 2", summary.Requests)
	}
	if summary.Success != 1 || summary.Failed != 1 {
		t.Fatalf("success/failed = %d/%d, want 1/1", summary.Success, summary.Failed)
	}
	if summary.StatusCodes["500"] != 1 {
		t.Fatalf("500 count = %d, want 1", summary.StatusCodes["500"])
	}
	if summary.LatencyMS.Max != 100 {
		t.Fatalf("successful max latency = %.0f, want 100", summary.LatencyMS.Max)
	}
}

func TestProduceJobsSendsFirstRPSJobImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	jobs := make(chan int)
	done := make(chan []int, 1)

	go func() {
		var got []int
		for job := range jobs {
			got = append(got, job)
		}
		done <- got
	}()

	produceJobs(ctx, options{Duration: 100 * time.Millisecond, RPS: 1}, jobs)
	close(jobs)
	got := <-done

	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("jobs = %v, want first job immediately", got)
	}
}

func TestFetchMetricsSnapshotParsesValueBeforeTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("# HELP llmgw_requests_total requests\nllmgw_requests_total{provider=\"openai\"} 42 1710000000000\n"))
	}))
	defer server.Close()

	snapshot := fetchMetricsSnapshot(context.Background(), server.Client(), server.URL)

	if snapshot.Error != "" {
		t.Fatalf("unexpected snapshot error: %s", snapshot.Error)
	}
	if got := snapshot.Values["llmgw_requests_total{provider=\"openai\"}"]; got != 42 {
		t.Fatalf("metric value = %v, want 42", got)
	}
}

func TestReadStreamingBodyPropagatesReadErrors(t *testing.T) {
	_, _, err := readStreamingBody(errReader{}, time.Now())
	if err == nil {
		t.Fatalf("expected read error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, context.Canceled
}
