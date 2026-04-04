package store

import (
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

func (s *JSONRuleStore) LoadRuleSet(_ context.Context, fiscalYear int) (domain.RuleSet, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return domain.RuleSet{}, fmt.Errorf("read rule set: %w", err)
	}

	var ruleSet domain.RuleSet
	if err := json.Unmarshal(data, &ruleSet); err != nil {
		return domain.RuleSet{}, fmt.Errorf("decode rule set: %w", err)
	}
	if ruleSet.FiscalYear != fiscalYear {
		return domain.RuleSet{}, fmt.Errorf("rule set fiscal year %d does not match requested %d", ruleSet.FiscalYear, fiscalYear)
	}

	return ruleSet, nil
}
