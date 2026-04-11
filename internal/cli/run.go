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
	"path/filepath"
	"slices"
	"strings"

	"github.com/ochanuco/marume/internal/domain"
	"github.com/ochanuco/marume/internal/evaluator"
	"github.com/ochanuco/marume/internal/store"
)

var (
	errInvalidInput = errors.New("入力エラー")
	errRuleRuntime  = errors.New("ルール実行エラー")
)

const defaultRulePath = "rules/rules-2026.sqlite"
const legacyDefaultRulePath = "rules/rules.json"

// Version is the CLI version and may be overridden at build time with -ldflags.
var Version = "dev"

// Run dispatches a CLI subcommand and writes user-facing output to the provided streams.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, jsonErrors ...bool) error {
	diagnosticStderr := stderr
	jsonMode := len(jsonErrors) > 0 && jsonErrors[0]
	if jsonMode {
		diagnosticStderr = io.Discard
	}

	if len(args) == 0 {
		printUsage(diagnosticStderr)
		return errInvalidInput
	}

	args = stripGlobalFlags(args)
	if len(args) == 0 {
		printUsage(diagnosticStderr)
		return errInvalidInput
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "classify":
		return runClassify(ctx, args[1:], stdin, stdout, diagnosticStderr)
	case "classify-batch":
		return runClassifyBatch(ctx, args[1:], stdin, stdout, diagnosticStderr)
	case "explain":
		return runExplain(ctx, args[1:], stdin, stdout, diagnosticStderr)
	case "capabilities":
		return runCapabilities(args[1:], stdout, diagnosticStderr)
	case "schema":
		return runSchema(args[1:], stdout, diagnosticStderr)
	case "testdata":
		return runTestdata(ctx, args[1:], stdout, diagnosticStderr)
	case "validate":
		return runValidate(args[1:], stdin, stdout, diagnosticStderr)
	case "version":
		return runVersion(args[1:], stdout, diagnosticStderr)
	default:
		printUsage(diagnosticStderr)
		return fmt.Errorf("%w: 不明なコマンド %q", errInvalidInput, args[0])
	}
}

// ExitCode maps domain and CLI errors to process exit codes.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errInvalidInput):
		return 1
	case errors.Is(err, store.ErrFiscalYearMismatch):
		return 1
	case errors.Is(err, evaluator.ErrNoClassification):
		return 2
	case errors.Is(err, os.ErrNotExist):
		return 3
	case errors.Is(err, evaluator.ErrRuleDefinition):
		return 5
	default:
		return 4
	}
}

// runClassify handles the single-case classify subcommand.
func runClassify(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("classify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume classify --input <症例JSON>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "単一症例を分類し、DPCコードと採用ルールをJSONで返します。")
		fmt.Fprintln(stderr, "")
		writeSchemaHelp(stderr, caseInputSchema)
		fmt.Fprintln(stderr, "出力スキーマ: marume schema classify-result")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONファイルのパス。標準入力を使う場合は -")
	rulesPath := flags.String("rules", defaultRulePath, "ルールスナップショットのパス (JSON または SQLite)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	input, err := loadCaseInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	if err := validateCaseInput(input); err != nil {
		return err
	}

	ruleStore, err := store.NewRuleStore(resolveRulesPath(flags, *rulesPath))
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

// runClassifyBatch handles the batch classify subcommand over JSONL input.
func runClassifyBatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (retErr error) {
	flags := flag.NewFlagSet("classify-batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume classify-batch --input <症例JSONL> [--output <結果JSONL>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "複数症例を1行ずつ分類し、結果をJSONLで返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "各行の入力には次のJSONを使います。")
		fmt.Fprintln(stderr, "")
		writeSchemaHelp(stderr, caseInputSchema)
		fmt.Fprintln(stderr, "出力スキーマ: marume schema batch-result")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONLファイルのパス。標準入力を使う場合は -")
	outputPath := flags.String("output", "-", "出力JSONLファイルのパス。標準出力に出す場合は -")
	rulesPath := flags.String("rules", defaultRulePath, "ルールスナップショットのパス (JSON または SQLite)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}
	if err := validateBatchPaths(*inputPath, *outputPath); err != nil {
		return err
	}

	reader, cleanupInput, err := openInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	defer cleanupInput()

	ruleStore, err := store.NewRuleStore(resolveRulesPath(flags, *rulesPath))
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
		if err := ctx.Err(); err != nil {
			return err
		}

		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		result := classifyBatchLine(ctx, engine, lineNo, line)
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("%w: バッチ結果の書き込みに失敗しました: %v", errRuleRuntime, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w: JSONLの読み込みに失敗しました: %v", errInvalidInput, err)
	}

	return nil
}

// runExplain handles the explain subcommand for a single case input.
func runExplain(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("explain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume explain --input <症例JSON>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "候補ルールごとの一致状況と、不一致理由をJSONで返します。")
		fmt.Fprintln(stderr, "")
		writeSchemaHelp(stderr, caseInputSchema)
		fmt.Fprintln(stderr, "出力スキーマ: marume schema explain-result")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	inputPath := flags.String("input", "-", "入力JSONファイルのパス。標準入力を使う場合は -")
	rulesPath := flags.String("rules", defaultRulePath, "ルールスナップショットのパス (JSON または SQLite)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	input, err := loadCaseInput(*inputPath, stdin)
	if err != nil {
		return err
	}
	if err := validateCaseInput(input); err != nil {
		return err
	}

	ruleStore, err := store.NewRuleStore(resolveRulesPath(flags, *rulesPath))
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	engine := evaluator.New(ruleStore)
	result, err := engine.Explain(ctx, input)
	if err != nil {
		if errors.Is(err, evaluator.ErrNoClassification) {
			// explain は候補ルールのJSONを返すことを優先し、分類不能は selected_rule=""
			// をシグナルとして writeJSON しつつ exit 0 にする。
			result.SelectedRule = ""
			if writeErr := writeJSON(stdout, result); writeErr != nil {
				return writeErr
			}
			return nil
		}
		return err
	}

	return writeJSON(stdout, result)
}

// runValidate validates the minimum required input fields without classifying.
func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume validate --input <症例JSON>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "入力JSONの最低限の必須項目を検証します。")
		fmt.Fprintln(stderr, "")
		writeSchemaHelp(stderr, caseInputSchema)
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
	if err := rejectExtraArgs(flags); err != nil {
		return err
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

// runSchema prints a JSON schema document for Agent and human inspection.
func runSchema(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("schema", flag.ContinueOnError)
	flags.SetOutput(stderr)
	listOnly := flags.Bool("list", false, "利用可能なスキーマ名を表示する")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume schema <名前>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "JSON Schema を標準出力へ返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "利用可能なスキーマ:")
		for _, name := range listSchemaNames() {
			fmt.Fprintf(stderr, "  %s\n", name)
		}
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "例:")
		fmt.Fprintln(stderr, "  marume schema case-input")
		fmt.Fprintln(stderr, "  marume schema classify-result")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if *listOnly {
		if flags.NArg() > 0 {
			return rejectExtraArgs(flags)
		}
		return writeJSON(stdout, map[string]any{"schemas": listSchemaNames()})
	}
	if flags.NArg() == 0 {
		return fmt.Errorf("%w: schema にはスキーマ名を1つ指定してください", errInvalidInput)
	}
	if flags.NArg() > 1 {
		return rejectExtraArgs(flags)
	}

	doc, ok := schemaRegistry[flags.Arg(0)]
	if !ok {
		return fmt.Errorf("%w: 不明なスキーマ %q", errInvalidInput, flags.Arg(0))
	}
	return writeJSON(stdout, doc.jsonSchema())
}

func runCapabilities(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume capabilities")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "CLI のコマンド・フラグ・終了コード・利用可能スキーマをJSONで返します。")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	defaultRulesPath := resolvedDefaultRulesPath()
	return writeJSON(stdout, map[string]any{
		"cli_version":       Version,
		"default_rule_path": defaultRulesPath,
		"global_flags": []capabilityFlag{
			{
				Name:        "--json-errors",
				Type:        "bool",
				Description: "失敗時に構造化エラーJSONを標準エラーへ出します",
			},
		},
		"commands":   commandCapabilities(defaultRulesPath),
		"schemas":    listSchemaNames(),
		"exit_codes": exitCodeDocs(),
	})
}

// runVersion prints CLI and rule snapshot metadata.
func runVersion(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume version")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "CLIバージョンとルールセットのバージョン情報をJSONで返します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "出力スキーマ: marume schema version-result")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	rulesPath := flags.String("rules", defaultRulePath, "ルールスナップショットのパス (JSON または SQLite)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	ruleStore, err := store.NewRuleStore(resolveRulesPath(flags, *rulesPath))
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}

	ruleSet, err := ruleStore.ReadRuleSet(context.Background())
	if err != nil {
		return err
	}
	switch {
	case ruleSet.RuleVersion == "":
		return fmt.Errorf("%w: rule_version は必須です", errInvalidInput)
	case ruleSet.BuildID == "":
		return fmt.Errorf("%w: build_id は必須です", errInvalidInput)
	case ruleSet.BuiltAt == "":
		return fmt.Errorf("%w: built_at は必須です", errInvalidInput)
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

type cliErrorEnvelope struct {
	ExitCode int              `json:"exit_code"`
	Error    batchErrorResult `json:"error"`
}

type capabilityFlag struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

type capabilityCommand struct {
	Name         string           `json:"name"`
	Summary      string           `json:"summary"`
	InputSchema  string           `json:"input_schema,omitempty"`
	OutputSchema string           `json:"output_schema,omitempty"`
	Examples     []string         `json:"examples,omitempty"`
	Flags        []capabilityFlag `json:"flags,omitempty"`
}

type exitCodeDoc struct {
	Code        int    `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type fixedRuleStore struct {
	ruleSet domain.RuleSet
}

// LoadRuleSet returns the preloaded rule set after checking the requested fiscal year.
func (s fixedRuleStore) LoadRuleSet(_ context.Context, fiscalYear int) (domain.RuleSet, error) {
	if s.ruleSet.FiscalYear != fiscalYear {
		return domain.RuleSet{}, store.FiscalYearMismatchError{
			RuleSetFiscalYear: s.ruleSet.FiscalYear,
			RequestedYear:     fiscalYear,
		}
	}
	return s.ruleSet, nil
}

// loadCaseInput reads and strictly decodes one case JSON document.
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

// JSONErrorsEnabled reports whether the caller requested structured stderr errors.
func JSONErrorsEnabled(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			return false
		}
		if arg == "--json-errors" {
			return true
		}
	}
	return false
}

func stripGlobalFlags(args []string) []string {
	filtered := make([]string, 0, len(args))
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		if arg == "--json-errors" {
			continue
		}
		filtered = append(filtered, arg)
	}
	filtered = append(filtered, args[index:]...)
	return filtered
}

// openInput returns stdin for "-" or opens the requested file path for reading.
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

// openOutput returns stdout for "-" or creates the requested file path for writing.
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

// resolveRulesPath keeps default rule loading backward compatible:
// explicit --rules wins; otherwise prefer the default SQLite path, then the legacy JSON path.
// flagWasProvided is used so an explicit --rules rules/rules-2026.sqlite keeps the original path.
func resolveRulesPath(flags *flag.FlagSet, requestedPath string) string {
	if requestedPath != defaultRulePath || flagWasProvided(flags, "rules") {
		return requestedPath
	}
	if _, err := os.Stat(requestedPath); err == nil {
		return requestedPath
	}
	if _, err := os.Stat(legacyDefaultRulePath); err == nil {
		return legacyDefaultRulePath
	}
	return requestedPath
}

func resolvedDefaultRulesPath() string {
	if _, err := os.Stat(defaultRulePath); err == nil {
		return defaultRulePath
	}
	if _, err := os.Stat(legacyDefaultRulePath); err == nil {
		return legacyDefaultRulePath
	}
	return defaultRulePath
}

func flagWasProvided(flags *flag.FlagSet, name string) bool {
	provided := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

// classifyBatchLine classifies one JSONL line and normalizes errors into batch output.
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

// decodeStrictJSON decodes one JSON value and rejects unknown fields or trailing data.
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

// classifyBatchError maps runtime errors to the public batch error payload shape.
func classifyBatchError(err error, caseID string) *batchErrorResult {
	switch {
	case errors.Is(err, evaluator.ErrNoClassification):
		return &batchErrorResult{
			Code:      "NO_CLASSIFICATION",
			Message:   fmt.Sprintf("症例 %s に一致する分類が見つかりません", caseID),
			MessageEN: fmt.Sprintf("no classification matched for case %s", caseID),
		}
	case errors.Is(err, evaluator.ErrRuleDefinition):
		messageEN := fmt.Sprintf("rule definition error: %v", err)
		var reasoned interface{ Reason() domain.ReasonEntry }
		if errors.As(err, &reasoned) && reasoned.Reason().MessageEN != "" {
			messageEN = fmt.Sprintf("rule definition error: %s", reasoned.Reason().MessageEN)
		}
		return &batchErrorResult{
			Code:      "RULE_DEFINITION_ERROR",
			Message:   fmt.Sprintf("ルール定義エラーが見つかりました: %v", err),
			MessageEN: messageEN,
		}
	case errors.Is(err, store.ErrFiscalYearMismatch):
		message := fmt.Sprintf("ルール年度と症例年度が一致しません: %v", err)
		messageEN := fmt.Sprintf("rule fiscal year does not match case fiscal year: %v", err)
		var mismatch store.FiscalYearMismatchError
		if errors.As(err, &mismatch) {
			message = fmt.Sprintf("ルール年度と症例年度が一致しません: %d と %d", mismatch.RuleSetFiscalYear, mismatch.RequestedYear)
			messageEN = fmt.Sprintf("rule fiscal year does not match case fiscal year: %d vs %d", mismatch.RuleSetFiscalYear, mismatch.RequestedYear)
		}
		return &batchErrorResult{
			Code:      "FISCAL_YEAR_MISMATCH",
			Message:   message,
			MessageEN: messageEN,
		}
	default:
		return &batchErrorResult{
			Code:      "CLASSIFICATION_ERROR",
			Message:   fmt.Sprintf("分類中にエラーが発生しました: %v", err),
			MessageEN: fmt.Sprintf("classification error: %v", err),
		}
	}
}

func cliErrorResult(err error) cliErrorEnvelope {
	errorResult := batchErrorResult{
		Code:    "INTERNAL_ERROR",
		Message: err.Error(),
	}

	switch {
	case errors.Is(err, errInvalidInput):
		errorResult.Code = "INVALID_INPUT"
		errorResult.MessageEN = "invalid input"
	case errors.Is(err, store.ErrFiscalYearMismatch):
		errorResult = *classifyBatchError(err, "")
	case errors.Is(err, evaluator.ErrNoClassification):
		errorResult = *classifyBatchError(err, "")
		errorResult.Message = "分類結果が見つかりません"
		errorResult.MessageEN = "no classification matched"
	case errors.Is(err, os.ErrNotExist):
		errorResult.Code = "FILE_NOT_FOUND"
		errorResult.MessageEN = "file not found"
	case errors.Is(err, evaluator.ErrRuleDefinition):
		errorResult = *classifyBatchError(err, "")
	case errors.Is(err, context.Canceled):
		errorResult.Code = "CANCELED"
		errorResult.Message = "処理はキャンセルされました"
		errorResult.MessageEN = "operation canceled"
	default:
		errorResult.MessageEN = "internal error"
	}

	return cliErrorEnvelope{
		ExitCode: ExitCode(err),
		Error:    errorResult,
	}
}

// WriteErrorJSON writes a structured stderr payload for machine callers.
func WriteErrorJSON(w io.Writer, err error) error {
	return writeJSON(w, cliErrorResult(err))
}

// validateCaseInput enforces the minimum POC input contract before evaluation.
func validateCaseInput(input domain.CaseInput) error {
	// NOTE: POCでは evaluator が最低限必要とする項目だけをここで検証している。
	// procedures/comorbidities の空配列や age/sex の未指定はルール次第で許容し、
	// JSONのキー欠落と空値の厳密な区別は、将来 input DTO を分ける段階で扱う。
	switch {
	case input.CaseID == "":
		return fmt.Errorf("%w: case_id は必須です", errInvalidInput)
	case input.FiscalYear <= 0:
		return fmt.Errorf("%w: fiscal_year は必須です", errInvalidInput)
	case input.Age != nil && *input.Age < 0:
		return fmt.Errorf("%w: age は負の値を指定できません", errInvalidInput)
	case input.MainDiagnosis == "":
		return fmt.Errorf("%w: main_diagnosis は必須です", errInvalidInput)
	}
	return nil
}

// validateBatchPaths rejects batch runs that would read from and write to the same file.
func validateBatchPaths(inputPath, outputPath string) error {
	if inputPath == "-" || outputPath == "-" {
		return nil
	}

	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("%w: 入力パスの解決に失敗しました: %v", errInvalidInput, err)
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("%w: 出力パスの解決に失敗しました: %v", errInvalidInput, err)
	}
	if inputAbs == outputAbs {
		return fmt.Errorf("%w: input と output に同じファイルは指定できません", errInvalidInput)
	}
	inputInfo, err := os.Stat(inputAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%w: 入力パスの確認に失敗しました: %v", errInvalidInput, err)
	}
	outputInfo, err := os.Stat(outputAbs)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: 出力パスの確認に失敗しました: %v", errInvalidInput, err)
	}
	if err == nil && os.SameFile(inputInfo, outputInfo) {
		return fmt.Errorf("%w: input と output に同じファイルは指定できません", errInvalidInput)
	}

	return nil
}

// rejectExtraArgs rejects leftover positional arguments for flag-based subcommands.
func rejectExtraArgs(flags *flag.FlagSet) error {
	if flags.NArg() == 0 {
		return nil
	}
	return fmt.Errorf("%w: 余分な引数があります: %v", errInvalidInput, flags.Args())
}

// writeJSON writes an indented JSON document followed by a trailing newline.
func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// printUsage writes the top-level CLI help in Japanese.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "使い方: marume <コマンド> [フラグ]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "DPC診断群分類をローカルで試すためのCLIです。")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "コマンド:")
	fmt.Fprintln(w, "  classify   単一症例を分類する")
	fmt.Fprintln(w, "  classify-batch 複数症例を一括分類する")
	fmt.Fprintln(w, "  explain    候補ルールと判定理由を表示する")
	fmt.Fprintln(w, "  capabilities CLIの機械可読な契約情報を表示する")
	fmt.Fprintln(w, "  schema     入出力JSON Schemaを表示する")
	fmt.Fprintln(w, "  testdata   サンプル入力とルールセットを生成する")
	fmt.Fprintln(w, "  validate   入力JSONを検証する")
	fmt.Fprintln(w, "  version    CLIとルールセットのバージョンを表示する")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "グローバルフラグ:")
	fmt.Fprintln(w, "  --json-errors  失敗時に構造化エラーJSONを標準エラーへ出す")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "例:")
	fmt.Fprintln(w, "  marume classify --input case.json")
	fmt.Fprintln(w, "  marume classify-batch --input cases.jsonl --output results.jsonl")
	fmt.Fprintln(w, "  marume explain --input case.json")
	fmt.Fprintln(w, "  marume capabilities")
	fmt.Fprintln(w, "  marume schema case-input")
	fmt.Fprintln(w, "  marume testdata write --dir ./.local/marume-sample")
	fmt.Fprintln(w, "  marume validate --input case.json")
	fmt.Fprintln(w, "  marume --json-errors classify --input bad.json")
	fmt.Fprintln(w, "  marume version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "各コマンドの詳細は `marume <コマンド> --help` で確認できます。")
}

func listSchemaNames() []string {
	names := make([]string, 0, len(schemaRegistry))
	for name := range schemaRegistry {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func commandCapabilities(defaultRulesPath string) []capabilityCommand {
	return []capabilityCommand{
		{
			Name:         "classify",
			Summary:      "単一症例を分類します",
			InputSchema:  caseInputSchema.Name,
			OutputSchema: classifyResultSchema.Name,
			Examples:     []string{"marume classify --input case.json", "marume classify --rules rules/rules-2026.sqlite --input -"},
			Flags: []capabilityFlag{
				{Name: "--input", Type: "string", Description: "入力JSONファイルのパス。標準入力は -", Required: true, Default: "-"},
				{Name: "--rules", Type: "string", Description: "ルールスナップショットのパス", Default: defaultRulesPath},
			},
		},
		{
			Name:         "classify-batch",
			Summary:      "複数症例を JSONL で一括分類します",
			InputSchema:  caseInputSchema.Name,
			OutputSchema: batchResultSchema.Name,
			Examples:     []string{"marume classify-batch --input cases.jsonl --output results.jsonl"},
			Flags: []capabilityFlag{
				{Name: "--input", Type: "string", Description: "入力JSONLファイルのパス。標準入力は -", Required: true, Default: "-"},
				{Name: "--output", Type: "string", Description: "結果JSONLの出力先。標準出力は -", Default: "-"},
				{Name: "--rules", Type: "string", Description: "ルールスナップショットのパス", Default: defaultRulesPath},
			},
		},
		{
			Name:         "explain",
			Summary:      "候補ルールと一致理由を返します",
			InputSchema:  caseInputSchema.Name,
			OutputSchema: explainResultSchema.Name,
			Examples:     []string{"marume explain --input case.json"},
			Flags: []capabilityFlag{
				{Name: "--input", Type: "string", Description: "入力JSONファイルのパス。標準入力は -", Required: true, Default: "-"},
				{Name: "--rules", Type: "string", Description: "ルールスナップショットのパス", Default: defaultRulesPath},
			},
		},
		{
			Name:         "capabilities",
			Summary:      "CLI のコマンド・フラグ・終了コード・スキーマ一覧を返します",
			OutputSchema: capabilitiesResultSchema.Name,
			Examples:     []string{"marume capabilities"},
		},
		{
			Name:     "schema",
			Summary:  "JSON Schema を返します",
			Examples: []string{"marume schema case-input", "marume schema --list"},
			Flags: []capabilityFlag{
				{Name: "--list", Type: "bool", Description: "利用可能なスキーマ名を表示する"},
			},
		},
		{
			Name:     "testdata",
			Summary:  "サンプル入力とサンプルルールを生成します",
			Examples: []string{"marume testdata write --dir ./.local/marume-sample"},
		},
		{
			Name:         "validate",
			Summary:      "入力JSONの最低限の必須項目を検証します",
			InputSchema:  caseInputSchema.Name,
			OutputSchema: validateResultSchema.Name,
			Examples:     []string{"marume validate --input case.json"},
			Flags: []capabilityFlag{
				{Name: "--input", Type: "string", Description: "入力JSONファイルのパス。標準入力は -", Required: true, Default: "-"},
			},
		},
		{
			Name:         "version",
			Summary:      "CLI とルールセットのバージョン情報を返します",
			OutputSchema: versionResultSchema.Name,
			Examples:     []string{"marume version"},
			Flags: []capabilityFlag{
				{Name: "--rules", Type: "string", Description: "ルールスナップショットのパス", Default: defaultRulesPath},
			},
		},
	}
}

func exitCodeDocs() []exitCodeDoc {
	return []exitCodeDoc{
		{Code: 0, Name: "OK", Description: "正常終了"},
		{Code: 1, Name: "INVALID_INPUT", Description: "入力値、引数、または年度不一致などの利用エラー"},
		{Code: 2, Name: "NO_CLASSIFICATION", Description: "分類結果が見つからない"},
		{Code: 3, Name: "FILE_NOT_FOUND", Description: "入力または rules ファイルが見つからない"},
		{Code: 4, Name: "RUNTIME_ERROR", Description: "その他の実行時エラー"},
		{Code: 5, Name: "RULE_DEFINITION_ERROR", Description: "ルール定義が不正"},
	}
}
