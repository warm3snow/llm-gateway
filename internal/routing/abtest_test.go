package routing

import (
	"fmt"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/types"
)

func TestABTestSelectsStableBucketForSameUserAndModel(t *testing.T) {
	engine := NewEngine(&types.Strategy{Mode: types.StrategyABTest, ABTests: []types.ABTestRule{{Model: "gpt-*", Options: []types.ABBucket{{Provider: "openai", Weight: 50}, {Provider: "anthropic", Weight: 50}}}}}, []types.Options{{Provider: "openai"}, {Provider: "anthropic"}})
	req := &types.ChatCompletionRequest{Model: "gpt-4o", User: "user-123"}

	first, err := engine.Select(req)
	if err != nil {
		t.Fatalf("first select: %v", err)
	}
	for i := 0; i < 20; i++ {
		next, err := engine.Select(req)
		if err != nil {
			t.Fatalf("select %d: %v", i, err)
		}
		if next.Provider != first.Provider {
			t.Fatalf("provider changed from %s to %s", first.Provider, next.Provider)
		}
	}
}

func TestABTestDistributionUsesWeights(t *testing.T) {
	engine := NewEngine(&types.Strategy{Mode: types.StrategyABTest, ABTests: []types.ABTestRule{{Model: "gpt-*", Options: []types.ABBucket{{Provider: "openai", Weight: 80}, {Provider: "anthropic", Weight: 20}}}}}, []types.Options{{Provider: "openai"}, {Provider: "anthropic"}})
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		opt, err := engine.Select(&types.ChatCompletionRequest{Model: "gpt-4o", User: fmt.Sprintf("user-%d", i)})
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		counts[opt.Provider]++
	}
	if counts["openai"] < 700 || counts["openai"] > 900 {
		t.Fatalf("openai count = %d, want roughly 800; counts=%v", counts["openai"], counts)
	}
	if counts["anthropic"] < 100 || counts["anthropic"] > 300 {
		t.Fatalf("anthropic count = %d, want roughly 200; counts=%v", counts["anthropic"], counts)
	}
}

func TestABTestFallsBackWhenNoRuleMatches(t *testing.T) {
	engine := NewEngine(&types.Strategy{Mode: types.StrategyABTest, Default: "anthropic", ABTests: []types.ABTestRule{{Model: "gpt-*", Options: []types.ABBucket{{Provider: "openai", Weight: 100}}}}}, []types.Options{{Provider: "openai"}, {Provider: "anthropic"}})
	opt, err := engine.Select(&types.ChatCompletionRequest{Model: "claude-3", User: "user-123"})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if opt.Provider != "anthropic" {
		t.Fatalf("provider = %s, want anthropic", opt.Provider)
	}
}
