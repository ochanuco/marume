package store

import "strings"

var sqliteToDomainConditionType = map[string]string{
	"procedure":   "procedures",
	"diagnosis":   "diagnoses",
	"comorbidity": "comorbidities",
}

var domainToSQLiteConditionType = map[string]string{
	"procedures":   "procedure",
	"diagnoses":    "diagnosis",
	"comorbidities": "comorbidity",
}

var sqliteToDomainConditionOperator = map[string]string{
	"eq": "equals",
	"in": "contains_any",
}

var domainToSQLiteConditionOperator = map[string]string{
	"equals":       "eq",
	"contains_any": "in",
}

func normalizeConditionType(value string) string {
	lower := strings.ToLower(value)
	if normalized, ok := sqliteToDomainConditionType[lower]; ok {
		return normalized
	}
	return lower
}

func normalizeConditionOperator(value string) string {
	lower := strings.ToLower(value)
	if normalized, ok := sqliteToDomainConditionOperator[lower]; ok {
		return normalized
	}
	return lower
}

// SQLiteConditionTypeFromDomain returns the SQLite snapshot representation for a domain condition type.
func SQLiteConditionTypeFromDomain(value string) string {
	lower := strings.ToLower(value)
	if normalized, ok := domainToSQLiteConditionType[lower]; ok {
		return normalized
	}
	return lower
}

// SQLiteConditionOperatorFromDomain returns the SQLite snapshot representation for a domain condition operator.
func SQLiteConditionOperatorFromDomain(value string) string {
	lower := strings.ToLower(value)
	if normalized, ok := domainToSQLiteConditionOperator[lower]; ok {
		return normalized
	}
	return lower
}
