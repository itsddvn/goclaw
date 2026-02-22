package store

import (
	"context"

	"github.com/google/uuid"
)

// LLMProviderData represents an LLM provider configuration.
type LLMProviderData struct {
	BaseModel
	Name         string `json:"name"`
	DisplayName  string `json:"display_name,omitempty"`
	ProviderType string `json:"provider_type"` // "anthropic_native", "openai_compat"
	APIBase      string `json:"api_base,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// ProviderStore manages LLM providers (managed mode only).
type ProviderStore interface {
	CreateProvider(ctx context.Context, p *LLMProviderData) error
	GetProvider(ctx context.Context, id uuid.UUID) (*LLMProviderData, error)
	ListProviders(ctx context.Context) ([]LLMProviderData, error)
	UpdateProvider(ctx context.Context, id uuid.UUID, updates map[string]any) error
	DeleteProvider(ctx context.Context, id uuid.UUID) error
}
