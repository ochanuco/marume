package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ochanuco/marume/internal/store"
	"github.com/ochanuco/marume/internal/testutil"
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

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("テストファイルのパス解決に失敗しました")
	}

	jsonPath := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "rules", fixtureName)
	sqlitePath := filepath.Join(t.TempDir(), "rules.sqlite")
	if err := testutil.WriteSQLiteRuleSetFromJSON(jsonPath, sqlitePath); err != nil {
		t.Fatalf("SQLite fixture の作成に失敗しました: %v", err)
	}
	return sqlitePath
}
