package routing

import (
	"hash/fnv"
	"path"

	"github.com/warm3snow/llm-gateway/internal/types"
)

func (e *Engine) abTest(req *types.ChatCompletionRequest) (*types.Options, error) {
	for _, rule := range e.Config.ABTests {
		if !abModelMatches(rule.Model, req.Model) {
			continue
		}
		if selected := e.selectABBucket(rule, abSeed(req)); selected != "" {
			return e.findOptionByName(selected)
		}
	}
	if e.Config.Default != "" {
		return e.findOptionByName(e.Config.Default)
	}
	return &e.Options[0], nil
}

func (e *Engine) selectABBucket(rule types.ABTestRule, seed string) string {
	total := 0
	for _, bucket := range rule.Options {
		if bucket.Weight > 0 {
			total += bucket.Weight
		}
	}
	if total <= 0 {
		return ""
	}
	bucketPoint := int(stableHash(seed) % uint32(total))
	for _, bucket := range rule.Options {
		weight := bucket.Weight
		if weight <= 0 {
			continue
		}
		bucketPoint -= weight
		if bucketPoint < 0 {
			return bucket.Provider
		}
	}
	return ""
}

func abModelMatches(pattern, model string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	matched, err := path.Match(pattern, model)
	if err != nil {
		return pattern == model
	}
	return matched
}

func abSeed(req *types.ChatCompletionRequest) string {
	if req.User != "" {
		return req.User + ":" + req.Model
	}
	return req.Model
}

func stableHash(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}
