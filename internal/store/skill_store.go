package store

import "context"

// SkillInfo describes a discovered skill.
type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	BaseDir     string `json:"baseDir"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// SkillSearchResult is a scored skill returned from embedding search.
type SkillSearchResult struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Path        string  `json:"path"`
	Score       float64 `json:"score"`
}

// SkillStore manages skill discovery and loading.
// In standalone mode, wraps the filesystem-based Loader.
// In managed mode, backed by Postgres + filesystem content.
type SkillStore interface {
	ListSkills() []SkillInfo
	LoadSkill(name string) (string, bool)
	LoadForContext(allowList []string) string
	BuildSummary(allowList []string) string
	GetSkill(name string) (*SkillInfo, bool)
	FilterSkills(allowList []string) []SkillInfo
	Version() int64
	BumpVersion()
	Dirs() []string
}

// EmbeddingSkillSearcher is an optional interface for stores that support
// vector-based skill search. PGSkillStore implements this; FileSkillStore does not.
type EmbeddingSkillSearcher interface {
	SearchByEmbedding(ctx context.Context, embedding []float32, limit int) ([]SkillSearchResult, error)
	SetEmbeddingProvider(provider EmbeddingProvider)
	BackfillSkillEmbeddings(ctx context.Context) (int, error)
}
