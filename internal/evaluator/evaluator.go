package evaluator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/ochanuco/marume/internal/domain"
)

// ErrNoClassification indicates that no rule matched the given case input.
var ErrNoClassification = errors.New("no classification matched")

// ErrRuleDefinition indicates that a rule set contains unsupported or malformed rule definitions.
var ErrRuleDefinition = errors.New("rule definition error")

// RuleStore loads a rule set for a requested fiscal year.
type RuleStore interface {
	LoadRuleSet(ctx context.Context, fiscalYear int) (domain.RuleSet, error)
}

// Evaluator applies rule sets to case input and produces classify/explain results.
type Evaluator struct {
	store RuleStore
}

// New constructs an Evaluator with a non-nil RuleStore.
func New(store RuleStore) *Evaluator {
	if isNilRuleStore(store) {
		panic("evaluator: store cannot be nil")
	}
	return &Evaluator{store: store}
}

// ValidateRuleSet checks that a rule set is structurally evaluable before use.
func ValidateRuleSet(ruleSet domain.RuleSet) error {
	if ruleSet.FiscalYear <= 0 {
		return newRuleDefinitionError("INVALID_RULESET_FISCAL_YEAR", "ruleset の fiscal_year は 1 以上である必要があります", "ruleset fiscal_year must be >= 1")
	}
	if ruleSet.RuleVersion == "" {
		return newRuleDefinitionError("MISSING_RULESET_VERSION", "ruleset の rule_version は必須です", "ruleset rule_version is required")
	}
	for _, rule := range ruleSet.Rules {
		if rule.ID == "" {
			return fmt.Errorf("rule <unknown>: %w", newRuleDefinitionError("MISSING_RULE_ID", "rule の id は必須です", "rule id is required"))
		}
		if rule.DPCCode == "" {
			return fmt.Errorf("rule %s: %w", rule.ID, newRuleDefinitionError("MISSING_RULE_DPC_CODE", "rule の dpc_code は必須です", "rule dpc_code is required"))
		}
		if len(rule.Conditions) == 0 {
			return fmt.Errorf("rule %s: %w", rule.ID, newRuleDefinitionError("NO_CONDITIONS_DEFINED", "ルールに条件が定義されていません", "no conditions defined for rule"))
		}
		for _, condition := range rule.Conditions {
			if err := validateConditionDefinition(condition); err != nil {
				return fmt.Errorf("rule %s: %w", rule.ID, err)
			}
		}
	}
	return nil
}

// Classify evaluates all candidate rules and returns the highest-priority match when the rule set is valid.
func (e *Evaluator) Classify(ctx context.Context, input domain.CaseInput) (domain.ClassificationResult, error) {
	ruleSet, err := e.store.LoadRuleSet(ctx, input.FiscalYear)
	if err != nil {
		return domain.ClassificationResult{}, err
	}
	if err := ValidateRuleSet(ruleSet); err != nil {
		return domain.ClassificationResult{}, err
	}

	var bestMatch *domain.ClassificationResult
	var definitionErr error

	for _, rule := range sortRules(ruleSet.Rules) {
		if err := ctx.Err(); err != nil {
			return domain.ClassificationResult{}, err
		}
		matched, reasons, _, err := evaluateRule(input, rule)
		if err != nil {
			if definitionErr == nil {
				definitionErr = fmt.Errorf("rule %s: %w", rule.ID, err)
			}
			continue
		}
		if matched && bestMatch == nil {
			bestMatch = &domain.ClassificationResult{
				CaseID:        input.CaseID,
				DPCCode:       rule.DPCCode,
				Version:       ruleSet.RuleVersion,
				MatchedRuleID: rule.ID,
				Reasons:       reasons,
			}
		}
	}

	if definitionErr != nil {
		return domain.ClassificationResult{}, definitionErr
	}
	if bestMatch != nil {
		return *bestMatch, nil
	}

	return domain.ClassificationResult{}, ErrNoClassification
}

// Explain returns per-rule diagnostics while mirroring Classify's context and error behavior.
func (e *Evaluator) Explain(ctx context.Context, input domain.CaseInput) (domain.ExplainResult, error) {
	ruleSet, err := e.store.LoadRuleSet(ctx, input.FiscalYear)
	if err != nil {
		return domain.ExplainResult{}, err
	}
	if err := ValidateRuleSet(ruleSet); err != nil {
		return domain.ExplainResult{}, err
	}

	result := domain.ExplainResult{
		CandidateRules: make([]domain.CandidateExplain, 0, len(ruleSet.Rules)),
	}
	var definitionErr error

	for _, rule := range sortRules(ruleSet.Rules) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		matched, reasons, mismatch, err := evaluateRule(input, rule)
		if err != nil {
			if definitionErr == nil {
				definitionErr = fmt.Errorf("rule %s: %w", rule.ID, err)
			}
			continue
		}

		candidate := domain.CandidateExplain{
			RuleID:         rule.ID,
			Priority:       rule.Priority,
			DPCCode:        rule.DPCCode,
			Matched:        matched,
			MatchedReasons: reasons,
		}
		if !matched {
			candidate.UnmatchedReason = mismatch
		} else if result.SelectedRule == "" {
			result.SelectedRule = rule.ID
		}

		result.CandidateRules = append(result.CandidateRules, candidate)
	}

	if definitionErr != nil {
		return result, definitionErr
	}
	if result.SelectedRule == "" {
		return result, ErrNoClassification
	}

	return result, nil
}

// sortRules returns a stable priority order used by classify and explain.
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

// evaluateRule checks all conditions for one rule and returns the first mismatch when present.
func evaluateRule(input domain.CaseInput, rule domain.Rule) (bool, []domain.ReasonEntry, *domain.ReasonEntry, error) {
	if len(rule.Conditions) == 0 {
		return false, nil, nil, newRuleDefinitionError("NO_CONDITIONS_DEFINED", "ルールに条件が定義されていません", "no conditions defined for rule")
	}

	reasons := make([]domain.ReasonEntry, 0, len(rule.Conditions))

	for _, condition := range rule.Conditions {
		matched, reason, err := evaluateCondition(input, condition)
		if err != nil {
			return false, reasons, nil, err
		}
		if !matched {
			return false, reasons, &reason, nil
		}
		reasons = append(reasons, reason)
	}

	return true, reasons, nil, nil
}

// evaluateCondition evaluates one normalized rule condition against a case input.
func evaluateCondition(input domain.CaseInput, condition domain.Condition) (bool, domain.ReasonEntry, error) {
	if err := validateConditionDefinition(condition); err != nil {
		return false, domain.ReasonEntry{}, err
	}

	switch condition.Type {
	case "main_diagnosis":
		if input.MainDiagnosis == condition.Values[0] {
			return true, domain.ReasonEntry{
				Code:      "MAIN_DIAGNOSIS_MATCH",
				Message:   fmt.Sprintf("主傷病名が %s に一致しました", condition.Values[0]),
				MessageEN: fmt.Sprintf("main diagnosis matched %s", condition.Values[0]),
			}, nil
		}
		return false, domain.ReasonEntry{
			Code:      "MAIN_DIAGNOSIS_MISMATCH",
			Message:   fmt.Sprintf("主傷病名 %s は %s と一致しません", input.MainDiagnosis, condition.Values[0]),
			MessageEN: fmt.Sprintf("main diagnosis %s did not equal %s", input.MainDiagnosis, condition.Values[0]),
		}, nil
	case "diagnoses":
		return evaluateContainsAny("diagnoses", input.Diagnoses, condition)
	case "procedures":
		return evaluateContainsAny("procedures", input.Procedures, condition)
	case "comorbidities":
		return evaluateContainsAny("comorbidities", input.Comorbidities, condition)
	case "sex":
		if strings.EqualFold(input.Sex, condition.Values[0]) {
			return true, domain.ReasonEntry{
				Code:      "SEX_MATCH",
				Message:   fmt.Sprintf("性別が %s に一致しました", strings.ToUpper(condition.Values[0])),
				MessageEN: fmt.Sprintf("sex matched %s", strings.ToUpper(condition.Values[0])),
			}, nil
		}
		return false, domain.ReasonEntry{
			Code:      "SEX_MISMATCH",
			Message:   fmt.Sprintf("性別 %s は %s と一致しません", input.Sex, strings.ToUpper(condition.Values[0])),
			MessageEN: fmt.Sprintf("sex %s did not equal %s", input.Sex, strings.ToUpper(condition.Values[0])),
		}, nil
	case "age":
		if input.Age == nil {
			return false, domain.ReasonEntry{
				Code:      "AGE_MISSING",
				Message:   "年齢が入力されていません",
				MessageEN: "age is missing",
			}, nil
		}
		switch condition.Operator {
		case "gte":
			if *input.Age >= *condition.IntValue {
				return true, domain.ReasonEntry{
					Code:      "AGE_GTE_MATCH",
					Message:   fmt.Sprintf("年齢 %d は %d 以上です", *input.Age, *condition.IntValue),
					MessageEN: fmt.Sprintf("age %d >= %d", *input.Age, *condition.IntValue),
				}, nil
			}
			return false, domain.ReasonEntry{
				Code:      "AGE_GTE_MISMATCH",
				Message:   fmt.Sprintf("年齢 %d は %d 未満です", *input.Age, *condition.IntValue),
				MessageEN: fmt.Sprintf("age %d < %d", *input.Age, *condition.IntValue),
			}, nil
		case "lte":
			if *input.Age <= *condition.IntValue {
				return true, domain.ReasonEntry{
					Code:      "AGE_LTE_MATCH",
					Message:   fmt.Sprintf("年齢 %d は %d 以下です", *input.Age, *condition.IntValue),
					MessageEN: fmt.Sprintf("age %d <= %d", *input.Age, *condition.IntValue),
				}, nil
			}
			return false, domain.ReasonEntry{
				Code:      "AGE_LTE_MISMATCH",
				Message:   fmt.Sprintf("年齢 %d は %d を超えています", *input.Age, *condition.IntValue),
				MessageEN: fmt.Sprintf("age %d > %d", *input.Age, *condition.IntValue),
			}, nil
		default:
			return false, domain.ReasonEntry{}, newRuleDefinitionError("UNSUPPORTED_AGE_CONDITION", "年齢条件の定義が未対応です", "unsupported age condition")
		}
	default:
		return false, domain.ReasonEntry{}, newRuleDefinitionError("UNSUPPORTED_CONDITION_TYPE", fmt.Sprintf("条件種別 %q は未対応です", condition.Type), fmt.Sprintf("unsupported condition type %q", condition.Type))
	}
}

// evaluateContainsAny evaluates contains_any style conditions against a string slice field.
func evaluateContainsAny(label string, actual []string, condition domain.Condition) (bool, domain.ReasonEntry, error) {
	if condition.Operator != "contains_any" || len(condition.Values) == 0 {
		return false, domain.ReasonEntry{}, newRuleDefinitionError(strings.ToUpper(label)+"_CONDITION_UNSUPPORTED", containsAnyUnsupportedMessage(label), fmt.Sprintf("unsupported %s condition", label))
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		actualSet[value] = struct{}{}
	}
	for _, value := range condition.Values {
		if _, ok := actualSet[value]; ok {
			return true, domain.ReasonEntry{
				Code:      strings.ToUpper(singularReasonPrefix(label)) + "_MATCH",
				Message:   fmt.Sprintf("%sに %s が含まれています", labelJa(label), value),
				MessageEN: fmt.Sprintf("%s contains %s", label, value),
			}, nil
		}
	}
	return false, domain.ReasonEntry{
		Code:      strings.ToUpper(singularReasonPrefix(label)) + "_MISMATCH",
		Message:   fmt.Sprintf("%sに %s が含まれていません", labelJa(label), strings.Join(condition.Values, ", ")),
		MessageEN: fmt.Sprintf("%s did not contain any of %s", label, strings.Join(condition.Values, ", ")),
	}, nil
}

// isNilRuleStore detects typed nil implementations hidden behind the RuleStore interface.
func isNilRuleStore(store RuleStore) bool {
	if store == nil {
		return true
	}

	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
		return value.IsNil()
	default:
		return false
	}
}

// validateConditionDefinition rejects unsupported or malformed condition definitions.
func validateConditionDefinition(condition domain.Condition) error {
	switch condition.Type {
	case "main_diagnosis", "sex":
		if condition.Operator != "equals" || len(condition.Values) != 1 {
			return newRuleDefinitionError("INVALID_EQUALS_CONDITION", fmt.Sprintf("%s 条件は equals と単一値が必要です", condition.Type), fmt.Sprintf("%s condition requires equals and a single value", condition.Type))
		}
	case "diagnoses", "procedures", "comorbidities":
		if condition.Operator != "contains_any" || len(condition.Values) == 0 {
			return newRuleDefinitionError("INVALID_CONTAINS_ANY_CONDITION", fmt.Sprintf("%s 条件は contains_any と 1 件以上の値が必要です", condition.Type), fmt.Sprintf("%s condition requires contains_any and at least one value", condition.Type))
		}
	case "age":
		if condition.Operator != "gte" && condition.Operator != "lte" {
			return newRuleDefinitionError("INVALID_AGE_CONDITION", "年齢条件は gte か lte が必要です", "age condition requires gte or lte")
		}
		if condition.IntValue == nil {
			return newRuleDefinitionError("INVALID_AGE_CONDITION", "年齢条件には int_value が必要です", "age condition requires int_value")
		}
		if *condition.IntValue < 0 {
			return newRuleDefinitionError("INVALID_AGE_CONDITION", "年齢条件の int_value は 0 以上である必要があります", "age condition int_value must be >= 0")
		}
	default:
		return newRuleDefinitionError("UNSUPPORTED_CONDITION_TYPE", fmt.Sprintf("条件種別 %q は未対応です", condition.Type), fmt.Sprintf("unsupported condition type %q", condition.Type))
	}
	return nil
}

type ruleDefinitionError struct {
	reason domain.ReasonEntry
}

// Error returns the localized rule-definition error message.
func (e *ruleDefinitionError) Error() string {
	return e.reason.Message
}

// Unwrap exposes ErrRuleDefinition for errors.Is checks.
func (e *ruleDefinitionError) Unwrap() error {
	return ErrRuleDefinition
}

// Reason returns the structured explanation attached to a rule-definition error.
func (e *ruleDefinitionError) Reason() domain.ReasonEntry {
	return e.reason
}

// newRuleDefinitionError creates a structured rule-definition error with bilingual messages.
func newRuleDefinitionError(code, message, messageEN string) error {
	return &ruleDefinitionError{
		reason: domain.ReasonEntry{
			Code:      code,
			Message:   message,
			MessageEN: messageEN,
		},
	}
}

// labelJa returns the Japanese display label for a condition target.
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

// singularReasonPrefix normalizes plural field names for reason code generation.
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

// containsAnyUnsupportedMessage returns the localized unsupported-definition message for a field.
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
