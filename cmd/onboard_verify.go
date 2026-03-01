package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// providerVerifyError holds the result of a provider connectivity probe.
type providerVerifyError struct {
	fatal   bool   // true = bad credentials, block startup
	message string // human-readable description
}

func (e *providerVerifyError) Error() string { return e.message }

// verifyProviderConnectivity checks whether a provider's API key is valid by
// sending a minimal POST to the chat completions endpoint. This endpoint always
// requires authentication, unlike /models which is public on many providers.
//
// Strategy: POST an empty JSON body to /chat/completions.
//   - 401/403 → invalid API key (fatal)
//   - 400/422 → key is valid, request is bad (expected — means auth passed)
//   - 2xx     → key is valid (unexpected but fine)
//   - 5xx     → transient server error (non-fatal warning)
func verifyProviderConnectivity(cfg *config.Config, providerName string) *providerVerifyError {
	apiBase := resolveProviderAPIBase(providerName)
	if apiBase == "" {
		return nil // custom/unknown provider — skip verification
	}

	apiKey := resolveProviderAPIKey(cfg, providerName)
	if apiKey == "" {
		return nil // no key to verify
	}

	url := resolveAuthCheckEndpoint(providerName, apiBase)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, strings.NewReader("{}"))
	if err != nil {
		return &providerVerifyError{fatal: false, message: fmt.Sprintf("build request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	// Anthropic uses x-api-key header; all others use Bearer token.
	if providerName == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &providerVerifyError{fatal: false, message: fmt.Sprintf("connectivity check failed (transient): %v", err)}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return &providerVerifyError{fatal: true, message: fmt.Sprintf("%s returned %d — invalid API key", providerName, resp.StatusCode)}
	case resp.StatusCode == 400 || resp.StatusCode == 422:
		// Auth passed but request body is invalid — key is valid
		return nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 500:
		return &providerVerifyError{fatal: false, message: fmt.Sprintf("%s returned %d (transient, continuing)", providerName, resp.StatusCode)}
	default:
		// Unexpected status (404, 429, etc.) — warn but don't block
		return &providerVerifyError{fatal: false, message: fmt.Sprintf("%s returned %d (unexpected, continuing)", providerName, resp.StatusCode)}
	}
}

// resolveAuthCheckEndpoint returns the chat completions URL for auth verification.
// This endpoint requires valid credentials on all providers.
func resolveAuthCheckEndpoint(providerName, apiBase string) string {
	switch providerName {
	case "anthropic":
		return "https://api.anthropic.com/v1/messages"
	default:
		return apiBase + "/chat/completions"
	}
}

// verifyAllProviders checks connectivity for every provider that has an API key.
// Returns a list of fatal errors (invalid keys). Non-fatal warnings are logged.
func verifyAllProviders(cfg *config.Config) []string {
	var fatalErrors []string

	for _, name := range providerPriority {
		apiKey := resolveProviderAPIKey(cfg, name)
		if apiKey == "" {
			continue
		}

		verr := verifyProviderConnectivity(cfg, name)
		if verr == nil {
			slog.Info("provider connectivity verified", "provider", name)
			fmt.Printf("    %s: OK\n", name)
			continue
		}

		if verr.fatal {
			slog.Error("provider key invalid", "provider", name, "error", verr.message)
			fmt.Printf("    %s: FAILED — %s\n", name, verr.message)
			fatalErrors = append(fatalErrors, fmt.Sprintf("%s: %s", name, verr.message))
		} else {
			slog.Warn("provider connectivity warning", "provider", name, "warning", verr.message)
			fmt.Printf("    %s: WARNING — %s\n", name, verr.message)
		}
	}

	return fatalErrors
}
