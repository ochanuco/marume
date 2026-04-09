package store_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ochanuco/marume/internal/domain"
	"github.com/ochanuco/marume/internal/store"
	"github.com/ochanuco/marume/internal/testutil"
	_ "modernc.org/sqlite"
)

func TestSQLiteRuleStoreはSQLiteからルールを正規化して読める(t *testing.T) {
	rulesPath := writeSQLiteFixture(t, "rules-2026.json")

	ruleStore, err := store.NewSQLiteRuleStore(rulesPath)
	if err != nil {
		t.Fatalf("SQLiteRuleStore の作成に失敗しました: %v", err)
	}

	ruleSet, err := ruleStore.ReadRuleSet(context.Background())
	if err != nil {
		t.Fatalf("SQLiteRuleStore の読み込みに失敗しました: %v", err)
	}

	if ruleSet.FiscalYear != 2026 {
		t.Fatalf("fiscal_year は 2026 を期待しましたが、実際は %d でした", ruleSet.FiscalYear)
	}
	if len(ruleSet.Rules) != 2 {
		t.Fatalf("rules は 2 件を期待しましたが、実際は %d 件でした", len(ruleSet.Rules))
	}
	if got := ruleSet.Rules[0].Conditions[0].Operator; got != "equals" {
		t.Fatalf("main_diagnosis の operator は equals を期待しましたが、実際は %q でした", got)
	}
	if got := ruleSet.Rules[0].Conditions[1].Type; got != "procedures" {
		t.Fatalf("procedure 条件は procedures に正規化されることを期待しましたが、実際は %q でした", got)
	}
	if got := ruleSet.Rules[1].Conditions[1].Operator; got != "gte" {
		t.Fatalf("age 条件の operator は gte を期待しましたが、実際は %q でした", got)
	}
	if ruleSet.Rules[1].Conditions[1].IntValue == nil || *ruleSet.Rules[1].Conditions[1].IntValue != 70 {
		t.Fatalf("age 条件の int_value は 70 を期待しましたが、実際は %+v でした", ruleSet.Rules[1].Conditions[1].IntValue)
	}
	if ruleSet.BuiltAt == "" {
		t.Fatal("built_at は SQLite から読めることを期待しましたが、空でした")
	}
}

func TestSQLiteRuleStoreは年度不一致で専用エラーを返す(t *testing.T) {
	rulesPath := writeSQLiteFixture(t, "rules-2026.json")

	ruleStore, err := store.NewSQLiteRuleStore(rulesPath)
	if err != nil {
		t.Fatalf("SQLiteRuleStore の作成に失敗しました: %v", err)
	}

	_, err = ruleStore.LoadRuleSet(context.Background(), 2027)
	if err == nil {
		t.Fatal("年度不一致では専用エラーを期待しましたが、エラーが返りませんでした")
	}
	if !errors.Is(err, store.ErrFiscalYearMismatch) {
		t.Fatalf("年度不一致は store.ErrFiscalYearMismatch を期待しましたが、実際は %v でした", err)
	}
}

func TestSQLiteRuleStoreは指定年度のruleSetを優先して読む(t *testing.T) {
	sqlitePath := filepath.Join(t.TempDir(), "rules.sqlite")
	ruleSet2026 := mustLoadRuleSetFixture(t, "rules-2026.json")
	if err := testutil.WriteSQLiteRuleSet(sqlitePath, ruleSet2026); err != nil {
		t.Fatalf("2026 fixture の作成に失敗しました: %v", err)
	}

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("SQLite fixture の再オープンに失敗しました: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`INSERT INTO rule_sets(rule_set_id, fiscal_year, rule_version, build_id, built_at) VALUES (?, ?, ?, ?, ?)`, "dpc-2027", 2027, "2027.0.0-poc", "build-2027", "2026-04-05T00:00:00Z"); err != nil {
		t.Fatalf("2027 rule_set の追加に失敗しました: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rules(rule_id, rule_set_id, priority, dpc_code, label) VALUES (?, ?, ?, ?, ?)`, "R-2027-00010", "dpc-2027", 10, "040000xx99x0xx", "040000xx99x0xx"); err != nil {
		t.Fatalf("2027 rule の追加に失敗しました: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rule_conditions(condition_id, rule_id, condition_type, operator, value_text, value_num, value_json, negated) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`, "R-2027-00010-01", "R-2027-00010", "main_diagnosis", "eq", "Z999", nil, nil); err != nil {
		t.Fatalf("2027 condition の追加に失敗しました: %v", err)
	}

	ruleStore, err := store.NewSQLiteRuleStore(sqlitePath)
	if err != nil {
		t.Fatalf("SQLiteRuleStore の作成に失敗しました: %v", err)
	}

	ruleSet, err := ruleStore.LoadRuleSet(context.Background(), 2026)
	if err != nil {
		t.Fatalf("2026 の読み込みに失敗しました: %v", err)
	}
	if ruleSet.FiscalYear != 2026 {
		t.Fatalf("指定年度 2026 を期待しましたが、実際は %d でした", ruleSet.FiscalYear)
	}
	if len(ruleSet.Rules) != 2 {
		t.Fatalf("2026 rule 数は 2 件を期待しましたが、実際は %d 件でした", len(ruleSet.Rules))
	}
}

func TestSQLiteRuleStoreは同一年複数PDF版でもReadとLoadで最新PDF由来のruleSetを選ぶ(t *testing.T) {
	sqlitePath := writeSQLiteFixture(t, "rules-2026.json")

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("SQLite fixture の再オープンに失敗しました: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`INSERT INTO rule_sets(rule_set_id, fiscal_year, rule_version, source_published_at, build_id, built_at) VALUES (?, ?, ?, ?, ?, ?)`, "zzz-older-pdf", 2026, "2026.20260318", "2026-03-18", "build-2026-old-pdf", "2026-04-05T00:00:00Z"); err != nil {
		t.Fatalf("旧 PDF 版の rule_set 作成に失敗しました: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rules(rule_id, rule_set_id, priority, dpc_code, label) VALUES (?, ?, ?, ?, ?)`, "R-2026-OLDPDF-00010", "zzz-older-pdf", 10, "111111xx99x0xx", "111111xx99x0xx"); err != nil {
		t.Fatalf("旧 PDF 版の rule 作成に失敗しました: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rule_conditions(condition_id, rule_id, condition_type, operator, value_text, value_num, value_json, negated) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`, "R-2026-OLDPDF-00010-01", "R-2026-OLDPDF-00010", "main_diagnosis", "eq", "Z998", nil, nil); err != nil {
		t.Fatalf("旧 PDF 版の condition 作成に失敗しました: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO rule_sets(rule_set_id, fiscal_year, rule_version, source_published_at, build_id, built_at) VALUES (?, ?, ?, ?, ?, ?)`, "aaa-newer-pdf", 2026, "2026.20260325", "2026-03-25", "build-2026-new-pdf", "2026-04-06T00:00:00Z"); err != nil {
		t.Fatalf("新 PDF 版の rule_set 作成に失敗しました: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rules(rule_id, rule_set_id, priority, dpc_code, label) VALUES (?, ?, ?, ?, ?)`, "R-2026-NEWPDF-00010", "aaa-newer-pdf", 10, "999999xx99x0xx", "999999xx99x0xx"); err != nil {
		t.Fatalf("新 PDF 版の rule 作成に失敗しました: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rule_conditions(condition_id, rule_id, condition_type, operator, value_text, value_num, value_json, negated) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`, "R-2026-NEWPDF-00010-01", "R-2026-NEWPDF-00010", "main_diagnosis", "eq", "Z999", nil, nil); err != nil {
		t.Fatalf("新 PDF 版の condition 作成に失敗しました: %v", err)
	}

	ruleStore, err := store.NewSQLiteRuleStore(sqlitePath)
	if err != nil {
		t.Fatalf("SQLiteRuleStore の作成に失敗しました: %v", err)
	}

	readRuleSet, err := ruleStore.ReadRuleSet(context.Background())
	if err != nil {
		t.Fatalf("ReadRuleSet に失敗しました: %v", err)
	}

	loadRuleSet, err := ruleStore.LoadRuleSet(context.Background(), 2026)
	if err != nil {
		t.Fatalf("LoadRuleSet(2026) に失敗しました: %v", err)
	}

	if readRuleSet.BuildID != "build-2026-new-pdf" {
		t.Fatalf("ReadRuleSet は最新 PDF 版を返すことを期待しましたが、実際は %q でした", readRuleSet.BuildID)
	}
	if loadRuleSet.BuildID != "build-2026-new-pdf" {
		t.Fatalf("LoadRuleSet(2026) は最新 PDF 版を返すことを期待しましたが、実際は %q でした", loadRuleSet.BuildID)
	}
	if readRuleSet.BuildID != loadRuleSet.BuildID {
		t.Fatalf("ReadRuleSet と LoadRuleSet は同じ build を返すことを期待しましたが、実際は %q と %q でした", readRuleSet.BuildID, loadRuleSet.BuildID)
	}
	if len(readRuleSet.Rules) != 1 || len(loadRuleSet.Rules) != 1 {
		t.Fatalf("最新 PDF 版の rule 数は 1 件を期待しましたが、実際は read=%d load=%d でした", len(readRuleSet.Rules), len(loadRuleSet.Rules))
	}
	if readRuleSet.Rules[0].ID != "R-2026-NEWPDF-00010" || loadRuleSet.Rules[0].ID != "R-2026-NEWPDF-00010" {
		t.Fatalf("ReadRuleSet と LoadRuleSet は最新 PDF 版の rule を返すことを期待しましたが、実際は read=%q load=%q でした", readRuleSet.Rules[0].ID, loadRuleSet.Rules[0].ID)
	}
}

func TestNewRuleStoreはSQLite拡張子からSQLiteRuleStoreを選ぶ(t *testing.T) {
	// NewRuleStore は拡張子による選択だけを担い、このテストでは存在確認までは求めない。
	ruleStore, err := store.NewRuleStore("rules/rules-2026.sqlite")
	if err != nil {
		t.Fatalf("NewRuleStore の作成に失敗しました: %v", err)
	}

	if _, ok := ruleStore.(*store.SQLiteRuleStore); !ok {
		t.Fatalf("SQLite 拡張子では *store.SQLiteRuleStore を期待しましたが、実際は %T でした", ruleStore)
	}
}

func TestSQLiteRuleStoreは存在しないファイルをosErrNotExistで返す(t *testing.T) {
	ruleStore, err := store.NewSQLiteRuleStore(filepath.Join(t.TempDir(), "missing.sqlite"))
	if err != nil {
		t.Fatalf("SQLiteRuleStore の作成に失敗しました: %v", err)
	}

	_, err = ruleStore.ReadRuleSet(context.Background())
	if err == nil {
		t.Fatal("存在しない SQLite ではエラーを期待しましたが、返りませんでした")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("存在しない SQLite は os.ErrNotExist を期待しましたが、実際は %v でした", err)
	}
}

func writeSQLiteFixture(t *testing.T, fixtureName string) string {
	t.Helper()

	sqlitePath := filepath.Join(t.TempDir(), "rules.sqlite")
	ruleSet := mustLoadRuleSetFixture(t, fixtureName)
	if err := testutil.WriteSQLiteRuleSet(sqlitePath, ruleSet); err != nil {
		t.Fatalf("SQLite fixture の作成に失敗しました: %v", err)
	}
	return sqlitePath
}

func mustLoadRuleSetFixture(t *testing.T, fixtureName string) domain.RuleSet {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("テストファイルのパス解決に失敗しました")
	}

	jsonPath := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "rules", fixtureName)
	ruleSet, err := testutil.LoadRuleSetJSON(jsonPath)
	if err != nil {
		t.Fatalf("JSON fixture の読込に失敗しました: %v", err)
	}
	return ruleSet
}
