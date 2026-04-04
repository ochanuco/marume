package evaluator_test

import (
	"context"
	"testing"

	"github.com/ochanuco/marume/internal/domain"
	"github.com/ochanuco/marume/internal/evaluator"
)

type stubStore struct {
	ruleSet domain.RuleSet
}

func (s stubStore) LoadRuleSet(_ context.Context, _ int) (domain.RuleSet, error) {
	return s.ruleSet, nil
}

func Test優先順位が最も高い一致ルールを採用する(t *testing.T) {
	engine := evaluator.New(stubStore{
		ruleSet: domain.RuleSet{
			FiscalYear:  2026,
			RuleVersion: "2026.0.0-poc",
			Rules: []domain.Rule{
				{
					ID:       "low-priority",
					Priority: 20,
					DPCCode:  "999999",
					Conditions: []domain.Condition{
						{Type: "main_diagnosis", Operator: "equals", Values: []string{"I219"}},
					},
				},
				{
					ID:       "selected",
					Priority: 10,
					DPCCode:  "040080xx99x0xx",
					Conditions: []domain.Condition{
						{Type: "main_diagnosis", Operator: "equals", Values: []string{"I219"}},
						{Type: "procedures", Operator: "contains_any", Values: []string{"K549"}},
					},
				},
			},
		},
	})

	result, err := engine.Classify(context.Background(), domain.CaseInput{
		CaseID:        "123",
		FiscalYear:    2026,
		MainDiagnosis: "I219",
		Procedures:    []string{"K549"},
	})
	if err != nil {
		t.Fatalf("分類でエラーが返りました: %v", err)
	}
	if result.MatchedRuleID != "selected" {
		t.Fatalf("採用ルールは selected を期待しましたが、実際は %s でした", result.MatchedRuleID)
	}
	if len(result.Reasons) != 2 {
		t.Fatalf("理由は 2 件を期待しましたが、実際は %d 件でした", len(result.Reasons))
	}
	if result.Reasons[0].Code != "MAIN_DIAGNOSIS_MATCH" {
		t.Fatalf("最初の理由コードは MAIN_DIAGNOSIS_MATCH を期待しましたが、実際は %s でした", result.Reasons[0].Code)
	}
	if result.Reasons[0].Message == "" {
		t.Fatal("最初の理由メッセージが空です")
	}
	if result.Reasons[0].MessageEN == "" {
		t.Fatal("最初の理由英語メッセージが空です")
	}
}

func Test一致するルールがない場合は分類不能エラーを返す(t *testing.T) {
	engine := evaluator.New(stubStore{
		ruleSet: domain.RuleSet{
			FiscalYear:  2026,
			RuleVersion: "2026.0.0-poc",
			Rules: []domain.Rule{
				{
					ID:       "candidate",
					Priority: 10,
					DPCCode:  "040080xx99x0xx",
					Conditions: []domain.Condition{
						{Type: "main_diagnosis", Operator: "equals", Values: []string{"I219"}},
						{Type: "procedures", Operator: "contains_any", Values: []string{"K549"}},
					},
				},
			},
		},
	})

	result, err := engine.Explain(context.Background(), domain.CaseInput{
		CaseID:        "123",
		FiscalYear:    2026,
		MainDiagnosis: "I219",
		Procedures:    []string{"OTHER"},
	})
	if err != evaluator.ErrNoClassification {
		t.Fatalf("ErrNoClassification を期待しましたが、実際は %v でした", err)
	}
	if len(result.CandidateRules) != 0 {
		t.Fatalf("分類不能時の結果は空を期待しましたが、実際は %#v でした", result)
	}
}
