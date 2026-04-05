package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/ochanuco/marume/internal/domain"
	_ "modernc.org/sqlite"
)

// SQLiteRuleStore loads one SQLite snapshot produced by the Python pipeline.
type SQLiteRuleStore struct {
	path string
}

var _ ReadableRuleStore = (*SQLiteRuleStore)(nil)

// NewSQLiteRuleStore creates a SQLite-backed rule store for a single snapshot file.
func NewSQLiteRuleStore(path string) (*SQLiteRuleStore, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlitestore: path cannot be empty")
	}
	return &SQLiteRuleStore{path: path}, nil
}

// ReadRuleSet reads a single SQLite rule snapshot without fiscal-year validation.
func (s *SQLiteRuleStore) ReadRuleSet(ctx context.Context) (domain.RuleSet, error) {
	if _, err := os.Stat(s.path); err != nil {
		return domain.RuleSet{}, fmt.Errorf("read sqlite rule set: %w", err)
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return domain.RuleSet{}, fmt.Errorf("open sqlite rule set: %w", err)
	}
	defer func() { _ = db.Close() }()

	ruleSet, ruleSetID, err := s.readRuleSetMetadata(ctx, db)
	if err != nil {
		return domain.RuleSet{}, err
	}

	rules, err := s.readRules(ctx, db, ruleSetID)
	if err != nil {
		return domain.RuleSet{}, err
	}
	ruleSet.Rules = rules
	return ruleSet, nil
}

// LoadRuleSet reads a SQLite snapshot and verifies that its fiscal year matches the request.
func (s *SQLiteRuleStore) LoadRuleSet(ctx context.Context, fiscalYear int) (domain.RuleSet, error) {
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

func (s *SQLiteRuleStore) readRuleSetMetadata(ctx context.Context, db *sql.DB) (domain.RuleSet, string, error) {
	const query = `
SELECT
	rule_set_id,
	fiscal_year,
	rule_version,
	build_id,
	built_at
FROM rule_sets
ORDER BY fiscal_year DESC, rule_set_id
LIMIT 1
`

	var (
		ruleSetID   string
		fiscalYear  int
		ruleVersion string
		buildID     string
		builtAt     sql.NullString
	)
	if err := db.QueryRowContext(ctx, query).Scan(&ruleSetID, &fiscalYear, &ruleVersion, &buildID, &builtAt); err != nil {
		if err == sql.ErrNoRows {
			return domain.RuleSet{}, "", fmt.Errorf("read sqlite rule set metadata: rule_sets is empty")
		}
		return domain.RuleSet{}, "", fmt.Errorf("read sqlite rule set metadata: %w", err)
	}

	return domain.RuleSet{
		FiscalYear:  fiscalYear,
		RuleVersion: ruleVersion,
		BuildID:     buildID,
		BuiltAt:     s.resolveBuiltAt(builtAt),
	}, ruleSetID, nil
}

func (s *SQLiteRuleStore) readRules(ctx context.Context, db *sql.DB, ruleSetID string) ([]domain.Rule, error) {
	const ruleQuery = `
SELECT
	rule_id,
	priority,
	dpc_code
FROM rules
WHERE rule_set_id = ?
ORDER BY priority, rule_id
`

	ruleRows, err := db.QueryContext(ctx, ruleQuery, ruleSetID)
	if err != nil {
		return nil, fmt.Errorf("read sqlite rules: %w", err)
	}
	defer func() { _ = ruleRows.Close() }()

	rules := make([]domain.Rule, 0)
	for ruleRows.Next() {
		var rule domain.Rule
		if err := ruleRows.Scan(&rule.ID, &rule.Priority, &rule.DPCCode); err != nil {
			return nil, fmt.Errorf("scan sqlite rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite rules: %w", err)
	}

	conditionsByRuleID, err := readConditions(ctx, db, ruleSetID)
	if err != nil {
		return nil, err
	}
	for idx := range rules {
		rules[idx].Conditions = conditionsByRuleID[rules[idx].ID]
	}

	return rules, nil
}

func readConditions(ctx context.Context, db *sql.DB, ruleSetID string) (map[string][]domain.Condition, error) {
	const conditionQuery = `
SELECT
	rc.rule_id,
	rc.condition_type,
	rc.operator,
	rc.value_text,
	rc.value_num,
	rc.value_json
FROM rule_conditions rc
JOIN rules r ON r.rule_id = rc.rule_id
WHERE r.rule_set_id = ?
ORDER BY rc.rule_id, rc.condition_id
`

	rows, err := db.QueryContext(ctx, conditionQuery, ruleSetID)
	if err != nil {
		return nil, fmt.Errorf("read sqlite rule conditions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	conditionsByRuleID := make(map[string][]domain.Condition)
	for rows.Next() {
		var (
			ruleID        string
			conditionType string
			operator      string
			valueText     sql.NullString
			valueNum      sql.NullFloat64
			valueJSON     sql.NullString
		)
		if err := rows.Scan(&ruleID, &conditionType, &operator, &valueText, &valueNum, &valueJSON); err != nil {
			return nil, fmt.Errorf("scan sqlite rule condition: %w", err)
		}

		condition, err := normalizeSQLiteCondition(conditionType, operator, valueText, valueNum, valueJSON)
		if err != nil {
			return nil, fmt.Errorf("normalize sqlite rule condition for %s: %w", ruleID, err)
		}
		conditionsByRuleID[ruleID] = append(conditionsByRuleID[ruleID], condition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite rule conditions: %w", err)
	}

	return conditionsByRuleID, nil
}

func normalizeSQLiteCondition(
	conditionType string,
	operator string,
	valueText sql.NullString,
	valueNum sql.NullFloat64,
	valueJSON sql.NullString,
) (domain.Condition, error) {
	normalizedType := normalizeConditionType(conditionType)
	normalizedOperator := normalizeConditionOperator(operator)

	condition := domain.Condition{
		Type:     normalizedType,
		Operator: normalizedOperator,
	}

	switch normalizedType {
	case "main_diagnosis", "sex":
		if !valueText.Valid || valueText.String == "" {
			return domain.Condition{}, fmt.Errorf("%s requires value_text", normalizedType)
		}
		condition.Values = []string{valueText.String}
	case "diagnoses", "procedures", "comorbidities":
		values, err := decodeConditionValues(valueText, valueJSON)
		if err != nil {
			return domain.Condition{}, err
		}
		condition.Values = values
	case "age":
		if !valueNum.Valid {
			return domain.Condition{}, fmt.Errorf("age requires value_num")
		}
		value, ok := exactIntFromFloat64(valueNum.Float64)
		if !ok {
			return domain.Condition{}, fmt.Errorf("age requires integer value_num")
		}
		condition.IntValue = &value
	default:
		values, err := decodeConditionValues(valueText, valueJSON)
		if err != nil {
			return domain.Condition{}, err
		}
		condition.Values = values
		if valueNum.Valid {
			if value, ok := exactIntFromFloat64(valueNum.Float64); ok {
				condition.IntValue = &value
			}
		}
	}

	return condition, nil
}

func normalizeConditionType(value string) string {
	switch strings.ToLower(value) {
	case "procedure":
		return "procedures"
	case "diagnosis":
		return "diagnoses"
	case "comorbidity":
		return "comorbidities"
	default:
		return strings.ToLower(value)
	}
}

func normalizeConditionOperator(value string) string {
	switch strings.ToLower(value) {
	case "eq":
		return "equals"
	case "in":
		return "contains_any"
	default:
		return strings.ToLower(value)
	}
}

func decodeConditionValues(valueText sql.NullString, valueJSON sql.NullString) ([]string, error) {
	if valueJSON.Valid && valueJSON.String != "" {
		var values []string
		if err := json.Unmarshal([]byte(valueJSON.String), &values); err != nil {
			return nil, fmt.Errorf("decode value_json: %w", err)
		}
		return values, nil
	}
	if valueText.Valid && valueText.String != "" {
		return []string{valueText.String}, nil
	}
	return nil, nil
}

func (s *SQLiteRuleStore) resolveBuiltAt(builtAt sql.NullString) string {
	if builtAt.Valid && builtAt.String != "" {
		return builtAt.String
	}

	// The current POC snapshots can omit built_at, so fall back to the file mtime.
	info, err := os.Stat(s.path)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
}

func exactIntFromFloat64(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value {
		return 0, false
	}

	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	if value < float64(minInt) || value > float64(maxInt) {
		return 0, false
	}

	return int(value), true
}
