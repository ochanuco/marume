package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ochanuco/marume/internal/domain"
)

// ReadableRuleStore exposes both direct reads and fiscal-year-aware reads.
type ReadableRuleStore interface {
	ReadRuleSet(ctx context.Context) (domain.RuleSet, error)
	LoadRuleSet(ctx context.Context, fiscalYear int) (domain.RuleSet, error)
}

// NewRuleStore selects the appropriate store implementation from the snapshot path.
func NewRuleStore(path string) (ReadableRuleStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store: path cannot be empty")
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".sqlite", ".sqlite3", ".db":
		return NewSQLiteRuleStore(path)
	default:
		return NewJSONRuleStore(path)
	}
}
