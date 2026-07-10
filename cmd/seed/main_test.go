package main

import (
	"reflect"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/types"
)

func TestOrderedProviderNamesPrefersDefaultProvider(t *testing.T) {
	providers := map[string]types.Options{
		"deepseek": {},
		"ollama":   {},
		"openai":   {},
	}

	got := orderedProviderNames(providers, "ollama")
	want := []string{"ollama", "deepseek", "openai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderedProviderNames() = %#v, want %#v", got, want)
	}
}

func TestOrderedProviderNamesSortsWithoutDefaultProvider(t *testing.T) {
	providers := map[string]types.Options{
		"ollama":   {},
		"deepseek": {},
	}

	got := orderedProviderNames(providers, "missing")
	want := []string{"deepseek", "ollama"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderedProviderNames() = %#v, want %#v", got, want)
	}
}
