package domain

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

type Condition struct {
	Type        string   `json:"type"`
	Operator    string   `json:"operator"`
	Values      []string `json:"values,omitempty"`
	IntValue    *int     `json:"int_value,omitempty"`
	Description string   `json:"description,omitempty"`
}

type Rule struct {
	ID         string      `json:"id"`
	Priority   int         `json:"priority"`
	DPCCode    string      `json:"dpc_code"`
	Conditions []Condition `json:"conditions"`
}

type RuleSet struct {
	FiscalYear  int    `json:"fiscal_year"`
	RuleVersion string `json:"rule_version"`
	BuildID     string `json:"build_id"`
	BuiltAt     string `json:"built_at"`
	Rules       []Rule `json:"rules"`
}

type ClassificationResult struct {
	CaseID        string        `json:"case_id"`
	DPCCode       string        `json:"dpc_code"`
	Version       string        `json:"version"`
	MatchedRuleID string        `json:"matched_rule_id"`
	Reasons       []ReasonEntry `json:"reasons"`
}

type ExplainResult struct {
	SelectedRule   string             `json:"selected_rule"`
	CandidateRules []CandidateExplain `json:"candidate_rules"`
}

type CandidateExplain struct {
	RuleID          string        `json:"rule_id"`
	Priority        int           `json:"priority"`
	DPCCode         string        `json:"dpc_code"`
	Matched         bool          `json:"matched"`
	MatchedReasons  []ReasonEntry `json:"matched_reasons,omitempty"`
	UnmatchedReason *ReasonEntry  `json:"unmatched_reason,omitempty"`
}

type ReasonEntry struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	MessageEN string `json:"message_en,omitempty"`
}
