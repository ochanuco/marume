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
	if !strings.Contains(lines[0], `"status":"ok"`) {
		t.Fatalf("1 行目は成功を期待しましたが、実際は %s でした", lines[0])
	}
	if !strings.Contains(lines[0], `"message":"主傷病名が I219 に一致しました"`) {
		t.Fatalf("1 行目の理由メッセージが想定と異なります: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"status":"error"`) || !strings.Contains(lines[1], `"code":"NO_CLASSIFICATION"`) {
		t.Fatalf("2 行目は分類不能エラーを期待しましたが、実際は %s でした", lines[1])
	}
	if !strings.Contains(lines[2], `"code":"INVALID_JSON"`) {
		t.Fatalf("3 行目は JSON エラーを期待しましたが、実際は %s でした", lines[2])
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
	if result["selected_rule"] != "" {
		t.Fatalf("分類不能時の selected_rule は空を期待しましたが、実際は %v でした", result["selected_rule"])
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

	if !strings.Contains(stdout.String(), `"rule_version": "2027.0.0-poc"`) {
		t.Fatalf("2027 年度の rule_version を期待しましたが、実際の出力は %s でした", stdout.String())
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
