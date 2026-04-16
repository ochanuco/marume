package store

import (
	"context"
	"errors"
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

// ErrFiscalYearMismatch indicates that the requested fiscal year differs from the loaded rule set.
var ErrFiscalYearMismatch = errors.New("fiscal year mismatch")

// FiscalYearMismatchError reports the actual and requested fiscal years for rule selection.
type FiscalYearMismatchError struct {
	RuleSetFiscalYear int
	RequestedYear     int
}

// Error formats the fiscal-year mismatch for user-facing logs and CLI output.
func (e FiscalYearMismatchError) Error() string {
	return fmt.Sprintf("rule set fiscal year %d does not match requested %d", e.RuleSetFiscalYear, e.RequestedYear)
}

// Unwrap exposes ErrFiscalYearMismatch for errors.Is checks.
func (e FiscalYearMismatchError) Unwrap() error {
	return ErrFiscalYearMismatch
}

// NewRuleStore selects the SQLite store implementation from the snapshot path.
// Only SQLite snapshots (.sqlite/.sqlite3/.db) are supported; other extensions
// are rejected explicitly so callers cannot accidentally pass a legacy JSON snapshot.
func NewRuleStore(path string) (ReadableRuleStore, error) {
	if err := ValidateSQLiteSnapshotPath(path); err != nil {
		return nil, err
	}
	return NewSQLiteRuleStore(path)
}

// ValidateSQLiteSnapshotPath rejects empty paths and non-SQLite extensions.
func ValidateSQLiteSnapshotPath(path string) error {
	if path == "" {
		return fmt.Errorf("store: path cannot be empty")
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".sqlite", ".sqlite3", ".db":
		return nil
	default:
		return fmt.Errorf("store: unsupported rule snapshot extension %q; SQLite (.sqlite/.sqlite3/.db) のみサポートしています", ext)
	}
}
