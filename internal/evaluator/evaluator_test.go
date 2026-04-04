package evaluator_test

import (
	"context"
	"errors"
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

type errorStore struct {
	err error
}

func (s errorStore) LoadRuleSet(_ context.Context, _ int) (domain.RuleSet, error) {
	return domain.RuleSet{}, s.err
}

func Test優先順位が最も高い一致ルールを採用する(t *testing.T) {
	age := 72
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
		Age:           &age,
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

func Testストアエラー時は分類を失敗させる(t *testing.T) {
	storeErr := errors.New("store unavailable")
	engine := evaluator.New(errorStore{err: storeErr})

	_, err := engine.Classify(context.Background(), domain.CaseInput{
		CaseID:     "123",
		FiscalYear: 2026,
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("ストアエラーが伝播されることを期待しましたが、実際は %v でした", err)
	}
}

func Test後続ルールの定義エラーを見逃さず分類を失敗させる(t *testing.T) {
	engine := evaluator.New(stubStore{
		ruleSet: domain.RuleSet{
			FiscalYear:  2026,
			RuleVersion: "2026.0.0-poc",
			Rules: []domain.Rule{
				{
					ID:       "selected",
					Priority: 10,
					DPCCode:  "040080xx99x0xx",
					Conditions: []domain.Condition{
						{Type: "main_diagnosis", Operator: "equals", Values: []string{"I219"}},
					},
				},
				{
					ID:       "broken",
					Priority: 20,
					DPCCode:  "999999",
					Conditions: []domain.Condition{
						{Type: "main_diagnosis", Operator: "contains_any", Values: []string{"I219"}},
					},
				},
			},
		},
	})

	_, err := engine.Classify(context.Background(), domain.CaseInput{
		CaseID:        "123",
		FiscalYear:    2026,
		MainDiagnosis: "I219",
	})
	if !errors.Is(err, evaluator.ErrRuleDefinition) {
		t.Fatalf("後続ルールの定義エラーで ErrRuleDefinition を期待しましたが、実際は %v でした", err)
	}
}

func Test一致するルールがない場合は分類不能エラーを返す(t *testing.T) {
	procedureAge := 75
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
		Age:           &procedureAge,
		MainDiagnosis: "I219",
		Procedures:    []string{"OTHER"},
	})
	if !errors.Is(err, evaluator.ErrNoClassification) {
		t.Fatalf("ErrNoClassification を期待しましたが、実際は %v でした", err)
	}
	if len(result.CandidateRules) != 1 {
		t.Fatalf("分類不能時も候補ルールは 1 件を期待しましたが、実際は %d 件でした", len(result.CandidateRules))
	}
	if result.CandidateRules[0].UnmatchedReason == nil {
		t.Fatal("分類不能時も不一致理由が保持されることを期待しましたが、nil でした")
	}
}

func Test未対応の条件定義はルール定義エラーとして返す(t *testing.T) {
	engine := evaluator.New(stubStore{
		ruleSet: domain.RuleSet{
			FiscalYear:  2026,
			RuleVersion: "2026.0.0-poc",
			Rules: []domain.Rule{
				{
					ID:       "broken",
					Priority: 10,
					DPCCode:  "040080xx99x0xx",
					Conditions: []domain.Condition{
						{Type: "main_diagnosis", Operator: "contains_any", Values: []string{"I219"}},
					},
				},
			},
		},
	})

	_, err := engine.Classify(context.Background(), domain.CaseInput{
		CaseID:        "123",
		FiscalYear:    2026,
		MainDiagnosis: "I219",
	})
	if !errors.Is(err, evaluator.ErrRuleDefinition) {
		t.Fatalf("ErrRuleDefinition を期待しましたが、実際は %v でした", err)
	}
}

func Testルールセット検証で未対応Typeを事前に検出する(t *testing.T) {
	ruleSet := domain.RuleSet{
		FiscalYear: 2026,
		Rules: []domain.Rule{
			{
				ID:       "broken",
				Priority: 10,
				DPCCode:  "040080xx99x0xx",
				Conditions: []domain.Condition{
					{Type: "unsupported_type", Operator: "equals", Values: []string{"72"}},
				},
			},
		},
	}

	err := evaluator.ValidateRuleSet(ruleSet)
	if !errors.Is(err, evaluator.ErrRuleDefinition) {
		t.Fatalf("ErrRuleDefinition を期待しましたが、実際は %v でした", err)
	}
}

func Test条件ゼロのルールはルール定義エラーとして拒否する(t *testing.T) {
	ruleSet := domain.RuleSet{
		FiscalYear: 2026,
		Rules: []domain.Rule{
			{
				ID:         "empty",
				Priority:   10,
				DPCCode:    "040080xx99x0xx",
				Conditions: nil,
			},
		},
	}

	err := evaluator.ValidateRuleSet(ruleSet)
	if !errors.Is(err, evaluator.ErrRuleDefinition) {
		t.Fatalf("nil 条件で ErrRuleDefinition を期待しましたが、実際は %v でした", err)
	}

	ruleSet.Rules[0].Conditions = []domain.Condition{}
	err = evaluator.ValidateRuleSet(ruleSet)
	if !errors.Is(err, evaluator.ErrRuleDefinition) {
		t.Fatalf("空スライス条件でも ErrRuleDefinition を期待しましたが、実際は %v でした", err)
	}
}

func Test年齢条件の負のintValueはルール定義エラーとして拒否する(t *testing.T) {
	negative := -1
	ruleSet := domain.RuleSet{
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
	}

	err := evaluator.ValidateRuleSet(ruleSet)
	if !errors.Is(err, evaluator.ErrRuleDefinition) {
		t.Fatalf("負の int_value で ErrRuleDefinition を期待しましたが、実際は %v でした", err)
	}
}
