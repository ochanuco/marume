package cli

import (
	"bufio"
	"bytes"
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

const defaultRulePath = "rules/rules.json"

// Version is the CLI version and may be overridden at build time with -ldflags.
var Version = "dev"

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
	case "classify-batch":
		return runClassifyBatch(ctx, args[1:], stdin, stdout, stderr)
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

	ruleStore, err := store.NewJSONRuleStore(*rulesPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	engine := evaluator.New(ruleStore)
	result, err := engine.Classify(ctx, input)
	if err != nil {
		return err
	}

	return writeJSON(stdout, result)
}

func runClassifyBatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (retErr error) {
	flags := flag.NewFlagSet("classify-batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume classify-batch --input <症例JSONL> [--output <結果JSONL>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "複数症例を1行ずつ分類し、結果をJSONLで返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONLファイルのパス。標準入力を使う場合は -")
	outputPath := flags.String("output", "-", "出力JSONLファイルのパス。標準出力に出す場合は -")
	rulesPath := flags.String("rules", defaultRulePath, "ルールセットJSONのパス")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	reader, cleanupInput, err := openInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	defer cleanupInput()

	ruleStore, err := store.NewJSONRuleStore(*rulesPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	preloadedRuleSet, err := ruleStore.ReadRuleSet(ctx)
	if err != nil {
		return err
	}
	if err := evaluator.ValidateRuleSet(preloadedRuleSet); err != nil {
		return err
	}

	writer, cleanupOutput, err := openOutput(*outputPath, stdout)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := cleanupOutput(); retErr == nil && closeErr != nil {
			retErr = fmt.Errorf("%w: バッチ結果のクローズに失敗しました: %v", errRuleRuntime, closeErr)
		}
	}()

	engine := evaluator.New(fixedRuleStore{ruleSet: preloadedRuleSet})
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(writer)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		result := classifyBatchLine(ctx, engine, lineNo, line)
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("%w: バッチ結果の書き込みに失敗しました: %v", errRuleRuntime, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w: JSONLの読み込みに失敗しました: %v", errInvalidInput, err)
	}

	return nil
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

	ruleStore, err := store.NewJSONRuleStore(*rulesPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	engine := evaluator.New(ruleStore)
	result, err := engine.Explain(ctx, input)
	if err != nil {
		if errors.Is(err, evaluator.ErrNoClassification) {
			if writeErr := writeJSON(stdout, result); writeErr != nil {
				return writeErr
			}
			return nil
		}
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

	ruleStore, err := store.NewJSONRuleStore(*rulesPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	ruleSet, err := ruleStore.ReadRuleSet(context.Background())
	if err != nil {
		return err
	}

	return writeJSON(stdout, map[string]string{
		"cli_version":  Version,
		"rule_version": ruleSet.RuleVersion,
		"build_id":     ruleSet.BuildID,
		"built_at":     ruleSet.BuiltAt,
	})
}

type batchClassifyResult struct {
	LineNo int                          `json:"line_no"`
	CaseID string                       `json:"case_id,omitempty"`
	Status string                       `json:"status"`
	Result *domain.ClassificationResult `json:"result,omitempty"`
	Error  *batchErrorResult            `json:"error,omitempty"`
}

type batchErrorResult struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	MessageEN string `json:"message_en,omitempty"`
}

type fixedRuleStore struct {
	ruleSet domain.RuleSet
}

func (s fixedRuleStore) LoadRuleSet(_ context.Context, fiscalYear int) (domain.RuleSet, error) {
	if s.ruleSet.FiscalYear != fiscalYear {
		return domain.RuleSet{}, fmt.Errorf("rule set fiscal year %d does not match requested %d", s.ruleSet.FiscalYear, fiscalYear)
	}
	return s.ruleSet, nil
}

func loadCaseInput(path string, stdin io.Reader) (domain.CaseInput, error) {
	reader, cleanup, err := openInput(path, stdin)
	if err != nil {
		return domain.CaseInput{}, err
	}
	defer cleanup()

	var input domain.CaseInput
	if err := decodeStrictJSON(reader, &input); err != nil {
		return domain.CaseInput{}, fmt.Errorf("%w: JSONの読み込みに失敗しました: %v", errInvalidInput, err)
	}
	return input, nil
}

func openInput(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return stdin, func() {}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return file, func() { _ = file.Close() }, nil
}

func openOutput(path string, stdout io.Writer) (io.Writer, func() error, error) {
	if path == "-" {
		return stdout, func() error { return nil }, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return file, file.Close, nil
}

func classifyBatchLine(ctx context.Context, engine *evaluator.Evaluator, lineNo int, line []byte) batchClassifyResult {
	var input domain.CaseInput
	if err := decodeStrictJSON(bytes.NewReader(line), &input); err != nil {
		return batchClassifyResult{
			LineNo: lineNo,
			Status: "error",
			Error: &batchErrorResult{
				Code:      "INVALID_JSON",
				Message:   fmt.Sprintf("%d 行目のJSONが不正です: %v", lineNo, err),
				MessageEN: fmt.Sprintf("invalid JSON at line %d: %v", lineNo, err),
			},
		}
	}

	result := batchClassifyResult{
		LineNo: lineNo,
		CaseID: input.CaseID,
	}

	if err := validateCaseInput(input); err != nil {
		result.Status = "error"
		result.Error = &batchErrorResult{
			Code:      "INVALID_INPUT",
			Message:   err.Error(),
			MessageEN: "invalid input",
		}
		return result
	}

	classified, err := engine.Classify(ctx, input)
	if err != nil {
		result.Status = "error"
		result.Error = classifyBatchError(err, input.CaseID)
		return result
	}

	result.Status = "ok"
	result.Result = &classified
	return result
}

func decodeStrictJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected trailing data")
	}
	return nil
}

func classifyBatchError(err error, caseID string) *batchErrorResult {
	switch {
	case errors.Is(err, evaluator.ErrNoClassification):
		return &batchErrorResult{
			Code:      "NO_CLASSIFICATION",
			Message:   fmt.Sprintf("症例 %s に一致する分類が見つかりません", caseID),
			MessageEN: fmt.Sprintf("no classification matched for case %s", caseID),
		}
	case errors.Is(err, evaluator.ErrRuleDefinition):
		return &batchErrorResult{
			Code:      "RULE_DEFINITION_ERROR",
			Message:   fmt.Sprintf("ルール定義エラーが見つかりました: %v", err),
			MessageEN: fmt.Sprintf("rule definition error: %v", err),
		}
	case errors.Is(err, os.ErrNotExist):
		return &batchErrorResult{
			Code:      "RULES_NOT_FOUND",
			Message:   "ルールセットファイルが見つかりません",
			MessageEN: "rule set file not found",
		}
	default:
		return &batchErrorResult{
			Code:      "CLASSIFICATION_ERROR",
			Message:   fmt.Sprintf("分類中にエラーが発生しました: %v", err),
			MessageEN: fmt.Sprintf("classification error: %v", err),
		}
	}
}

func validateCaseInput(input domain.CaseInput) error {
	// NOTE: POCでは evaluator が最低限必要とする項目だけをここで検証している。
	// procedures/comorbidities の空配列や age/sex の未指定はルール次第で許容し、
	// JSONのキー欠落と空値の厳密な区別は、将来 input DTO を分ける段階で扱う。
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
	fmt.Fprintln(w, "DPC診断群分類をローカルで試すためのCLIです。")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "コマンド:")
	fmt.Fprintln(w, "  classify   単一症例を分類する")
	fmt.Fprintln(w, "  classify-batch 複数症例を一括分類する")
	fmt.Fprintln(w, "  explain    候補ルールと判定理由を表示する")
	fmt.Fprintln(w, "  validate   入力JSONを検証する")
	fmt.Fprintln(w, "  version    CLIとルールセットのバージョンを表示する")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "例:")
	fmt.Fprintln(w, "  marume classify --input case.json")
	fmt.Fprintln(w, "  marume classify-batch --input cases.jsonl --output results.jsonl")
	fmt.Fprintln(w, "  marume explain --input case.json")
	fmt.Fprintln(w, "  marume validate --input case.json")
	fmt.Fprintln(w, "  marume version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "各コマンドの詳細は `marume <コマンド> --help` で確認できます。")
}
