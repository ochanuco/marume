package cli

import (
	"bytes"
	"encoding/json"
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

func TestWriteErrorJSONは機械可読なエラーを返す(t *testing.T) {
	var stderr bytes.Buffer

	err := WriteErrorJSON(&stderr, store.FiscalYearMismatchError{
		RuleSetFiscalYear: 2026,
		RequestedYear:     2027,
	})
	if err != nil {
		t.Fatalf("WriteErrorJSON でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stderr.Bytes(), &result); decodeErr != nil {
		t.Fatalf("JSONエラー出力を読み取れませんでした: %v", decodeErr)
	}
	if result["exit_code"] != float64(1) {
		t.Fatalf("exit_code は 1 を期待しましたが、実際は %v でした", result["exit_code"])
	}
	errorValue, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("error オブジェクトを期待しましたが、実際は %v でした", result["error"])
	}
	if errorValue["code"] != "FISCAL_YEAR_MISMATCH" {
		t.Fatalf("error.code は FISCAL_YEAR_MISMATCH を期待しましたが、実際は %v でした", errorValue["code"])
	}
}

func TestWriteErrorJSONはnilエラーなら何も出力しない(t *testing.T) {
	var stderr bytes.Buffer

	err := WriteErrorJSON(&stderr, nil)
	if err != nil {
		t.Fatalf("WriteErrorJSON(nil) でエラーが返りました: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("WriteErrorJSON(nil) は no-op を期待しましたが、実際は %q が出力されました", stderr.String())
	}
}

func TestJSONErrorsEnabledはグローバルフラグを検出する(t *testing.T) {
	if !JSONErrorsEnabled([]string{"--json-errors", "classify", "--input", "-"}) {
		t.Fatal("JSONErrorsEnabled は --json-errors を検出する想定でした")
	}
	if !JSONErrorsEnabled([]string{"--json-errors=true", "classify", "--input", "-"}) {
		t.Fatal("JSONErrorsEnabled は --json-errors=true を検出する想定でした")
	}
	if JSONErrorsEnabled([]string{"--json-errors=false", "classify", "--input", "-"}) {
		t.Fatal("JSONErrorsEnabled は --json-errors=false で false を返す想定でした")
	}
	if JSONErrorsEnabled([]string{"classify", "--input", "-"}) {
		t.Fatal("JSONErrorsEnabled はフラグなしで false を返す想定でした")
	}
	if JSONErrorsEnabled([]string{"classify", "--input", "--json-errors"}) {
		t.Fatal("JSONErrorsEnabled はサブコマンド以降の値をグローバルフラグとして扱わない想定でした")
	}
}

func TestStripGlobalFlagsは先頭のグローバルフラグだけを取り除く(t *testing.T) {
	args := stripGlobalFlags([]string{"--json-errors", "classify", "--input", "--json-errors"})
	expected := []string{"classify", "--input", "--json-errors"}
	if len(args) != len(expected) {
		t.Fatalf("stripGlobalFlags の結果長が想定と異なります: %v", args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("stripGlobalFlags[%d] は %q を期待しましたが、実際は %q でした", i, expected[i], args[i])
		}
	}
}

func TestStripGlobalFlagsはbool代入形式の先頭グローバルフラグも取り除く(t *testing.T) {
	args := stripGlobalFlags([]string{"--json-errors=false", "classify", "--input", "-"})
	expected := []string{"classify", "--input", "-"}
	if len(args) != len(expected) {
		t.Fatalf("stripGlobalFlags の結果長が想定と異なります: %v", args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("stripGlobalFlags[%d] は %q を期待しましたが、実際は %q でした", i, expected[i], args[i])
		}
	}
}

func TestRunCapabilitiesは実行時と同じ既定rulesパスを広告する(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runCapabilities(nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCapabilities でエラーが返りました: %v", err)
	}

	var result struct {
		DefaultRulePath string              `json:"default_rule_path"`
		GlobalFlags     []capabilityFlag    `json:"global_flags"`
		Commands        []capabilityCommand `json:"commands"`
		Schemas         []string            `json:"schemas"`
	}
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("capabilities のJSON出力を読み取れませんでした: %v", decodeErr)
	}

	expected := resolvedDefaultRulesPath()
	if result.DefaultRulePath != expected {
		t.Fatalf("default_rule_path は %q を期待しましたが、実際は %q でした", expected, result.DefaultRulePath)
	}

	foundGlobalJSONErrors := false
	for _, flag := range result.GlobalFlags {
		if flag.Name != "--json-errors" {
			continue
		}
		foundGlobalJSONErrors = true
		defaultValue, ok := flag.Default.(bool)
		if !ok || defaultValue {
			t.Fatalf("global --json-errors default は false を期待しましたが、実際は %#v でした", flag.Default)
		}
	}
	if !foundGlobalJSONErrors {
		t.Fatal("global_flags に --json-errors がありません")
	}

	schemaNames := make(map[string]struct{}, len(result.Schemas))
	for _, schemaName := range result.Schemas {
		schemaNames[schemaName] = struct{}{}
	}
	for _, command := range result.Commands {
		if command.OutputSchema == "" {
			continue
		}
		if _, ok := schemaNames[command.OutputSchema]; !ok {
			t.Fatalf("%s の output_schema %q が schemas にありません", command.Name, command.OutputSchema)
		}
	}

	commandChecks := []struct {
		name           string
		expectRules    bool
		expectBoolFlag string
	}{
		{name: "classify", expectRules: true},
		{name: "classify-batch", expectRules: true},
		{name: "explain", expectRules: true},
		{name: "version", expectRules: true},
		{name: "schema", expectBoolFlag: "--list"},
	}

	for _, check := range commandChecks {
		var command *capabilityCommand
		for i := range result.Commands {
			if result.Commands[i].Name == check.name {
				command = &result.Commands[i]
				break
			}
		}
		if command == nil {
			t.Fatalf("%s が capabilities にありません", check.name)
		}

		foundRulesFlag := false
		foundBoolDefault := false
		for _, flag := range command.Flags {
			if flag.Name == "--rules" {
				foundRulesFlag = true
				if flag.Default != expected {
					t.Fatalf("%s の --rules default は %q を期待しましたが、実際は %q でした", command.Name, expected, flag.Default)
				}
			}
			if check.expectBoolFlag != "" && flag.Name == check.expectBoolFlag {
				foundBoolDefault = true
				defaultValue, ok := flag.Default.(bool)
				if !ok || defaultValue {
					t.Fatalf("%s の %s default は false を期待しましたが、実際は %#v でした", command.Name, flag.Name, flag.Default)
				}
			}
		}
		if check.expectRules && !foundRulesFlag {
			t.Fatalf("%s に --rules フラグがありません", command.Name)
		}
		if check.expectBoolFlag != "" && !foundBoolDefault {
			t.Fatalf("%s の bool default を検証できませんでした", command.Name)
		}
	}
}
