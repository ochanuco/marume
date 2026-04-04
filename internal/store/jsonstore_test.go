package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ochanuco/marume/internal/store"
)

func TestLoadRuleSetは年度不一致で専用エラーを返す(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("テストファイルのパス解決に失敗しました")
	}
	rulesPath := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "rules", "rules-2026.json")

	ruleStore, err := store.NewJSONRuleStore(rulesPath)
	if err != nil {
		t.Fatalf("JSONRuleStore の作成に失敗しました: %v", err)
	}

	_, err = ruleStore.LoadRuleSet(context.Background(), 2027)
	if err == nil {
		t.Fatal("年度不一致では専用エラーを期待しましたが、エラーが返りませんでした")
	}
	if !errors.Is(err, store.ErrFiscalYearMismatch) {
		t.Fatalf("年度不一致は store.ErrFiscalYearMismatch を期待しましたが、実際は %v でした", err)
	}
}
