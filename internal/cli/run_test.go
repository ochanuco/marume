package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ochanuco/marume/internal/cli"
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
	selectedRule, exists := result["selected_rule"]
	if exists && selectedRule != nil && selectedRule != "" {
		t.Fatalf("分類不能時の selected_rule は空または未設定を期待しましたが、実際は %v でした", selectedRule)
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
	if !strings.Contains(stdout.String(), `"status":"ok"`) {
		t.Fatalf("長いJSONL行でも成功を期待しましたが、実際の出力は %s でした", stdout.String())
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
