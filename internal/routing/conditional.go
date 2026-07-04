package routing

import (
	"fmt"
	"strings"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// conditional selects a provider based on request content.
func (e *Engine) conditional(req *types.ChatCompletionRequest) (*types.Options, error) {
	if len(e.Config.Conditions) == 0 {
		if e.Config.Default != "" {
			return e.findOptionByName(e.Config.Default)
		}
		return &e.Options[0], nil
	}

	for _, cond := range e.Config.Conditions {
		if matchesCondition(cond.Query, req) {
			return e.findOptionByName(cond.Then)
		}
	}

	if e.Config.Default != "" {
		return e.findOptionByName(e.Config.Default)
	}

	return &e.Options[0], nil
}

// matchesCondition checks if a query map matches the request.
func matchesCondition(query map[string]interface{}, req *types.ChatCompletionRequest) bool {
	for key, expected := range query {
		actual := getRequestField(key, req)
		if !matchValue(expected, actual) {
			return false
		}
	}
	return true
}

// getRequestField extracts a field from the request by dot-notation path.
func getRequestField(path string, req *types.ChatCompletionRequest) interface{} {
	parts := splitDot(path)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "model":
		return req.Model
	case "stream":
		return req.Stream
	}
	return nil
}

// matchValue checks if expected matches actual.
func matchValue(expected, actual interface{}) bool {
	if expected == nil {
		return true
	}

	switch e := expected.(type) {
	case map[string]interface{}:
		return matchesOperator(e, actual)
	case string:
		s, ok := actual.(string)
		if !ok {
			return false
		}
		return strings.EqualFold(s, e)
	}
	return false
}

// matchesOperator handles $eq, $ne, $contains, etc.
func matchesOperator(op map[string]interface{}, actual interface{}) bool {
	for opName, opVal := range op {
		switch opName {
		case "$eq":
			return matchValue(opVal, actual)
		case "$ne":
			return !matchValue(opVal, actual)
		case "$contains":
			s, ok := actual.(string)
			if !ok {
				return false
			}
			sub, ok := opVal.(string)
			if !ok {
				return false
			}
			return strings.Contains(s, sub)
		}
	}
	return false
}

// splitDot splits a string by ".".
func splitDot(s string) []string {
	result := make([]string, 0)
	current := ""
	for _, c := range s {
		if c == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// findOptionByName finds an option by its Provider field.
func (e *Engine) findOptionByName(name string) (*types.Options, error) {
	for _, o := range e.Options {
		if o.Provider == name {
			return &o, nil
		}
	}
	if len(e.Options) > 0 {
		return &e.Options[0], nil
	}
	return nil, fmt.Errorf("no option found for: %s", name)
}
