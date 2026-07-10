package routing

import (
	"fmt"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// Engine selects a provider based on the configured strategy.
type Engine struct {
	Config  *types.Strategy
	Options []types.Options
}

// NewEngine creates a new routing engine.
func NewEngine(cfg *types.Strategy, opts []types.Options) *Engine {
	if cfg == nil {
		cfg = &types.Strategy{Mode: types.StrategySingle}
	}
	return &Engine{Config: cfg, Options: opts}
}

// Select returns the selected Options for this request.
func (e *Engine) Select(req interface{}) (*types.Options, error) {
	if len(e.Options) == 0 {
		return nil, fmt.Errorf("no provider options configured")
	}

	switch e.Config.Mode {
	case types.StrategyLoadBalance:
		return e.loadBalance(), nil
	case types.StrategyFallback:
		return &e.Options[0], nil
	case types.StrategyConditional:
		if r, ok := req.(*types.ChatCompletionRequest); ok {
			return e.conditional(r)
		}
		return &e.Options[0], nil
	case types.StrategyABTest:
		if r, ok := req.(*types.ChatCompletionRequest); ok {
			return e.abTest(r)
		}
		return &e.Options[0], nil
	case types.StrategySingle:
		return &e.Options[0], nil
	default:
		return &e.Options[0], nil
	}
}

// All returns all configured options (for retry/fallback iteration).
func (e *Engine) All() []types.Options {
	return e.Options
}
