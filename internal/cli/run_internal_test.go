package cli

import (
	"testing"

	"github.com/ochanuco/marume/internal/domain"
	"github.com/ochanuco/marume/internal/evaluator"
)

func TestClassifyBatchErrorはルール定義エラーの英語メッセージを英語で返す(t *testing.T) {
	negative := -1
	err := evaluator.ValidateRuleSet(domain.RuleSet{
		FiscalYear: 2026,
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
