package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ochanuco/marume/internal/domain"
	"github.com/ochanuco/marume/internal/evaluator"
	"github.com/ochanuco/marume/internal/store"
)

var (
	errInvalidInput = errors.New("入力エラー")
	errRuleRuntime  = errors.New("ルール実行エラー")
)

const defaultRulePath = "testdata/rules/rules-2026.json"

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errInvalidInput
	}

	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "classify":
		return runClassify(ctx, args[1:], stdin, stdout, stderr)
	case "explain":
		return runExplain(ctx, args[1:], stdin, stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdin, stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return fmt.Errorf("%w: 不明なコマンド %q", errInvalidInput, args[0])
	}
}

func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errInvalidInput):
		return 1
	case errors.Is(err, evaluator.ErrNoClassification):
		return 2
	case errors.Is(err, os.ErrNotExist):
		return 3
	default:
		return 4
	}
}

func runClassify(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("classify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume classify --input <症例JSON>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "単一症例を分類し、DPCコードと採用ルールをJSONで返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONファイルのパス。標準入力を使う場合は -")
	rulesPath := flags.String("rules", defaultRulePath, "ルールセットJSONのパス")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	input, err := loadCaseInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	if err := validateCaseInput(input); err != nil {
		return err
	}

	engine := evaluator.New(store.NewJSONRuleStore(*rulesPath))
	result, err := engine.Classify(ctx, input)
	if err != nil {
		return err
	}

	return writeJSON(stdout, result)
}

func runExplain(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("explain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume explain --input <症例JSON>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "候補ルールごとの一致状況と、不一致理由をJSONで返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONファイルのパス。標準入力を使う場合は -")
	rulesPath := flags.String("rules", defaultRulePath, "ルールセットJSONのパス")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	input, err := loadCaseInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	if err := validateCaseInput(input); err != nil {
		return err
	}

	engine := evaluator.New(store.NewJSONRuleStore(*rulesPath))
	result, err := engine.Explain(ctx, input)
	if err != nil {
		return err
	}

	return writeJSON(stdout, result)
}

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume validate --input <症例JSON>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "入力JSONの最低限の必須項目を検証します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONファイルのパス。標準入力を使う場合は -")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	input, err := loadCaseInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	if err := validateCaseInput(input); err != nil {
		return err
	}

	return writeJSON(stdout, map[string]string{
		"status":  "ok",
		"case_id": input.CaseID,
	})
}

func runVersion(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume version")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "CLIバージョンとルールセットのバージョン情報をJSONで返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	rulesPath := flags.String("rules", defaultRulePath, "ルールセットJSONのパス")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	ruleSet, err := store.NewJSONRuleStore(*rulesPath).LoadRuleSet(context.Background(), 2026)
	if err != nil {
		return err
	}

	return writeJSON(stdout, map[string]string{
		"cli_version":  "0.1.0-poc",
		"rule_version": ruleSet.RuleVersion,
		"build_id":     ruleSet.BuildID,
		"built_at":     ruleSet.BuiltAt,
	})
}

func loadCaseInput(path string, stdin io.Reader) (domain.CaseInput, error) {
	var reader io.Reader
	if path == "-" {
		reader = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return domain.CaseInput{}, err
		}
		defer file.Close()
		reader = file
	}

	var input domain.CaseInput
	if err := json.NewDecoder(reader).Decode(&input); err != nil {
		return domain.CaseInput{}, fmt.Errorf("%w: JSONの読み込みに失敗しました: %v", errInvalidInput, err)
	}
	return input, nil
}

func validateCaseInput(input domain.CaseInput) error {
	switch {
	case input.CaseID == "":
		return fmt.Errorf("%w: case_id は必須です", errInvalidInput)
	case input.FiscalYear == 0:
		return fmt.Errorf("%w: fiscal_year は必須です", errInvalidInput)
	case input.MainDiagnosis == "":
		return fmt.Errorf("%w: main_diagnosis は必須です", errInvalidInput)
	}
	return nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "使い方: marume <コマンド> [フラグ]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "DPC分類をローカルで試すためのCLIです。")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "コマンド:")
	fmt.Fprintln(w, "  classify   単一症例を分類する")
	fmt.Fprintln(w, "  explain    候補ルールと判定理由を表示する")
	fmt.Fprintln(w, "  validate   入力JSONを検証する")
	fmt.Fprintln(w, "  version    CLIとルールセットのバージョンを表示する")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "例:")
	fmt.Fprintln(w, "  marume classify --input case.json")
	fmt.Fprintln(w, "  marume explain --input case.json")
	fmt.Fprintln(w, "  marume validate --input case.json")
	fmt.Fprintln(w, "  marume version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "各コマンドの詳細は `marume <コマンド> --help` で確認できます。")
}
