package testutil

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ochanuco/marume/internal/domain"
	_ "modernc.org/sqlite"
)

// LoadRuleSetJSON reads a strict JSON rule fixture from disk.
func LoadRuleSetJSON(path string) (domain.RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.RuleSet{}, fmt.Errorf("read rule fixture: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var ruleSet domain.RuleSet
	if err := decoder.Decode(&ruleSet); err != nil {
		return domain.RuleSet{}, fmt.Errorf("decode rule fixture: %w", err)
	}
	return ruleSet, nil
}

// WriteSQLiteRuleSetFromJSON reads a JSON rule fixture and writes an equivalent SQLite snapshot.
func WriteSQLiteRuleSetFromJSON(jsonPath string, sqlitePath string) error {
	ruleSet, err := LoadRuleSetJSON(jsonPath)
	if err != nil {
		return err
	}
	return WriteSQLiteRuleSet(sqlitePath, ruleSet)
}

// WriteSQLiteRuleSet materializes a RuleSet into a SQLite snapshot for tests.
func WriteSQLiteRuleSet(path string, ruleSet domain.RuleSet) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create sqlite fixture dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open sqlite fixture: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := createSchema(db); err != nil {
		return err
	}
	if err := insertRuleSet(db, ruleSet); err != nil {
		return err
	}
	return nil
}

func createSchema(db *sql.DB) error {
	const schema = `
CREATE TABLE rule_sets (
	rule_set_id TEXT PRIMARY KEY,
	fiscal_year INTEGER NOT NULL,
	rule_version TEXT NOT NULL,
	source_url TEXT,
	source_published_at TEXT,
	build_id TEXT NOT NULL,
	built_at TEXT
);
CREATE TABLE rules (
	rule_id TEXT PRIMARY KEY,
	rule_set_id TEXT NOT NULL,
	priority INTEGER NOT NULL,
	dpc_code TEXT NOT NULL,
	mdc_code TEXT,
	label TEXT,
	FOREIGN KEY (rule_set_id) REFERENCES rule_sets(rule_set_id)
);
CREATE TABLE rule_conditions (
	condition_id TEXT PRIMARY KEY,
	rule_id TEXT NOT NULL,
	condition_type TEXT NOT NULL,
	operator TEXT NOT NULL,
	value_text TEXT,
	value_num REAL,
	value_json TEXT,
	negated INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (rule_id) REFERENCES rules(rule_id)
);
CREATE TABLE metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create sqlite fixture schema: %w", err)
	}
	return nil
}

func insertRuleSet(db *sql.DB, ruleSet domain.RuleSet) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin sqlite fixture tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	ruleSetID := fmt.Sprintf("dpc-%d", ruleSet.FiscalYear)
	if _, err := tx.Exec(
		`INSERT INTO rule_sets(rule_set_id, fiscal_year, rule_version, build_id, built_at) VALUES (?, ?, ?, ?, ?)`,
		ruleSetID,
		ruleSet.FiscalYear,
		ruleSet.RuleVersion,
		ruleSet.BuildID,
		ruleSet.BuiltAt,
	); err != nil {
		return fmt.Errorf("insert sqlite fixture rule_set: %w", err)
	}

	for _, rule := range ruleSet.Rules {
		if _, err := tx.Exec(
			`INSERT INTO rules(rule_id, rule_set_id, priority, dpc_code, label) VALUES (?, ?, ?, ?, ?)`,
			rule.ID,
			ruleSetID,
			rule.Priority,
			rule.DPCCode,
			rule.DPCCode,
		); err != nil {
			return fmt.Errorf("insert sqlite fixture rule %s: %w", rule.ID, err)
		}

		for idx, condition := range rule.Conditions {
			conditionType, operator, valueText, valueNum, valueJSON, err := denormalizeCondition(condition)
			if err != nil {
				return fmt.Errorf("denormalize sqlite fixture condition %s[%d]: %w", rule.ID, idx, err)
			}
			if _, err := tx.Exec(
				`INSERT INTO rule_conditions(condition_id, rule_id, condition_type, operator, value_text, value_num, value_json, negated) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
				fmt.Sprintf("%s-%02d", rule.ID, idx+1),
				rule.ID,
				conditionType,
				operator,
				valueText,
				valueNum,
				valueJSON,
			); err != nil {
				return fmt.Errorf("insert sqlite fixture condition %s[%d]: %w", rule.ID, idx, err)
			}
		}
	}

	if _, err := tx.Exec(`INSERT INTO metadata(key, value) VALUES (?, ?)`, "rule_count", fmt.Sprintf("%d", len(ruleSet.Rules))); err != nil {
		return fmt.Errorf("insert sqlite fixture metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite fixture tx: %w", err)
	}
	return nil
}

func denormalizeCondition(condition domain.Condition) (string, string, any, any, any, error) {
	switch condition.Type {
	case "main_diagnosis", "sex":
		if len(condition.Values) != 1 {
			return "", "", nil, nil, nil, fmt.Errorf("%s requires a single value", condition.Type)
		}
		return condition.Type, "eq", condition.Values[0], nil, nil, nil
	case "procedures":
		valueJSON, err := json.Marshal(condition.Values)
		if err != nil {
			return "", "", nil, nil, nil, fmt.Errorf("encode procedure values: %w", err)
		}
		return "procedure", "in", nil, nil, string(valueJSON), nil
	case "diagnoses":
		valueJSON, err := json.Marshal(condition.Values)
		if err != nil {
			return "", "", nil, nil, nil, fmt.Errorf("encode diagnosis values: %w", err)
		}
		return "diagnosis", "in", nil, nil, string(valueJSON), nil
	case "comorbidities":
		valueJSON, err := json.Marshal(condition.Values)
		if err != nil {
			return "", "", nil, nil, nil, fmt.Errorf("encode comorbidity values: %w", err)
		}
		return "comorbidity", "in", nil, nil, string(valueJSON), nil
	case "age":
		if condition.IntValue == nil {
			return "", "", nil, nil, nil, fmt.Errorf("age requires int_value")
		}
		return "age", condition.Operator, nil, *condition.IntValue, nil, nil
	default:
		if len(condition.Values) == 1 {
			return condition.Type, condition.Operator, condition.Values[0], nil, nil, nil
		}
		if len(condition.Values) > 1 {
			valueJSON, err := json.Marshal(condition.Values)
			if err != nil {
				return "", "", nil, nil, nil, fmt.Errorf("encode fallback values: %w", err)
			}
			return condition.Type, condition.Operator, nil, nil, string(valueJSON), nil
		}
		if condition.IntValue != nil {
			return condition.Type, condition.Operator, nil, *condition.IntValue, nil, nil
		}
		return condition.Type, condition.Operator, nil, nil, nil, nil
	}
}
