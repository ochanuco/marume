package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ochanuco/marume/internal/cli"
	"github.com/ochanuco/marume/internal/store"
	"github.com/ochanuco/marume/internal/testutil"
)

func TestClassifyBatchはJSONLを1行ずつ処理する(t *testing.T) {
	inputPath := testdataPath(t, "cases", "cases.jsonl")
	rulesPath := testdataPath(t, "rules", "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", inputPath, "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("classify-batch でエラーが返りました: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("出力行数は 3 行を期待しましたが、実際は %d 行でした", len(lines))
	}

	var line1 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line1); err != nil {
		t.Fatalf("1 行目のJSONを読み取れませんでした: %v", err)
	}
	if line1["status"] != "ok" {
		t.Fatalf("1 行目は成功を期待しましたが、実際は %v でした", line1["status"])
	}
	result, ok := line1["result"].(map[string]any)
	if !ok {
		t.Fatalf("1 行目の result を期待しましたが、実際は %v でした", line1)
	}
	reasons, ok := result["reasons"].([]any)
	if !ok || len(reasons) == 0 {
		t.Fatalf("1 行目の reasons を期待しましたが、実際は %v でした", result["reasons"])
	}
	firstReason, ok := reasons[0].(map[string]any)
	if !ok {
		t.Fatalf("1 行目の最初の reason を期待しましたが、実際は %v でした", reasons[0])
	}
	if firstReason["message"] != "主傷病名が I219 に一致しました" {
		t.Fatalf("1 行目の理由メッセージが想定と異なります: %v", firstReason["message"])
	}

	var line2 map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &line2); err != nil {
		t.Fatalf("2 行目のJSONを読み取れませんでした: %v", err)
	}
	if line2["status"] != "error" {
		t.Fatalf("2 行目は error を期待しましたが、実際は %v でした", line2["status"])
	}
	line2Err, ok := line2["error"].(map[string]any)
	if !ok {
		t.Fatalf("2 行目の error を期待しましたが、実際は %v でした", line2["error"])
	}
	if line2Err["code"] != "NO_CLASSIFICATION" {
		t.Fatalf("2 行目は NO_CLASSIFICATION を期待しましたが、実際は %v でした", line2Err["code"])
	}

	var line3 map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &line3); err != nil {
		t.Fatalf("3 行目のJSONを読み取れませんでした: %v", err)
	}
	line3Err, ok := line3["error"].(map[string]any)
	if !ok {
		t.Fatalf("3 行目の error を期待しましたが、実際は %v でした", line3["error"])
	}
	if line3Err["code"] != "INVALID_JSON" {
		t.Fatalf("3 行目は INVALID_JSON を期待しましたが、実際は %v でした", line3Err["code"])
	}
}

func TestExplainは分類不能でも候補ルールをJSON出力する(t *testing.T) {
	input := `{"case_id":"999","fiscal_year":2026,"main_diagnosis":"Z999","diagnoses":["Z999"],"procedures":[],"comorbidities":[]}`
	rulesPath := testdataPath(t, "rules", "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"explain", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("explain でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("explain のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if _, ok := result["candidate_rules"]; !ok {
		t.Fatalf("candidate_rules を期待しましたが、実際の出力は %v でした", result)
	}
	selectedRule, _ := result["selected_rule"].(string)
	if selectedRule != "" {
		t.Fatalf("分類不能時の selected_rule は空文字を期待しましたが、実際は %v でした", selectedRule)
	}
}

func TestVersionは別年度ルールでもメタ情報を表示できる(t *testing.T) {
	rulesPath := testdataPath(t, "rules", "rules-2027.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"version", "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("version でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("version のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if result["rule_version"] != "2027.0.0-poc" {
		t.Fatalf("2027 年度の rule_version を期待しましたが、実際は %v でした", result["rule_version"])
	}
}

func TestClassifyはSQLiteスナップショットも読める(t *testing.T) {
	input := `{"case_id":"123","fiscal_year":2026,"main_diagnosis":"I219","diagnoses":["I219"],"procedures":["K549"],"comorbidities":[]}`
	rulesPath := sqliteRulesPath(t, "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("SQLite classify でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("SQLite classify のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if result["dpc_code"] != "040080xx99x0xx" {
		t.Fatalf("SQLite classify の dpc_code は 040080xx99x0xx を期待しましたが、実際は %v でした", result["dpc_code"])
	}
}

func TestExplainはSQLiteスナップショットでも候補ルールを返す(t *testing.T) {
	input := `{"case_id":"999","fiscal_year":2026,"main_diagnosis":"Z999","diagnoses":["Z999"],"procedures":[],"comorbidities":[]}`
	rulesPath := sqliteRulesPath(t, "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"explain", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("SQLite explain でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("SQLite explain のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if _, ok := result["candidate_rules"]; !ok {
		t.Fatalf("SQLite explain は candidate_rules を期待しましたが、実際は %v でした", result)
	}
}

func TestVersionはSQLiteスナップショットのメタ情報も表示できる(t *testing.T) {
	rulesPath := sqliteRulesPath(t, "rules-2027.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"version", "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("SQLite version でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("SQLite version のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if result["rule_version"] != "2027.0.0-poc" {
		t.Fatalf("SQLite version の rule_version は 2027.0.0-poc を期待しましたが、実際は %v でした", result["rule_version"])
	}
	builtAt, ok := result["built_at"].(string)
	if !ok || builtAt == "" {
		t.Fatalf("SQLite version の built_at は空でないことを期待しましたが、実際は %v でした", result["built_at"])
	}
}

func TestSchemaはcaseInputのJSONSchemaを返す(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"schema", "case-input"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("schema でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("schema のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if result["title"] != "Case Input" {
		t.Fatalf("schema の title が想定と異なります: %v", result["title"])
	}
	properties, ok := result["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema の properties を期待しましたが、実際は %T でした", result["properties"])
	}
	if _, ok := properties["case_id"]; !ok {
		t.Fatalf("schema に case_id プロパティがありません: %v", properties)
	}
	if result["additionalProperties"] != false {
		t.Fatalf("schema の additionalProperties は false を期待しましたが、実際は %v でした", result["additionalProperties"])
	}
	caseID, ok := properties["case_id"].(map[string]any)
	if !ok || caseID["minLength"] != float64(1) {
		t.Fatalf("case_id の minLength は 1 を期待しましたが、実際は %v でした", properties["case_id"])
	}
	fiscalYear, ok := properties["fiscal_year"].(map[string]any)
	if !ok || fiscalYear["minimum"] != float64(1) {
		t.Fatalf("fiscal_year の minimum は 1 を期待しましたが、実際は %v でした", properties["fiscal_year"])
	}
	age, ok := properties["age"].(map[string]any)
	if !ok || age["minimum"] != float64(0) {
		t.Fatalf("age の minimum は 0 を期待しましたが、実際は %v でした", properties["age"])
	}
}

func TestSchemaListは利用可能なスキーマ名を返す(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"schema", "--list"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("schema --list でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("schema --list のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	schemas, ok := result["schemas"].([]any)
	if !ok || len(schemas) == 0 {
		t.Fatalf("schema --list は schemas を返す想定でしたが、実際は %v でした", result["schemas"])
	}
}

func TestCapabilitiesはCLI契約のJSONを返す(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"capabilities"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("capabilities でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("capabilities のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if result["default_rule_path"] != "rules/rules-2026.sqlite" {
		t.Fatalf("default_rule_path が想定と異なります: %v", result["default_rule_path"])
	}
	commands, ok := result["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("commands を期待しましたが、実際は %v でした", result["commands"])
	}
	foundCapabilities := false
	for _, item := range commands {
		command, ok := item.(map[string]any)
		if ok && command["name"] == "capabilities" {
			foundCapabilities = true
			break
		}
	}
	if !foundCapabilities {
		t.Fatalf("capabilities コマンド自身が一覧にありません: %v", commands)
	}

	for _, item := range commands {
		command, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if command["name"] == "schema" {
			if _, exists := command["output_schema"]; exists {
				t.Fatalf("schema コマンドは取得不能な output_schema を広告しない想定でした: %v", command)
			}
		}
	}
}

func TestSchemaListは余分な引数を拒否する(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"schema", "--list", "extra"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("schema --list extra は入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("schema --list extra の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
}

func TestClassifyHelpは入力スキーマの要約を表示する(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify", "--help"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("classify --help でエラーが返りました: %v", err)
	}

	helpText := stderr.String()
	if !strings.Contains(helpText, "入力スキーマ:") {
		t.Fatalf("classify --help に入力スキーマ要約がありません: %s", helpText)
	}
	if !strings.Contains(helpText, "case_id (string, 必須)") {
		t.Fatalf("classify --help に case_id の説明がありません: %s", helpText)
	}
	if !strings.Contains(helpText, "marume schema classify-result") {
		t.Fatalf("classify --help に出力スキーマ導線がありません: %s", helpText)
	}
}

func TestTopLevelHelpはCapabilitiesとJSONErrorsを案内する(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"--help"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("--help でエラーが返りました: %v", err)
	}

	helpText := stdout.String()
	if !strings.Contains(helpText, "capabilities") {
		t.Fatalf("top-level help に capabilities がありません: %s", helpText)
	}
	if !strings.Contains(helpText, "--json-errors") {
		t.Fatalf("top-level help に --json-errors がありません: %s", helpText)
	}
}

func TestRunはJSONErrors単独でもpanicせず入力エラーを返す(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"--json-errors"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("--json-errors 単独では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("--json-errors 単独の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(stderr.String(), "使い方:") {
		t.Fatalf("--json-errors 単独でも usage を表示する想定でした: %s", stderr.String())
	}
}

func Test各サブコマンドは余分な引数を入力エラーにする(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "classify", args: []string{"classify", "--input", "-", "extra"}},
		{name: "classify-batch", args: []string{"classify-batch", "--input", "-", "extra"}},
		{name: "explain", args: []string{"explain", "--input", "-", "extra"}},
		{name: "schema", args: []string{"schema", "case-input", "extra"}},
		{name: "validate", args: []string{"validate", "--input", "-", "extra"}},
		{name: "version", args: []string{"version", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			err := cli.Run(
				context.Background(),
				tt.args,
				strings.NewReader("{}"),
				&stdout,
				&stderr,
			)
			if err == nil {
				t.Fatal("余分な引数では入力エラーを期待しましたが、エラーが返りませんでした")
			}
			if cli.ExitCode(err) != 1 {
				t.Fatalf("余分な引数の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
			}
			if !strings.Contains(err.Error(), "余分な引数があります") {
				t.Fatalf("余分な引数のエラーメッセージが想定と異なります: %v", err)
			}
		})
	}
}

func TestClassifyBatchは64KBを超える行も処理できる(t *testing.T) {
	rulesPath := testdataPath(t, "rules", "rules-2026.json")
	largeDiagnosis := strings.Repeat("A", 70*1024)
	input := fmt.Sprintf("{\"case_id\":\"large\",\"fiscal_year\":2026,\"main_diagnosis\":\"I219\",\"diagnoses\":[\"%s\"],\"procedures\":[\"K549\"],\"comorbidities\":[]}\n", largeDiagnosis)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("長いJSONL行の classify-batch でエラーが返りました: %v", err)
	}
	line := strings.TrimSpace(stdout.String())
	var result map[string]any
	if decodeErr := json.Unmarshal([]byte(line), &result); decodeErr != nil {
		t.Fatalf("長いJSONL行の結果をJSONとして読み取れませんでした: %v", decodeErr)
	}
	if result["status"] != "ok" {
		t.Fatalf("長いJSONL行でも成功を期待しましたが、実際の出力は %v でした", result["status"])
	}
}

func TestClassifyは年度不一致を入力エラーとして返す(t *testing.T) {
	input := `{"case_id":"123","fiscal_year":2027,"main_diagnosis":"I219","diagnoses":["I219"],"procedures":["K549"],"comorbidities":[]}`
	rulesPath := testdataPath(t, "rules", "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("年度不一致では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("年度不一致の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !errors.Is(err, store.ErrFiscalYearMismatch) {
		t.Fatalf("年度不一致は store.ErrFiscalYearMismatch を期待しましたが、実際は %v でした", err)
	}
}

func TestClassifyBatchは年度不一致を専用エラーコードで返す(t *testing.T) {
	input := "{\"case_id\":\"123\",\"fiscal_year\":2027,\"main_diagnosis\":\"I219\",\"diagnoses\":[\"I219\"],\"procedures\":[\"K549\"],\"comorbidities\":[]}\n"
	rulesPath := testdataPath(t, "rules", "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("年度不一致の classify-batch は行単位エラーを期待しましたが、実際は %v でした", err)
	}

	line := strings.TrimSpace(stdout.String())
	var result map[string]any
	if decodeErr := json.Unmarshal([]byte(line), &result); decodeErr != nil {
		t.Fatalf("年度不一致の結果をJSONとして読み取れませんでした: %v", decodeErr)
	}
	errorResult, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("年度不一致では error オブジェクトを期待しましたが、実際は %v でした", result)
	}
	if errorResult["code"] != "FISCAL_YEAR_MISMATCH" {
		t.Fatalf("年度不一致の error.code は FISCAL_YEAR_MISMATCH を期待しましたが、実際は %v でした", errorResult["code"])
	}
}

func TestClassifyは負の年齢を入力エラーとして返す(t *testing.T) {
	age := -1
	input := fmt.Sprintf(`{"case_id":"123","fiscal_year":2026,"age":%d,"main_diagnosis":"I219","diagnoses":["I219"],"procedures":["K549"],"comorbidities":[]}`, age)
	rulesPath := testdataPath(t, "rules", "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("負の年齢では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("負の年齢の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "age は負の値を指定できません") {
		t.Fatalf("負の年齢のエラーメッセージが想定と異なります: %v", err)
	}
}

func TestRulesPathが空文字なら入力エラーを返す(t *testing.T) {
	input := `{"case_id":"123","fiscal_year":2026,"main_diagnosis":"I219","diagnoses":["I219"],"procedures":["K549"],"comorbidities":[]}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify", "--input", "-", "--rules", ""},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("空の rules パスでは入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("空の rules パスの終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "path cannot be empty") {
		t.Fatalf("空の rules パスのエラーメッセージが想定と異なります: %v", err)
	}
}

func TestFiscalYearが負値なら入力エラーを返す(t *testing.T) {
	input := `{"case_id":"123","fiscal_year":-2026,"main_diagnosis":"I219","diagnoses":["I219"],"procedures":["K549"],"comorbidities":[]}`
	rulesPath := testdataPath(t, "rules", "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify", "--input", "-", "--rules", rulesPath},
		strings.NewReader(input),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("負の fiscal_year では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("負の fiscal_year の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
}

func TestVersionは必須メタデータ欠落を入力エラーにする(t *testing.T) {
	tmpDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "rules-missing-meta.json")
	rulesJSON := `{
  "fiscal_year": 2026,
  "rule_version": "",
  "build_id": "build-1",
  "built_at": "2026-01-01T00:00:00Z",
  "rules": [
    {
      "id": "R-1",
      "priority": 10,
      "dpc_code": "040080xx99x0xx",
      "conditions": [
        {
          "type": "main_diagnosis",
          "operator": "equals",
          "values": ["I219"]
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(rulesPath, []byte(rulesJSON), 0o644); err != nil {
		t.Fatalf("メタデータ欠落ルールの作成に失敗しました: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"version", "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("必須メタデータ欠落では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("必須メタデータ欠落の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "rule_version は必須です") {
		t.Fatalf("必須メタデータ欠落のエラーメッセージが想定と異なります: %v", err)
	}
}

func TestClassifyBatchはルール読み込み失敗時に既存出力を壊さない(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "result.jsonl")
	original := "keep-me\n"
	if err := os.WriteFile(outputPath, []byte(original), 0o644); err != nil {
		t.Fatalf("事前出力ファイルの作成に失敗しました: %v", err)
	}

	inputPath := testdataPath(t, "cases", "cases.jsonl")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", inputPath, "--output", outputPath, "--rules", filepath.Join(tmpDir, "missing-rules.json")},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("ルール読み込み失敗を期待しましたが、エラーが返りませんでした")
	}

	got, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("事後出力ファイルの読み込みに失敗しました: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("ルール読み込み失敗時は既存出力を保持したいですが、実際は %q でした", string(got))
	}
}

func TestClassifyBatchは入力と出力が同じファイルなら入力エラーを返して内容を壊さない(t *testing.T) {
	tmpDir := t.TempDir()
	ioPath := filepath.Join(tmpDir, "cases.jsonl")
	original := "{\"case_id\":\"123\",\"fiscal_year\":2026,\"main_diagnosis\":\"I219\",\"diagnoses\":[\"I219\"],\"procedures\":[\"K549\"],\"comorbidities\":[]}\n"
	if err := os.WriteFile(ioPath, []byte(original), 0o644); err != nil {
		t.Fatalf("入出力兼用ファイルの作成に失敗しました: %v", err)
	}

	rulesPath := testdataPath(t, "rules", "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", ioPath, "--output", ioPath, "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("入力と出力が同じファイルなら入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("入力と出力が同じファイルの終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "同じファイルは指定できません") {
		t.Fatalf("同一ファイル指定時のエラーメッセージが想定と異なります: %v", err)
	}

	got, readErr := os.ReadFile(ioPath)
	if readErr != nil {
		t.Fatalf("入出力兼用ファイルの再読込に失敗しました: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("同一ファイル指定時も元データは保持されることを期待しましたが、実際は %q でした", string(got))
	}
}

func TestClassifyBatchは別ファイル出力を許容する(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "cases.jsonl")
	outputPath := filepath.Join(tmpDir, "result.jsonl")
	input := "{\"case_id\":\"123\",\"fiscal_year\":2026,\"main_diagnosis\":\"I219\",\"diagnoses\":[\"I219\"],\"procedures\":[\"K549\"],\"comorbidities\":[]}\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("入力ファイルの作成に失敗しました: %v", err)
	}

	rulesPath := testdataPath(t, "rules", "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", inputPath, "--output", outputPath, "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("別ファイル出力は成功を期待しましたが、実際は %v でした", err)
	}

	got, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("出力ファイルの読込に失敗しました: %v", readErr)
	}
	if len(bytes.TrimSpace(got)) == 0 {
		t.Fatal("別ファイル出力では結果JSONLが書き込まれることを期待しましたが、空でした")
	}
}

func TestClassifyBatchはシンボリックリンク経由でも同じファイルを拒否する(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "cases.jsonl")
	outputPath := filepath.Join(tmpDir, "cases-link.jsonl")
	original := "{\"case_id\":\"123\",\"fiscal_year\":2026,\"main_diagnosis\":\"I219\",\"diagnoses\":[\"I219\"],\"procedures\":[\"K549\"],\"comorbidities\":[]}\n"
	if err := os.WriteFile(inputPath, []byte(original), 0o644); err != nil {
		t.Fatalf("入力ファイルの作成に失敗しました: %v", err)
	}
	if err := os.Symlink(inputPath, outputPath); err != nil {
		t.Fatalf("シンボリックリンクの作成に失敗しました: %v", err)
	}

	rulesPath := testdataPath(t, "rules", "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", inputPath, "--output", outputPath, "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("シンボリックリンク経由の同一ファイルでは入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("シンボリックリンク経由の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}

	got, readErr := os.ReadFile(inputPath)
	if readErr != nil {
		t.Fatalf("入力ファイルの再読込に失敗しました: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("シンボリックリンク経由でも元データは保持されることを期待しましたが、実際は %q でした", string(got))
	}
}

func TestClassifyBatchはハードリンク経由でも同じファイルを拒否する(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "cases.jsonl")
	outputPath := filepath.Join(tmpDir, "cases-hardlink.jsonl")
	original := "{\"case_id\":\"123\",\"fiscal_year\":2026,\"main_diagnosis\":\"I219\",\"diagnoses\":[\"I219\"],\"procedures\":[\"K549\"],\"comorbidities\":[]}\n"
	if err := os.WriteFile(inputPath, []byte(original), 0o644); err != nil {
		t.Fatalf("入力ファイルの作成に失敗しました: %v", err)
	}
	if err := os.Link(inputPath, outputPath); err != nil {
		t.Fatalf("ハードリンクの作成に失敗しました: %v", err)
	}

	rulesPath := testdataPath(t, "rules", "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"classify-batch", "--input", inputPath, "--output", outputPath, "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("ハードリンク経由の同一ファイルでは入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("ハードリンク経由の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}

	got, readErr := os.ReadFile(inputPath)
	if readErr != nil {
		t.Fatalf("入力ファイルの再読込に失敗しました: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("ハードリンク経由でも元データは保持されることを期待しましたが、実際は %q でした", string(got))
	}
}

func testdataPath(t *testing.T, elems ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("テストファイルのパス解決に失敗しました")
	}

	base := filepath.Join(filepath.Dir(file), "..", "..", "testdata")
	parts := append([]string{base}, elems...)
	path := filepath.Join(parts...)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("テストデータが見つかりません: %v", err)
	}

	return path
}

func sqliteRulesPath(t *testing.T, fixtureName string) string {
	t.Helper()

	jsonPath := testdataPath(t, "rules", fixtureName)
	sqlitePath := filepath.Join(t.TempDir(), strings.TrimSuffix(fixtureName, filepath.Ext(fixtureName))+".sqlite")
	if err := testutil.WriteSQLiteRuleSetFromJSON(jsonPath, sqlitePath); err != nil {
		t.Fatalf("SQLite fixture の作成に失敗しました: %v", err)
	}
	return sqlitePath
}
