package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// topOpenRouterProviders is a curated whitelist of the best model providers on OpenRouter.
var topOpenRouterProviders = map[string]bool{
	"anthropic":    true,
	"openai":       true,
	"google":       true,
	"mistralai":    true,
	"deepseek":     true,
	"meta-llama":   true,
	"qwen":         true,
	"x-ai":         true,
	"z-ai":         true,
	"nvidia":       true,
	"moonshotai":   true,
	"minimax":      true,
	"allenai":      true,
	"cohere":       true,
	"perplexity":   true,
	"amazon":       true,
	"microsoft":    true,
	"ai21":         true,
	"nousresearch": true,
	"baidu":        true,
}

type openRouterModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContextLength int    `json:"context_length"`
}

func fetchOpenRouterModels() ([]openRouterModel, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []openRouterModel `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	sort.Slice(result.Data, func(i, j int) bool {
		return result.Data[i].ID < result.Data[j].ID
	})

	return result.Data, nil
}

func filterTopProviderModels(models []openRouterModel) []openRouterModel {
	var filtered []openRouterModel
	for _, m := range models {
		parts := strings.SplitN(m.ID, "/", 2)
		if len(parts) == 2 && topOpenRouterProviders[parts[0]] {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func formatCtx(ctxLen int) string {
	if ctxLen >= 1_000_000 {
		return fmt.Sprintf("%.0fM ctx", float64(ctxLen)/1_000_000)
	}
	if ctxLen >= 1000 {
		return fmt.Sprintf("%dK ctx", ctxLen/1000)
	}
	return fmt.Sprintf("%d ctx", ctxLen)
}

// fallbackModels used when API is unreachable.
var fallbackModels = []openRouterModel{
	{ID: "anthropic/claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5", ContextLength: 200000},
	{ID: "openai/gpt-4o", Name: "GPT-4o", ContextLength: 128000},
	{ID: "google/gemini-2.0-flash-001", Name: "Gemini 2.0 Flash", ContextLength: 1048576},
	{ID: "deepseek/deepseek-chat-v3-0324", Name: "DeepSeek V3", ContextLength: 131072},
}

// buildOpenRouterModelOptions builds select options from OpenRouter models.
// Pre-fetches from API; uses fallback on failure.
func buildOpenRouterModelOptions() []SelectOption[string] {
	models, err := fetchOpenRouterModels()
	if err != nil || len(models) == 0 {
		models = fallbackModels
	} else {
		models = filterTopProviderModels(models)
		if len(models) == 0 {
			models = fallbackModels
		}
	}

	options := make([]SelectOption[string], 0, len(models)+1)
	for _, m := range models {
		label := fmt.Sprintf("%-50s (%s)", m.ID, formatCtx(m.ContextLength))
		options = append(options, SelectOption[string]{Label: label, Value: m.ID})
	}
	options = append(options, SelectOption[string]{Label: "Enter custom model ID...", Value: "__custom__"})
	return options
}
