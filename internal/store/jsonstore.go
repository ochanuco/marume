package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ochanuco/marume/internal/domain"
)

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

// JSONRuleStore loads a single JSON rule snapshot from disk.
type JSONRuleStore struct {
	path string
}

// NewJSONRuleStore creates a strict JSON-backed rule store for a single rule snapshot file.
func NewJSONRuleStore(path string) (*JSONRuleStore, error) {
	if path == "" {
		return nil, fmt.Errorf("jsonstore: path cannot be empty")
	}
	return &JSONRuleStore{path: path}, nil
}

// ReadRuleSet reads a single strict JSON rule file without fiscal-year validation.
func (s *JSONRuleStore) ReadRuleSet(_ context.Context) (domain.RuleSet, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return domain.RuleSet{}, fmt.Errorf("read rule set: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var ruleSet domain.RuleSet
	if err := decoder.Decode(&ruleSet); err != nil {
		return domain.RuleSet{}, fmt.Errorf("decode rule set: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.RuleSet{}, fmt.Errorf("decode rule set: unexpected trailing data")
	}

	return ruleSet, nil
}

// LoadRuleSet reads a rule file and verifies that its fiscal year matches the requested year.
func (s *JSONRuleStore) LoadRuleSet(ctx context.Context, fiscalYear int) (domain.RuleSet, error) {
	ruleSet, err := s.ReadRuleSet(ctx)
	if err != nil {
		return domain.RuleSet{}, err
	}
	if ruleSet.FiscalYear != fiscalYear {
		return domain.RuleSet{}, FiscalYearMismatchError{
			RuleSetFiscalYear: ruleSet.FiscalYear,
			RequestedYear:     fiscalYear,
		}
	}

	return ruleSet, nil
}
