package cli

import (
	"testing"

	"github.com/ochanuco/marume/internal/domain"
	"github.com/ochanuco/marume/internal/evaluator"
	"github.com/ochanuco/marume/internal/store"
)

func TestClassifyBatchErrorはルール定義エラーの英語メッセージを英語で返す(t *testing.T) {
	negative := -1
	err := evaluator.ValidateRuleSet(domain.RuleSet{
		FiscalYear:  2026,
		RuleVersion: "2026.0.0-poc",
		Rules: []domain.Rule{
			{
				ID:       "invalid-age",
				Priority: 10,
				DPCCode:  "040080xx99x0xx",
				Conditions: []domain.Condition{
					{Type: "age", Operator: "gte", IntValue: &negative},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("不正な年齢条件でルール定義エラーを期待しましたが、エラーが返りませんでした")
	}

	result := classifyBatchError(err, "123")
	if result.Code != "RULE_DEFINITION_ERROR" {
		t.Fatalf("error.code は RULE_DEFINITION_ERROR を期待しましたが、実際は %s でした", result.Code)
	}
	if result.MessageEN != "rule definition error: age condition int_value must be >= 0" {
		t.Fatalf("error.message_en が想定と異なります: %s", result.MessageEN)
	}
}

func TestClassifyBatchErrorは年度不一致のメッセージを日英で分けて返す(t *testing.T) {
	err := store.FiscalYearMismatchError{
		RuleSetFiscalYear: 2026,
		RequestedYear:     2027,
	}

	result := classifyBatchError(err, "123")
	if result.Code != "FISCAL_YEAR_MISMATCH" {
		t.Fatalf("error.code は FISCAL_YEAR_MISMATCH を期待しましたが、実際は %s でした", result.Code)
	}
	if result.Message != "ルール年度と症例年度が一致しません: 2026 と 2027" {
		t.Fatalf("error.message が想定と異なります: %s", result.Message)
	}
	if result.MessageEN != "rule fiscal year does not match case fiscal year: 2026 vs 2027" {
		t.Fatalf("error.message_en が想定と異なります: %s", result.MessageEN)
	}
}
