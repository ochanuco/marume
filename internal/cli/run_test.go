package cli_test

import (
	"bytes"
	"context"
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
