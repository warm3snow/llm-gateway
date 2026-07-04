package routing

import (
	"math/rand"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// loadBalance selects a provider by weight.
func (e *Engine) loadBalance() *types.Options {
	totalWeight := 0
	for _, o := range e.Options {
		w := o.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	if totalWeight == 0 {
		idx := rand.Intn(len(e.Options))
		return &e.Options[idx]
	}

	r := rand.Intn(totalWeight)
	for i := range e.Options {
		w := e.Options[i].Weight
		if w <= 0 {
			w = 1
		}
		r -= w
		if r < 0 {
			return &e.Options[i]
		}
	}

	return &e.Options[0]
}
