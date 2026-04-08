package cli

import (
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
)

var testdataCasePresets = map[string]domain.CaseInput{
	"ok": {
		CaseID:        "123",
		FiscalYear:    2026,
		Age:           intPtr(72),
		Sex:           "M",
		MainDiagnosis: "I219",
		Diagnoses:     []string{"I219"},
		Procedures:    []string{"K549"},
		Comorbidities: []string{},
	},
}

var testdataBatchPresets = map[string][]domain.CaseInput{
	"basic": {
		testdataCasePresets["ok"],
		{
			CaseID:        "124",
			FiscalYear:    2026,
			Age:           intPtr(75),
			Sex:           "F",
			MainDiagnosis: "I219",
			Diagnoses:     []string{"I219"},
			Procedures:    []string{},
			Comorbidities: []string{},
		},
	},
}

var testdataRulesPresets = map[string]domain.RuleSet{
	"minimal": {
		FiscalYear:  2026,
		RuleVersion: "2026.0.0-poc",
		BuildID:     "sample-minimal",
		BuiltAt:     "2026-04-08T00:00:00+09:00",
		Rules: []domain.Rule{
			{
				ID:       "R-2026-00010",
				Priority: 10,
				DPCCode:  "040080xx99x0xx",
				Conditions: []domain.Condition{
					{
						Type:     "main_diagnosis",
						Operator: "equals",
						Values:   []string{"I219"},
					},
					{
						Type:     "procedures",
						Operator: "contains_any",
						Values:   []string{"K549"},
					},
				},
			},
			{
				ID:       "R-2026-00020",
				Priority: 20,
				DPCCode:  "040081xx99x0xx",
				Conditions: []domain.Condition{
					{
						Type:     "main_diagnosis",
						Operator: "equals",
						Values:   []string{"I219"},
					},
					{
						Type:     "age",
						Operator: "gte",
						IntValue: intPtr(70),
					},
				},
			},
		},
	},
}

func runTestdata(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printTestdataUsage(stderr)
		return fmt.Errorf("%w: testdata にはサブコマンドを指定してください", errInvalidInput)
	}

	switch args[0] {
	case "--help", "-h", "help":
		printTestdataUsage(stdout)
		return nil
	case "case":
		return runTestdataCase(args[1:], stdout, stderr)
	case "batch":
		return runTestdataBatch(args[1:], stdout, stderr)
	case "rules":
		return runTestdataRules(args[1:], stdout, stderr)
	case "write":
		return runTestdataWrite(args[1:], stdout, stderr)
	default:
		printTestdataUsage(stderr)
		return fmt.Errorf("%w: 不明な testdata サブコマンド %q", errInvalidInput, args[0])
	}
}

func runTestdataCase(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata case", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata case [--preset ok] [--output <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "単一症例のサンプルJSONを生成します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintf(stderr, "利用可能な preset: %s\n", joinPresetNames(testdataCasePresets))
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	preset := flags.String("preset", "ok", "生成する症例プリセット名")
	outputPath := flags.String("output", "-", "出力先。標準出力に出す場合は -")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	value, ok := testdataCasePresets[*preset]
	if !ok {
		return unknownPresetError("case", *preset, presetNames(testdataCasePresets))
	}
	return writeJSONToPath(*outputPath, stdout, value)
}

func runTestdataBatch(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata batch [--preset basic] [--output <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "複数症例のサンプルJSONLを生成します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintf(stderr, "利用可能な preset: %s\n", joinPresetNames(testdataBatchPresets))
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	preset := flags.String("preset", "basic", "生成するバッチプリセット名")
	outputPath := flags.String("output", "-", "出力先。標準出力に出す場合は -")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	value, ok := testdataBatchPresets[*preset]
	if !ok {
		return unknownPresetError("batch", *preset, presetNames(testdataBatchPresets))
	}
	return writeJSONLToPath(*outputPath, stdout, value)
}

func runTestdataRules(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata rules", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata rules [--preset minimal] [--output <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "ルールセットのサンプルJSONを生成します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintf(stderr, "利用可能な preset: %s\n", joinPresetNames(testdataRulesPresets))
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	preset := flags.String("preset", "minimal", "生成するルールセットプリセット名")
	outputPath := flags.String("output", "-", "出力先。標準出力に出す場合は -")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}

	value, ok := testdataRulesPresets[*preset]
	if !ok {
		return unknownPresetError("rules", *preset, presetNames(testdataRulesPresets))
	}
	return writeJSONToPath(*outputPath, stdout, value)
}

func runTestdataWrite(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata write", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata write --dir <path>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "分類デモに使える sample 一式をディレクトリへ書き出します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "生成物:")
		fmt.Fprintln(stderr, "  case-ok.json")
		fmt.Fprintln(stderr, "  cases-basic.jsonl")
		fmt.Fprintln(stderr, "  rules-minimal.json")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	dir := flags.String("dir", ".local/marume-sample", "サンプル一式の出力先ディレクトリ")
	casePreset := flags.String("case-preset", "ok", "case 用プリセット名")
	batchPreset := flags.String("batch-preset", "basic", "batch 用プリセット名")
	rulesPreset := flags.String("rules-preset", "minimal", "rules 用プリセット名")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}
	if *dir == "" {
		return fmt.Errorf("%w: dir は必須です", errInvalidInput)
	}

	caseValue, ok := testdataCasePresets[*casePreset]
	if !ok {
		return unknownPresetError("case", *casePreset, presetNames(testdataCasePresets))
	}
	batchValue, ok := testdataBatchPresets[*batchPreset]
	if !ok {
		return unknownPresetError("batch", *batchPreset, presetNames(testdataBatchPresets))
	}
	rulesValue, ok := testdataRulesPresets[*rulesPreset]
	if !ok {
		return unknownPresetError("rules", *rulesPreset, presetNames(testdataRulesPresets))
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return fmt.Errorf("%w: 出力ディレクトリを作成できません: %v", errInvalidInput, err)
	}

	casePath := filepath.Join(*dir, fmt.Sprintf("case-%s.json", *casePreset))
	batchPath := filepath.Join(*dir, fmt.Sprintf("cases-%s.jsonl", *batchPreset))
	rulesPath := filepath.Join(*dir, fmt.Sprintf("rules-%s.json", *rulesPreset))

	if err := writePrettyJSONFile(casePath, caseValue); err != nil {
		return err
	}
	if err := writeJSONLFile(batchPath, batchValue); err != nil {
		return err
	}
	if err := writePrettyJSONFile(rulesPath, rulesValue); err != nil {
		return err
	}

	return writeJSON(stdout, map[string]any{
		"dir": *dir,
		"files": map[string]string{
			"case":  casePath,
			"batch": batchPath,
			"rules": rulesPath,
		},
	})
}

func writeJSONToPath(path string, stdout io.Writer, value any) error {
	if path == "-" {
		return writeJSON(stdout, value)
	}
	return writePrettyJSONFile(path, value)
}

func writePrettyJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: JSONの生成に失敗しました: %v", errRuleRuntime, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("%w: 出力ファイルを書き込めません: %v", errInvalidInput, err)
	}
	return nil
}

func writeJSONLToPath(path string, stdout io.Writer, lines []domain.CaseInput) error {
	if path == "-" {
		return writeJSONL(stdout, lines)
	}
	return writeJSONLFile(path, lines)
}

func writeJSONLFile(path string, lines []domain.CaseInput) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: 出力ファイルを書き込めません: %v", errInvalidInput, err)
	}
	writeErr := writeJSONL(file, lines)
	closeErr := file.Close()
	if writeErr != nil && closeErr != nil {
		return fmt.Errorf("%w: %v (close error: %v)", errInvalidInput, writeErr, closeErr)
	}
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("%w: 出力ファイルのクローズに失敗しました: %v", errInvalidInput, closeErr)
	}
	return nil
}

func writeJSONL(w io.Writer, lines []domain.CaseInput) error {
	encoder := json.NewEncoder(w)
	for _, line := range lines {
		if err := encoder.Encode(line); err != nil {
			return fmt.Errorf("%w: JSONLの生成に失敗しました: %v", errRuleRuntime, err)
		}
	}
	return nil
}

func printTestdataUsage(w io.Writer) {
	fmt.Fprintln(w, "使い方: marume testdata <サブコマンド> [フラグ]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "README の手順で使えるサンプル入力・ルールセットを生成します。")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "サブコマンド:")
	fmt.Fprintln(w, "  case    単一症例のサンプルJSONを生成する")
	fmt.Fprintln(w, "  batch   複数症例のサンプルJSONLを生成する")
	fmt.Fprintln(w, "  rules   ルールセットのサンプルJSONを生成する")
	fmt.Fprintln(w, "  write   classify/explain 用の一式をまとめて書き出す")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "例:")
	fmt.Fprintln(w, "  marume testdata case --preset ok")
	fmt.Fprintln(w, "  marume testdata batch --preset basic --output ./cases.jsonl")
	fmt.Fprintln(w, "  marume testdata rules --preset minimal --output ./rules.json")
	fmt.Fprintln(w, "  marume testdata write --dir ./.local/marume-sample")
}

func unknownPresetError(kind, preset string, names []string) error {
	return fmt.Errorf("%w: %s preset %q は未定義です (利用可能: %s)", errInvalidInput, kind, preset, joinPresetNamesFromSlice(names))
}

func joinPresetNames[K comparable, V any](values map[K]V) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, fmt.Sprint(name))
	}
	slices.Sort(names)
	return joinPresetNamesFromSlice(names)
}

func presetNames[K comparable, V any](values map[K]V) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, fmt.Sprint(name))
	}
	slices.Sort(names)
	return names
}

func joinPresetNamesFromSlice(names []string) string {
	return fmt.Sprintf("[%s]", strings.Join(names, ", "))
}
