package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ochanuco/marume/internal/domain"
)

type JSONRuleStore struct {
	path string
}

func NewJSONRuleStore(path string) *JSONRuleStore {
	return &JSONRuleStore{path: path}
}

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

	return ruleSet, nil
}

func (s *JSONRuleStore) LoadRuleSet(ctx context.Context, fiscalYear int) (domain.RuleSet, error) {
	ruleSet, err := s.ReadRuleSet(ctx)
	if err != nil {
		return domain.RuleSet{}, err
	}
	if ruleSet.FiscalYear != fiscalYear {
		return domain.RuleSet{}, fmt.Errorf("rule set fiscal year %d does not match requested %d", ruleSet.FiscalYear, fiscalYear)
	}

	return ruleSet, nil
}
