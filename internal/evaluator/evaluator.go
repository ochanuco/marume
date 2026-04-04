package evaluator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ochanuco/marume/internal/domain"
)

var ErrNoClassification = errors.New("no classification matched")

type RuleStore interface {
	LoadRuleSet(ctx context.Context, fiscalYear int) (domain.RuleSet, error)
}

type Evaluator struct {
	store RuleStore
}

func New(store RuleStore) *Evaluator {
	return &Evaluator{store: store}
}

func (e *Evaluator) Classify(ctx context.Context, input domain.CaseInput) (domain.ClassificationResult, error) {
	ruleSet, err := e.store.LoadRuleSet(ctx, input.FiscalYear)
	if err != nil {
		return domain.ClassificationResult{}, err
	}

	for _, rule := range sortRules(ruleSet.Rules) {
		matched, reasons, err := evaluateRule(input, rule)
		if err != nil {
			return domain.ClassificationResult{}, err
		}
		if matched {
			return domain.ClassificationResult{
				CaseID:        input.CaseID,
				DPCCode:       rule.DPCCode,
				Version:       ruleSet.RuleVersion,
				MatchedRuleID: rule.ID,
				Reasons:       reasons,
			}, nil
		}
	}

	return domain.ClassificationResult{}, ErrNoClassification
}

func (e *Evaluator) Explain(ctx context.Context, input domain.CaseInput) (domain.ExplainResult, error) {
	ruleSet, err := e.store.LoadRuleSet(ctx, input.FiscalYear)
	if err != nil {
		return domain.ExplainResult{}, err
	}

	result := domain.ExplainResult{
		CandidateRules: make([]domain.CandidateExplain, 0, len(ruleSet.Rules)),
	}

	for _, rule := range sortRules(ruleSet.Rules) {
		matched, reasons, err := evaluateRule(input, rule)
		if err != nil {
			return domain.ExplainResult{}, err
		}

		candidate := domain.CandidateExplain{
			RuleID:         rule.ID,
			Priority:       rule.Priority,
			DPCCode:        rule.DPCCode,
			Matched:        matched,
			MatchedReasons: reasons,
		}
		if !matched {
			candidate.UnmatchedReason = firstMismatch(input, rule)
		} else if result.SelectedRule == "" {
			result.SelectedRule = rule.ID
		}

		result.CandidateRules = append(result.CandidateRules, candidate)
	}

	if result.SelectedRule == "" {
		return domain.ExplainResult{}, ErrNoClassification
	}

	return result, nil
}

func sortRules(rules []domain.Rule) []domain.Rule {
	cloned := append([]domain.Rule(nil), rules...)
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].Priority == cloned[j].Priority {
			return cloned[i].ID < cloned[j].ID
		}
		return cloned[i].Priority < cloned[j].Priority
	})
	return cloned
}

func firstMismatch(input domain.CaseInput, rule domain.Rule) *domain.ReasonEntry {
	for _, condition := range rule.Conditions {
		ok, reason := evaluateCondition(input, condition)
		if !ok {
			return &reason
		}
	}
	return nil
}

func evaluateRule(input domain.CaseInput, rule domain.Rule) (bool, []domain.ReasonEntry, error) {
	reasons := make([]domain.ReasonEntry, 0, len(rule.Conditions))

	for _, condition := range rule.Conditions {
		matched, reason := evaluateCondition(input, condition)
		if !matched {
			return false, reasons, nil
		}
		reasons = append(reasons, reason)
	}

	return true, reasons, nil
}

func evaluateCondition(input domain.CaseInput, condition domain.Condition) (bool, domain.ReasonEntry) {
	switch condition.Type {
	case "main_diagnosis":
		if condition.Operator != "equals" || len(condition.Values) != 1 {
			return false, unsupportedReason("UNSUPPORTED_MAIN_DIAGNOSIS_CONDITION", "主傷病名条件の定義が未対応です", "unsupported main_diagnosis condition")
		}
		if input.MainDiagnosis == condition.Values[0] {
			return true, domain.ReasonEntry{
				Code:      "MAIN_DIAGNOSIS_MATCH",
				Message:   fmt.Sprintf("主傷病名が %s に一致しました", condition.Values[0]),
				MessageEN: fmt.Sprintf("main diagnosis matched %s", condition.Values[0]),
			}
		}
		return false, domain.ReasonEntry{
			Code:      "MAIN_DIAGNOSIS_MISMATCH",
			Message:   fmt.Sprintf("主傷病名 %s は %s と一致しません", input.MainDiagnosis, condition.Values[0]),
			MessageEN: fmt.Sprintf("main diagnosis %s did not equal %s", input.MainDiagnosis, condition.Values[0]),
		}
	case "diagnoses":
		return evaluateContainsAny("diagnoses", input.Diagnoses, condition)
	case "procedures":
		return evaluateContainsAny("procedures", input.Procedures, condition)
	case "comorbidities":
		return evaluateContainsAny("comorbidities", input.Comorbidities, condition)
	case "sex":
		if condition.Operator != "equals" || len(condition.Values) != 1 {
			return false, unsupportedReason("UNSUPPORTED_SEX_CONDITION", "性別条件の定義が未対応です", "unsupported sex condition")
		}
		if strings.EqualFold(input.Sex, condition.Values[0]) {
			return true, domain.ReasonEntry{
				Code:      "SEX_MATCH",
				Message:   fmt.Sprintf("性別が %s に一致しました", strings.ToUpper(condition.Values[0])),
				MessageEN: fmt.Sprintf("sex matched %s", strings.ToUpper(condition.Values[0])),
			}
		}
		return false, domain.ReasonEntry{
			Code:      "SEX_MISMATCH",
			Message:   fmt.Sprintf("性別 %s は %s と一致しません", input.Sex, strings.ToUpper(condition.Values[0])),
			MessageEN: fmt.Sprintf("sex %s did not equal %s", input.Sex, strings.ToUpper(condition.Values[0])),
		}
	case "age":
		switch condition.Operator {
		case "gte":
			if input.Age >= condition.IntValue {
				return true, domain.ReasonEntry{
					Code:      "AGE_GTE_MATCH",
					Message:   fmt.Sprintf("年齢 %d は %d 以上です", input.Age, condition.IntValue),
					MessageEN: fmt.Sprintf("age %d >= %d", input.Age, condition.IntValue),
				}
			}
			return false, domain.ReasonEntry{
				Code:      "AGE_GTE_MISMATCH",
				Message:   fmt.Sprintf("年齢 %d は %d 未満です", input.Age, condition.IntValue),
				MessageEN: fmt.Sprintf("age %d < %d", input.Age, condition.IntValue),
			}
		case "lte":
			if input.Age <= condition.IntValue {
				return true, domain.ReasonEntry{
					Code:      "AGE_LTE_MATCH",
					Message:   fmt.Sprintf("年齢 %d は %d 以下です", input.Age, condition.IntValue),
					MessageEN: fmt.Sprintf("age %d <= %d", input.Age, condition.IntValue),
				}
			}
			return false, domain.ReasonEntry{
				Code:      "AGE_LTE_MISMATCH",
				Message:   fmt.Sprintf("年齢 %d は %d を超えています", input.Age, condition.IntValue),
				MessageEN: fmt.Sprintf("age %d > %d", input.Age, condition.IntValue),
			}
		default:
			return false, unsupportedReason("UNSUPPORTED_AGE_CONDITION", "年齢条件の定義が未対応です", "unsupported age condition")
		}
	default:
		return false, unsupportedReason("UNSUPPORTED_CONDITION_TYPE", fmt.Sprintf("条件種別 %q は未対応です", condition.Type), fmt.Sprintf("unsupported condition type %q", condition.Type))
	}
}

func evaluateContainsAny(label string, actual []string, condition domain.Condition) (bool, domain.ReasonEntry) {
	if condition.Operator != "contains_any" || len(condition.Values) == 0 {
		return false, unsupportedReason(strings.ToUpper(label)+"_CONDITION_UNSUPPORTED", containsAnyUnsupportedMessage(label), fmt.Sprintf("unsupported %s condition", label))
	}
	for _, value := range condition.Values {
		if slices.Contains(actual, value) {
			return true, domain.ReasonEntry{
				Code:      strings.ToUpper(singularReasonPrefix(label)) + "_MATCH",
				Message:   fmt.Sprintf("%sに %s が含まれています", labelJa(label), value),
				MessageEN: fmt.Sprintf("%s contains %s", label, value),
			}
		}
	}
	return false, domain.ReasonEntry{
		Code:      strings.ToUpper(singularReasonPrefix(label)) + "_MISMATCH",
		Message:   fmt.Sprintf("%sに %s が含まれていません", labelJa(label), strings.Join(condition.Values, ", ")),
		MessageEN: fmt.Sprintf("%s did not contain any of %s", label, strings.Join(condition.Values, ", ")),
	}
}

func unsupportedReason(code, message, messageEN string) domain.ReasonEntry {
	return domain.ReasonEntry{
		Code:      code,
		Message:   message,
		MessageEN: messageEN,
	}
}

func labelJa(label string) string {
	switch label {
	case "diagnoses":
		return "診断名"
	case "procedures":
		return "手術・処置コード"
	case "comorbidities":
		return "併存症"
	default:
		return label
	}
}

func singularReasonPrefix(label string) string {
	switch label {
	case "diagnoses":
		return "diagnosis"
	case "procedures":
		return "procedure"
	case "comorbidities":
		return "comorbidity"
	default:
		return label
	}
}

func containsAnyUnsupportedMessage(label string) string {
	switch label {
	case "diagnoses":
		return "診断名条件の定義が未対応です"
	case "procedures":
		return "手術・処置条件の定義が未対応です"
	case "comorbidities":
		return "併存症条件の定義が未対応です"
	default:
		return fmt.Sprintf("%s 条件の定義が未対応です", label)
	}
}
