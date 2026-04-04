package domain

// CaseInput is a single case payload passed to the classifier.
type CaseInput struct {
	CaseID        string   `json:"case_id"`
	FiscalYear    int      `json:"fiscal_year"`
	Age           *int     `json:"age"`
	Sex           string   `json:"sex"`
	MainDiagnosis string   `json:"main_diagnosis"`
	Diagnoses     []string `json:"diagnoses"`
	Procedures    []string `json:"procedures"`
	Comorbidities []string `json:"comorbidities"`
}

// Condition is one rule predicate evaluated against a case.
type Condition struct {
	Type        string   `json:"type"`
	Operator    string   `json:"operator"`
	Values      []string `json:"values,omitempty"`
	IntValue    *int     `json:"int_value,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Rule is one DPC診断群分類 rule with priority and conditions.
type Rule struct {
	ID         string      `json:"id"`
	Priority   int         `json:"priority"`
	DPCCode    string      `json:"dpc_code"`
	Conditions []Condition `json:"conditions"`
}

// RuleSet is the versioned collection of rules loaded by the CLI.
type RuleSet struct {
	FiscalYear  int    `json:"fiscal_year"`
	RuleVersion string `json:"rule_version"`
	BuildID     string `json:"build_id"`
	// NOTE: POCではスナップショット入力の柔軟性を優先して string のままにしている。
	// 時刻演算や厳密な検証が必要になった段階で time.Time への移行を検討する。
	BuiltAt string `json:"built_at"`
	Rules   []Rule `json:"rules"`
}

// ClassificationResult is the machine-readable output of classify.
type ClassificationResult struct {
	CaseID        string        `json:"case_id"`
	DPCCode       string        `json:"dpc_code"`
	Version       string        `json:"version"`
	MatchedRuleID string        `json:"matched_rule_id"`
	Reasons       []ReasonEntry `json:"reasons"`
}

// ExplainResult is the machine-readable output of explain.
type ExplainResult struct {
	SelectedRule   string             `json:"selected_rule"`
	CandidateRules []CandidateExplain `json:"candidate_rules"`
}

// CandidateExplain describes how one candidate rule evaluated.
type CandidateExplain struct {
	RuleID          string        `json:"rule_id"`
	Priority        int           `json:"priority"`
	DPCCode         string        `json:"dpc_code"`
	Matched         bool          `json:"matched"`
	MatchedReasons  []ReasonEntry `json:"matched_reasons,omitempty"`
	UnmatchedReason *ReasonEntry  `json:"unmatched_reason,omitempty"`
}

// ReasonEntry is a structured explanation with stable code and localized text.
type ReasonEntry struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	MessageEN string `json:"message_en,omitempty"`
}
