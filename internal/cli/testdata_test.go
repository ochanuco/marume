package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ochanuco/marume/internal/cli"
	"github.com/ochanuco/marume/internal/store"
)

func TestTestdataCaseは症例サンプルJSONを返す(t *testing.T) {
	rulesPath := sqliteRulesPath(t, "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"testdata", "case", "--preset", "ok", "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("testdata case でエラーが返りました: %v", err)
	}

	var result map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("testdata case のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if result["case_id"] != "sample-ok" {
		t.Fatalf("case_id は sample-ok を期待しましたが、実際は %v でした", result["case_id"])
	}
	if result["main_diagnosis"] != "I219" {
		t.Fatalf("main_diagnosis は I219 を期待しましたが、実際は %v でした", result["main_diagnosis"])
	}
}

func TestTestdataBatchはJSONLを返す(t *testing.T) {
	rulesPath := sqliteRulesPath(t, "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"testdata", "batch", "--preset", "basic", "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("testdata batch でエラーが返りました: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("testdata batch の出力行数は 2 行を期待しましたが、実際は %d 行でした", len(lines))
	}

	var first map[string]any
	if decodeErr := json.Unmarshal([]byte(lines[0]), &first); decodeErr != nil {
		t.Fatalf("1 行目のJSONを読み取れませんでした: %v", decodeErr)
	}
	if first["main_diagnosis"] != "I219" {
		t.Fatalf("1 行目の main_diagnosis は I219 を期待しましたが、実際は %v でした", first["main_diagnosis"])
	}
}

func TestTestdataWriteはサンプル一式を書き出す(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "sample")
	rulesPath := sqliteRulesPath(t, "rules-2026.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"testdata", "write", "--dir", outputDir, "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("testdata write でエラーが返りました: %v", err)
	}

	var summary map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &summary); decodeErr != nil {
		t.Fatalf("testdata write のJSON出力を読み取れませんでした: %v", decodeErr)
	}
	if summary["dir"] != outputDir {
		t.Fatalf("dir は %s を期待しましたが、実際は %v でした", outputDir, summary["dir"])
	}
	files, ok := summary["files"].(map[string]any)
	if !ok {
		t.Fatalf("files はオブジェクトを期待しましたが、実際は %v でした", summary["files"])
	}

	for _, path := range []string{
		filepath.Join(outputDir, "case-ok.json"),
		filepath.Join(outputDir, "cases-basic.jsonl"),
		filepath.Join(outputDir, "rules-minimal.sqlite"),
	} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("生成ファイルが見つかりません: %s (%v)", path, statErr)
		}
	}
	if files["case"] != filepath.Join(outputDir, "case-ok.json") {
		t.Fatalf("files.case は %s を期待しましたが、実際は %v でした", filepath.Join(outputDir, "case-ok.json"), files["case"])
	}
	if files["batch"] != filepath.Join(outputDir, "cases-basic.jsonl") {
		t.Fatalf("files.batch は %s を期待しましたが、実際は %v でした", filepath.Join(outputDir, "cases-basic.jsonl"), files["batch"])
	}
	if files["rules"] != filepath.Join(outputDir, "rules-minimal.sqlite") {
		t.Fatalf("files.rules は %s を期待しましたが、実際は %v でした", filepath.Join(outputDir, "rules-minimal.sqlite"), files["rules"])
	}

	caseData, readErr := os.ReadFile(filepath.Join(outputDir, "case-ok.json"))
	if readErr != nil {
		t.Fatalf("case-ok.json の読み込みに失敗しました: %v", readErr)
	}
	var caseContent map[string]any
	if decodeErr := json.Unmarshal(caseData, &caseContent); decodeErr != nil {
		t.Fatalf("case-ok.json のJSONパースに失敗しました: %v", decodeErr)
	}
	if caseContent["case_id"] != "sample-ok" {
		t.Fatalf("case-ok.json の case_id は sample-ok を期待しましたが、実際は %v でした", caseContent["case_id"])
	}

	rulesSQLitePath := filepath.Join(outputDir, "rules-minimal.sqlite")
	ruleStore, rulesErr := store.NewSQLiteRuleStore(rulesSQLitePath)
	if rulesErr != nil {
		t.Fatalf("rules-minimal.sqlite のストア作成に失敗しました: %v", rulesErr)
	}
	ruleSet, rulesErr := ruleStore.ReadRuleSet(context.Background())
	if rulesErr != nil {
		t.Fatalf("rules-minimal.sqlite の読み込みに失敗しました: %v", rulesErr)
	}
	if ruleSet.RuleVersion != "2026.0.0-poc" {
		t.Fatalf("rules-minimal.sqlite の rule_version は 2026.0.0-poc を期待しましたが、実際は %q でした", ruleSet.RuleVersion)
	}
	expectedRuleCount := 2
	if len(ruleSet.Rules) != expectedRuleCount {
		t.Fatalf("rules-minimal.sqlite の rules は %d 件を期待しましたが、実際は %d 件でした", expectedRuleCount, len(ruleSet.Rules))
	}

	batchFile, openErr := os.Open(filepath.Join(outputDir, "cases-basic.jsonl"))
	if openErr != nil {
		t.Fatalf("cases-basic.jsonl のオープンに失敗しました: %v", openErr)
	}
	defer batchFile.Close()

	scanner := bufio.NewScanner(batchFile)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var row map[string]any
		if decodeErr := json.Unmarshal(scanner.Bytes(), &row); decodeErr != nil {
			t.Fatalf("cases-basic.jsonl の %d 行目のJSONパースに失敗しました: %v", lineCount, decodeErr)
		}
		rawCaseID, ok := row["case_id"]
		if !ok || rawCaseID == nil {
			t.Fatalf("cases-basic.jsonl の %d 行目は case_id キーを期待しましたが、実際は %v でした", lineCount, row)
		}
		caseID, ok := rawCaseID.(string)
		if !ok || caseID == "" {
			t.Fatalf("cases-basic.jsonl の %d 行目は空でない case_id を期待しましたが、実際は %v でした", lineCount, rawCaseID)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("cases-basic.jsonl の読み込みに失敗しました: %v", scanErr)
	}
	if lineCount != 2 {
		t.Fatalf("cases-basic.jsonl の行数は 2 行を期待しましたが、実際は %d 行でした", lineCount)
	}
}

func TestTestdataUnknownPresetは入力エラーを返す(t *testing.T) {
	rulesPath := sqliteRulesPath(t, "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"testdata", "case", "--preset", "missing", "--rules", rulesPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("未知の preset では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("未知の preset の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(err.Error(), `case preset "missing" は未定義です`) {
		t.Fatalf("未知の preset のエラーメッセージが想定と異なります: %v", err)
	}
}

func TestTestdataRulesは非SQLite拡張子の出力を入力エラーにする(t *testing.T) {
	rulesPath := sqliteRulesPath(t, "rules-2026.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := cli.Run(
		context.Background(),
		[]string{"testdata", "rules", "--rules", rulesPath, "--output", filepath.Join(t.TempDir(), "rules.json")},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("非 SQLite 拡張子の出力では入力エラーを期待しましたが、エラーが返りませんでした")
	}
	if cli.ExitCode(err) != 1 {
		t.Fatalf("非 SQLite 拡張子の終了コードは 1 を期待しましたが、実際は %d でした", cli.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "--output は SQLite ファイルパスを指定してください") {
		t.Fatalf("非 SQLite 拡張子のエラーメッセージが想定と異なります: %v", err)
	}
}
