package cli

import (
	"cmp"
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
	testdataCasePresets  = []string{"ok"}
	testdataBatchPresets = []string{"basic"}
	testdataRulesPresets = []string{"minimal"}
)

const testdataMinimalRuleCount = 2

func runTestdata(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printTestdataUsage(stderr)
		return fmt.Errorf("%w: testdata にはサブコマンドを指定してください", errInvalidInput)
	}

	switch args[0] {
	case "--help", "-h", "help":
		printTestdataUsage(stdout)
		return nil
	case "case":
		return runTestdataCase(ctx, args[1:], stdout, stderr)
	case "batch":
		return runTestdataBatch(ctx, args[1:], stdout, stderr)
	case "rules":
		return runTestdataRules(ctx, args[1:], stdout, stderr)
	case "write":
		return runTestdataWrite(ctx, args[1:], stdout, stderr)
	default:
		printTestdataUsage(stderr)
		return fmt.Errorf("%w: 不明な testdata サブコマンド %q", errInvalidInput, args[0])
	}
}

func runTestdataCase(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata case", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata case [--preset ok] [--rules <path>] [--output <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "rules snapshot から単一症例のサンプルJSONを生成します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintf(stderr, "利用可能な preset: %s\n", joinNames(testdataCasePresets))
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	preset := flags.String("preset", "ok", "生成する症例プリセット名")
	rulesPath := flags.String("rules", defaultRulePath, "サンプル生成元のルールスナップショット (JSON または SQLite)")
	outputPath := flags.String("output", "-", "出力先。標準出力に出す場合は -")
	verbose := flags.Bool("verbose", false, "サンプル生成でスキップしたルールを標準エラーに出す")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}
	if !containsName(testdataCasePresets, *preset) {
		return unknownPresetError("case", *preset, testdataCasePresets)
	}

	sourceRuleSet, err := loadTestdataSourceRuleSet(ctx, flags, *rulesPath)
	if err != nil {
		return err
	}
	sampleRuleSet, err := buildTestdataRuleSet(sourceRuleSet, testdataMinimalRuleCount, *verbose, stderr)
	if err != nil {
		return err
	}
	value, err := buildCasePreset(*preset, sampleRuleSet)
	if err != nil {
		return err
	}
	return writeJSONToPath(*outputPath, stdout, value)
}

func runTestdataBatch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata batch [--preset basic] [--rules <path>] [--output <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "rules snapshot から複数症例のサンプルJSONLを生成します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintf(stderr, "利用可能な preset: %s\n", joinNames(testdataBatchPresets))
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	preset := flags.String("preset", "basic", "生成するバッチプリセット名")
	rulesPath := flags.String("rules", defaultRulePath, "サンプル生成元のルールスナップショット (JSON または SQLite)")
	outputPath := flags.String("output", "-", "出力先。標準出力に出す場合は -")
	verbose := flags.Bool("verbose", false, "サンプル生成でスキップしたルールを標準エラーに出す")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}
	if !containsName(testdataBatchPresets, *preset) {
		return unknownPresetError("batch", *preset, testdataBatchPresets)
	}

	sourceRuleSet, err := loadTestdataSourceRuleSet(ctx, flags, *rulesPath)
	if err != nil {
		return err
	}
	sampleRuleSet, err := buildTestdataRuleSet(sourceRuleSet, testdataMinimalRuleCount, *verbose, stderr)
	if err != nil {
		return err
	}
	value, err := buildBatchPreset(*preset, sampleRuleSet)
	if err != nil {
		return err
	}
	return writeJSONLToPath(*outputPath, stdout, value)
}

func runTestdataRules(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata rules", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata rules [--preset minimal] [--rules <path>] [--output <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "rules snapshot からサンプル用の最小ルールセットJSONを生成します。")
		fmt.Fprintln(stderr, "")
		fmt.Fprintf(stderr, "利用可能な preset: %s\n", joinNames(testdataRulesPresets))
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "フラグ:")
		flags.PrintDefaults()
	}

	preset := flags.String("preset", "minimal", "生成するルールセットプリセット名")
	rulesPath := flags.String("rules", defaultRulePath, "サンプル生成元のルールスナップショット (JSON または SQLite)")
	outputPath := flags.String("output", "-", "出力先。標準出力に出す場合は -")
	verbose := flags.Bool("verbose", false, "サンプル生成でスキップしたルールを標準エラーに出す")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	if err := rejectExtraArgs(flags); err != nil {
		return err
	}
	if !containsName(testdataRulesPresets, *preset) {
		return unknownPresetError("rules", *preset, testdataRulesPresets)
	}

	sourceRuleSet, err := loadTestdataSourceRuleSet(ctx, flags, *rulesPath)
	if err != nil {
		return err
	}
	value, err := buildTestdataRuleSet(sourceRuleSet, testdataMinimalRuleCount, *verbose, stderr)
	if err != nil {
		return err
	}
	return writeJSONToPath(*outputPath, stdout, value)
}

func runTestdataWrite(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("testdata write", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "使い方: marume testdata write --dir <path> [--rules <path>]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "rules snapshot から分類デモに使える sample 一式をディレクトリへ書き出します。")
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
	rulesPath := flags.String("rules", defaultRulePath, "サンプル生成元のルールスナップショット (JSON または SQLite)")
	casePreset := flags.String("case-preset", "ok", "case 用プリセット名")
	batchPreset := flags.String("batch-preset", "basic", "batch 用プリセット名")
	rulesPreset := flags.String("rules-preset", "minimal", "rules 用プリセット名")
	verbose := flags.Bool("verbose", false, "サンプル生成でスキップしたルールを標準エラーに出す")
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
	if !containsName(testdataCasePresets, *casePreset) {
		return unknownPresetError("case", *casePreset, testdataCasePresets)
	}
	if !containsName(testdataBatchPresets, *batchPreset) {
		return unknownPresetError("batch", *batchPreset, testdataBatchPresets)
	}
	if !containsName(testdataRulesPresets, *rulesPreset) {
		return unknownPresetError("rules", *rulesPreset, testdataRulesPresets)
	}

	sourceRuleSet, err := loadTestdataSourceRuleSet(ctx, flags, *rulesPath)
	if err != nil {
		return err
	}
	sampleRuleSet, err := buildTestdataRuleSet(sourceRuleSet, testdataMinimalRuleCount, *verbose, stderr)
	if err != nil {
		return err
	}
	caseValue, err := buildCasePreset(*casePreset, sampleRuleSet)
	if err != nil {
		return err
	}
	batchValue, err := buildBatchPreset(*batchPreset, sampleRuleSet)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return fmt.Errorf("%w: 出力ディレクトリを作成できません: %v", errInvalidInput, err)
	}

	casePath := filepath.Join(*dir, fmt.Sprintf("case-%s.json", *casePreset))
	batchPath := filepath.Join(*dir, fmt.Sprintf("cases-%s.jsonl", *batchPreset))
	rulesOutputPath := filepath.Join(*dir, fmt.Sprintf("rules-%s.json", *rulesPreset))

	if err := writePrettyJSONFile(casePath, caseValue); err != nil {
		return err
	}
	if err := writeJSONLFile(batchPath, batchValue); err != nil {
		return err
	}
	if err := writePrettyJSONFile(rulesOutputPath, sampleRuleSet); err != nil {
		return err
	}

	return writeJSON(stdout, map[string]any{
		"dir": *dir,
		"files": map[string]string{
			"case":  casePath,
			"batch": batchPath,
			"rules": rulesOutputPath,
		},
	})
}

func loadTestdataSourceRuleSet(ctx context.Context, flags *flag.FlagSet, requestedPath string) (domain.RuleSet, error) {
	ruleStore, err := store.NewRuleStore(resolveRulesPath(flags, requestedPath))
	if err != nil {
		return domain.RuleSet{}, fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	ruleSet, err := ruleStore.ReadRuleSet(ctx)
	if err != nil {
		return domain.RuleSet{}, err
	}
	if err := evaluator.ValidateRuleSet(ruleSet); err != nil {
		return domain.RuleSet{}, err
	}
	return ruleSet, nil
}

func buildTestdataRuleSet(source domain.RuleSet, maxRules int, verbose bool, stderr io.Writer) (domain.RuleSet, error) {
	selected := make([]domain.Rule, 0, maxRules)
	for _, rule := range sortRulesForTestdata(source.Rules) {
		if len(selected) == maxRules {
			break
		}
		if _, err := synthesizeCaseForRule(source.FiscalYear, rule, fmt.Sprintf("sample-%s", rule.ID)); err != nil {
			if verbose && stderr != nil {
				fmt.Fprintf(stderr, "skipping rule %s: %v\n", rule.ID, err)
			}
			continue
		}
		selected = append(selected, rule)
	}
	if len(selected) == 0 {
		return domain.RuleSet{}, fmt.Errorf("%w: サンプル生成に使えるルールが見つかりません", errInvalidInput)
	}
	return domain.RuleSet{
		FiscalYear:  source.FiscalYear,
		RuleVersion: source.RuleVersion,
		BuildID:     source.BuildID,
		BuiltAt:     source.BuiltAt,
		Rules:       selected,
	}, nil
}

func buildCasePreset(preset string, ruleSet domain.RuleSet) (domain.CaseInput, error) {
	switch preset {
	case "ok":
		if len(ruleSet.Rules) == 0 {
			return domain.CaseInput{}, fmt.Errorf("%w: case サンプル生成に使えるルールがありません", errInvalidInput)
		}
		return synthesizeCaseForRule(ruleSet.FiscalYear, ruleSet.Rules[0], "sample-ok")
	default:
		return domain.CaseInput{}, unknownPresetError("case", preset, testdataCasePresets)
	}
}

func buildBatchPreset(preset string, ruleSet domain.RuleSet) ([]domain.CaseInput, error) {
	switch preset {
	case "basic":
		cases := make([]domain.CaseInput, 0, len(ruleSet.Rules))
		for idx, rule := range ruleSet.Rules {
			item, err := synthesizeCaseForRule(ruleSet.FiscalYear, rule, fmt.Sprintf("sample-%02d", idx+1))
			if err != nil {
				return nil, err
			}
			cases = append(cases, item)
		}
		return cases, nil
	default:
		return nil, unknownPresetError("batch", preset, testdataBatchPresets)
	}
}

func synthesizeCaseForRule(fiscalYear int, rule domain.Rule, caseID string) (domain.CaseInput, error) {
	input := domain.CaseInput{
		CaseID:        caseID,
		FiscalYear:    fiscalYear,
		Diagnoses:     []string{},
		Procedures:    []string{},
		Comorbidities: []string{},
	}

	var minAge *int
	var maxAge *int

	for _, condition := range rule.Conditions {
		switch condition.Type {
		case "main_diagnosis":
			if condition.Operator != "equals" || len(condition.Values) == 0 {
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の main_diagnosis 条件からサンプルを作れません", errInvalidInput, rule.ID)
			}
			input.MainDiagnosis = condition.Values[0]
		case "diagnoses":
			if condition.Operator != "contains_any" || len(condition.Values) == 0 {
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の diagnoses 条件からサンプルを作れません", errInvalidInput, rule.ID)
			}
			input.Diagnoses = uniqueStrings(append(input.Diagnoses, condition.Values[0]))
		case "procedures":
			if condition.Operator != "contains_any" || len(condition.Values) == 0 {
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の procedures 条件からサンプルを作れません", errInvalidInput, rule.ID)
			}
			input.Procedures = uniqueStrings(append(input.Procedures, condition.Values[0]))
		case "comorbidities":
			if condition.Operator != "contains_any" || len(condition.Values) == 0 {
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の comorbidities 条件からサンプルを作れません", errInvalidInput, rule.ID)
			}
			input.Comorbidities = uniqueStrings(append(input.Comorbidities, condition.Values[0]))
		case "sex":
			if condition.Operator != "equals" || len(condition.Values) == 0 {
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の sex 条件からサンプルを作れません", errInvalidInput, rule.ID)
			}
			input.Sex = strings.ToUpper(condition.Values[0])
		case "age":
			if condition.IntValue == nil {
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の age 条件に int_value がありません", errInvalidInput, rule.ID)
			}
			switch condition.Operator {
			case "gte":
				minAge = maxIntPtr(minAge, condition.IntValue)
			case "lte":
				maxAge = minIntPtr(maxAge, condition.IntValue)
			default:
				return domain.CaseInput{}, fmt.Errorf("%w: rule %s の age 条件 %q からサンプルを作れません", errInvalidInput, rule.ID, condition.Operator)
			}
		default:
			return domain.CaseInput{}, fmt.Errorf("%w: rule %s の条件 %q はサンプル生成対象外です", errInvalidInput, rule.ID, condition.Type)
		}
	}

	if input.MainDiagnosis == "" {
		return domain.CaseInput{}, fmt.Errorf("%w: rule %s に main_diagnosis 条件がありません", errInvalidInput, rule.ID)
	}
	// Keep the synthesized sample aligned with the rule's main diagnosis even when
	// the source rule does not have an explicit diagnoses condition.
	if len(input.Diagnoses) == 0 {
		input.Diagnoses = []string{input.MainDiagnosis}
	}
	if minAge != nil || maxAge != nil {
		age, err := chooseAge(minAge, maxAge, rule.ID)
		if err != nil {
			return domain.CaseInput{}, err
		}
		input.Age = &age
	}

	return input, nil
}

func chooseAge(minAge, maxAge *int, ruleID string) (int, error) {
	switch {
	case minAge != nil && maxAge != nil && *minAge > *maxAge:
		return 0, fmt.Errorf("%w: rule %s の age 条件が矛盾しています", errInvalidInput, ruleID)
	case minAge != nil:
		return *minAge, nil
	case maxAge != nil:
		return *maxAge, nil
	default:
		// Defensive: callers only invoke chooseAge when at least one bound is non-nil.
		return 0, fmt.Errorf("%w: rule %s から age を選べません", errInvalidInput, ruleID)
	}
}

func maxIntPtr(current, candidate *int) *int {
	if candidate == nil {
		return current
	}
	if current == nil || *candidate > *current {
		value := *candidate
		return &value
	}
	return current
}

func minIntPtr(current, candidate *int) *int {
	if candidate == nil {
		return current
	}
	if current == nil || *candidate < *current {
		value := *candidate
		return &value
	}
	return current
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sortRulesForTestdata(rules []domain.Rule) []domain.Rule {
	sorted := append([]domain.Rule(nil), rules...)
	slices.SortFunc(sorted, func(a, b domain.Rule) int {
		if diff := cmp.Compare(a.Priority, b.Priority); diff != 0 {
			return diff
		}
		return strings.Compare(a.ID, b.ID)
	})
	return sorted
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
		return fmt.Errorf("%w: 出力ファイルを書き込めません: %v", errRuleRuntime, err)
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
	fmt.Fprintln(w, "README の手順で使えるサンプル入力・ルールセットを、採用済み rules snapshot から生成します。")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "サブコマンド:")
	fmt.Fprintln(w, "  case    単一症例のサンプルJSONを生成する")
	fmt.Fprintln(w, "  batch   複数症例のサンプルJSONLを生成する")
	fmt.Fprintln(w, "  rules   ルールセットのサンプルJSONを生成する")
	fmt.Fprintln(w, "  write   classify/explain 用の一式をまとめて書き出す")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "例:")
	fmt.Fprintln(w, "  marume testdata case --rules rules/rules-2026.sqlite --preset ok")
	fmt.Fprintln(w, "  marume testdata batch --rules rules/rules-2026.sqlite --preset basic --output ./cases.jsonl")
	fmt.Fprintln(w, "  marume testdata rules --rules rules/rules-2026.sqlite --preset minimal --output ./rules.json")
	fmt.Fprintln(w, "  marume testdata write --rules rules/rules-2026.sqlite --dir ./.local/marume-sample")
}

func unknownPresetError(kind, preset string, names []string) error {
	return fmt.Errorf("%w: %s preset %q は未定義です (利用可能: %s)", errInvalidInput, kind, preset, joinNames(names))
}

func containsName(values []string, target string) bool {
	return slices.Contains(values, target)
}

func joinNames(names []string) string {
	sorted := append([]string(nil), names...)
	slices.Sort(sorted)
	return fmt.Sprintf("[%s]", strings.Join(sorted, ", "))
}
