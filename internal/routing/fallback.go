package routing

import (
	"github.com/warm3snow/llm-gateway/internal/types"
)

// Fallback returns providers in order.
func (e *Engine) Fallback() []*types.Options {
	result := make([]*types.Options, 0, len(e.Options))
	for i := range e.Options {
		result = append(result, &e.Options[i])
	}
	return result
}

// SelectFallback picks the next provider to try.
func (e *Engine) SelectFallback(attempt int, lastStatusCode int) (*types.Options, bool) {
	onCodes := e.Config.OnStatusCodes
	if len(onCodes) > 0 {
		matched := false
		for _, c := range onCodes {
			if c == lastStatusCode {
				matched = true
				break
			}
		}
		if !matched {
			return nil, false
		}
	}

	if attempt >= len(e.Options) {
		return nil, false
	}
	return &e.Options[attempt], true
}
