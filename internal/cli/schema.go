package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

type schemaDoc struct {
	Name        string
	Title       string
	Description string
	Type        string
	Fields      []schemaField
	Example     any
}

type schemaField struct {
	Name        string
	Type        string
	Description string
	Required    bool
	ItemsType   string
	ItemSchema  map[string]any
	Const       any
	MinLength   *int
	Minimum     *int
}

func (d schemaDoc) jsonSchema() map[string]any {
	properties := make(map[string]any, len(d.Fields))
	required := make([]string, 0, len(d.Fields))
	for _, field := range d.Fields {
		property := map[string]any{
			"type":        field.Type,
			"description": field.Description,
		}
		if field.Type == "array" {
			switch {
			case field.ItemSchema != nil:
				property["items"] = field.ItemSchema
			case field.ItemsType != "":
				property["items"] = map[string]any{"type": field.ItemsType}
			}
		}
		if field.MinLength != nil {
			property["minLength"] = *field.MinLength
		}
		if field.Minimum != nil {
			property["minimum"] = *field.Minimum
		}
		if field.Const != nil {
			property["const"] = field.Const
		}
		properties[field.Name] = property
		if field.Required {
			required = append(required, field.Name)
		}
	}

	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  fmt.Sprintf("https://marume.dev/schema/%s.json", d.Name),
		"title":                d.Title,
		"description":          d.Description,
		"type":                 d.Type,
		"properties":           properties,
		"example":              d.Example,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		slices.Sort(required)
		schema["required"] = required
	}
	return schema
}

func (d schemaDoc) helpText() string {
	var b strings.Builder
	if d.Description != "" {
		b.WriteString(d.Description)
		b.WriteString("\n\n")
	}
	b.WriteString("入力スキーマ:\n")
	for _, field := range d.Fields {
		required := "任意"
		if field.Required {
			required = "必須"
		}
		b.WriteString(fmt.Sprintf("  - %s (%s, %s): %s\n", field.Name, field.TypeLabel(), required, field.Description))
	}
	b.WriteString("\n例:\n")
	example, err := json.MarshalIndent(d.Example, "  ", "  ")
	if err != nil {
		b.WriteString(fmt.Sprintf("  %v\n", d.Example))
		return b.String()
	}
	b.Write(example)
	b.WriteString("\n")
	return b.String()
}

func (f schemaField) TypeLabel() string {
	if f.Type != "array" || f.ItemsType == "" {
		return f.Type
	}
	return fmt.Sprintf("%s[%s]", f.Type, f.ItemsType)
}

var caseInputSchema = schemaDoc{
	Name:        "case-input",
	Title:       "Case Input",
	Description: "症例入力JSONです。classify / explain / validate と classify-batch の各行で共通です。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "case_id", Type: "string", Required: true, MinLength: intPtr(1), Description: "症例を識別するIDです。"},
		{Name: "fiscal_year", Type: "integer", Required: true, Minimum: intPtr(1), Description: "症例を評価する年度です。"},
		{Name: "age", Type: "integer", Minimum: intPtr(0), Description: "患者年齢です。0以上のみ許容します。"},
		{Name: "sex", Type: "string", Description: "患者性別です。現在のPOCでは未指定でも受け付けます。"},
		{Name: "main_diagnosis", Type: "string", Required: true, MinLength: intPtr(1), Description: "主傷病名コードです。"},
		{Name: "diagnoses", Type: "array", ItemsType: "string", Description: "診断コード一覧です。"},
		{Name: "procedures", Type: "array", ItemsType: "string", Description: "手術・処置コード一覧です。"},
		{Name: "comorbidities", Type: "array", ItemsType: "string", Description: "併存症コード一覧です。"},
	},
	Example: map[string]any{
		"case_id":        "123",
		"fiscal_year":    2026,
		"age":            72,
		"sex":            "male",
		"main_diagnosis": "I219",
		"diagnoses":      []string{"I219"},
		"procedures":     []string{"K549"},
		"comorbidities":  []string{},
	},
}

var classifyResultSchema = schemaDoc{
	Name:        "classify-result",
	Title:       "Classification Result",
	Description: "classify のJSON出力です。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "case_id", Type: "string", Required: true, MinLength: intPtr(1), Description: "入力症例IDです。"},
		{Name: "dpc_code", Type: "string", Required: true, Description: "分類されたDPCコードです。"},
		{Name: "version", Type: "string", Required: true, Description: "適用ルールセットのバージョンです。"},
		{Name: "matched_rule_id", Type: "string", Required: true, Description: "採用されたルールIDです。"},
		{Name: "reasons", Type: "array", ItemsType: "object", Required: true, Description: "分類理由の一覧です。"},
	},
	Example: map[string]any{
		"case_id":         "123",
		"dpc_code":        "040080xx99x0xx",
		"version":         "2026.0.0-poc",
		"matched_rule_id": "R-2026-00010",
		"reasons": []map[string]any{
			{
				"code":       "MAIN_DIAGNOSIS_MATCH",
				"message":    "主傷病名が I219 に一致しました",
				"message_en": "main diagnosis matched I219",
			},
		},
	},
}

var explainResultSchema = schemaDoc{
	Name:        "explain-result",
	Title:       "Explain Result",
	Description: "explain のJSON出力です。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "selected_rule", Type: "string", Required: true, Description: "採用ルールIDです。分類不能時は空文字です。"},
		{Name: "candidate_rules", Type: "array", ItemsType: "object", Required: true, Description: "候補ルールの評価一覧です。"},
	},
	Example: map[string]any{
		"selected_rule": "R-2026-00010",
		"candidate_rules": []map[string]any{
			{
				"rule_id":         "R-2026-00010",
				"priority":        10,
				"dpc_code":        "040080xx99x0xx",
				"matched":         true,
				"matched_reasons": []map[string]any{{"code": "MAIN_DIAGNOSIS_MATCH", "message": "主傷病名が I219 に一致しました"}},
			},
		},
	},
}

var batchResultSchema = schemaDoc{
	Name:        "batch-result",
	Title:       "Batch Classification Result",
	Description: "classify-batch が各行に返す JSONL オブジェクトです。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "line_no", Type: "integer", Required: true, Description: "入力JSONLの行番号です。"},
		{Name: "case_id", Type: "string", Description: "入力症例IDです。JSONデコード前エラー時は省略されます。"},
		{Name: "status", Type: "string", Required: true, Description: "ok または error です。"},
		{Name: "result", Type: "object", Description: "status=ok のときの分類結果です。"},
		{Name: "error", Type: "object", Description: "status=error のときのエラー情報です。"},
	},
	Example: map[string]any{
		"line_no": 1,
		"case_id": "123",
		"status":  "ok",
		"result": map[string]any{
			"case_id":         "123",
			"dpc_code":        "040080xx99x0xx",
			"version":         "2026.0.0-poc",
			"matched_rule_id": "R-2026-00010",
			"reasons":         []map[string]any{{"code": "MAIN_DIAGNOSIS_MATCH", "message": "主傷病名が I219 に一致しました"}},
		},
	},
}

var validateResultSchema = schemaDoc{
	Name:        "validate-result",
	Title:       "Validate Result",
	Description: "validate のJSON出力です。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "status", Type: "string", Required: true, Const: "ok", Description: "検証結果です。現在は ok のみ返します。"},
		{Name: "case_id", Type: "string", Required: true, Description: "入力症例IDです。"},
	},
	Example: map[string]any{
		"status":  "ok",
		"case_id": "123",
	},
}

var versionResultSchema = schemaDoc{
	Name:        "version-result",
	Title:       "Version Result",
	Description: "version のJSON出力です。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "cli_version", Type: "string", Required: true, Description: "CLIバージョンです。"},
		{Name: "rule_version", Type: "string", Required: true, Description: "ルールセットのバージョンです。"},
		{Name: "build_id", Type: "string", Required: true, Description: "ルールセットのビルドIDです。"},
		{Name: "built_at", Type: "string", Required: true, Description: "ルールセットのビルド時刻です。"},
	},
	Example: map[string]any{
		"cli_version":  "dev",
		"rule_version": "2026.0.0-poc",
		"build_id":     "snapshot-20260408",
		"built_at":     "2026-04-08T10:00:00+09:00",
	},
}

var capabilitiesResultSchema = schemaDoc{
	Name:        "capabilities-result",
	Title:       "Capabilities Result",
	Description: "capabilities のJSON出力です。",
	Type:        "object",
	Fields: []schemaField{
		{Name: "cli_version", Type: "string", Required: true, Description: "CLIバージョンです。"},
		{Name: "default_rule_path", Type: "string", Required: true, Description: "rules 未指定時に参照するデフォルトの snapshot パスです。"},
		{
			Name:        "global_flags",
			Type:        "array",
			Required:    true,
			Description: "グローバルフラグ一覧です。",
			ItemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"type":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"required":    map[string]any{"type": "boolean"},
					"default":     map[string]any{"type": "string"},
				},
				"required":             []string{"description", "name", "type"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "commands",
			Type:        "array",
			Required:    true,
			Description: "コマンド一覧と入出力契約です。",
			ItemSchema:  capabilityCommandSchema(2),
		},
		{
			Name:        "schemas",
			Type:        "array",
			Required:    true,
			Description: "利用可能なスキーマ名一覧です。",
			ItemSchema:  map[string]any{"type": "string"},
		},
		{
			Name:        "exit_codes",
			Type:        "array",
			Required:    true,
			Description: "終了コード一覧です。",
			ItemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"code":        map[string]any{"type": "integer"},
					"name":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
				},
				"required":             []string{"code", "description", "name"},
				"additionalProperties": false,
			},
		},
	},
	Example: map[string]any{
		"cli_version":       "dev",
		"default_rule_path": "rules/rules-2026.sqlite",
		"global_flags": []map[string]any{
			{"name": "--json-errors", "type": "bool", "description": "失敗時に構造化エラーJSONを標準エラーへ出します"},
		},
		"commands": []map[string]any{
			{"name": "classify", "summary": "単一症例を分類します", "input_schema": "case-input", "output_schema": "classify-result"},
			{
				"name":    "schema",
				"summary": "JSON Schema を返します",
				"positional_args": []map[string]any{
					{"name": "name", "type": "string", "description": "返したいスキーマ名。--list を使わない場合に指定する"},
				},
			},
			{
				"name":    "testdata",
				"summary": "サンプル入力とサンプルルールを生成します",
				"subcommands": []map[string]any{
					{"name": "write", "summary": "サンプル一式をディレクトリへ書き出します", "examples": []string{"marume testdata write --dir ./.local/marume-sample"}},
				},
			},
		},
		"schemas": []string{"case-input", "classify-result"},
		"exit_codes": []map[string]any{
			{"code": 0, "name": "OK", "description": "正常終了"},
			{"code": 1, "name": "INVALID_INPUT", "description": "入力または CLI 引数が不正"},
		},
	},
}

var schemaRegistry = map[string]schemaDoc{
	caseInputSchema.Name:          caseInputSchema,
	classifyResultSchema.Name:     classifyResultSchema,
	explainResultSchema.Name:      explainResultSchema,
	batchResultSchema.Name:        batchResultSchema,
	validateResultSchema.Name:     validateResultSchema,
	versionResultSchema.Name:      versionResultSchema,
	capabilitiesResultSchema.Name: capabilitiesResultSchema,
}

func writeSchemaHelp(w io.Writer, doc schemaDoc) {
	fmt.Fprintln(w, doc.helpText())
}

func intPtr(v int) *int {
	return &v
}

func capabilityCommandSchema(depth int) map[string]any {
	flagSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"required":    map[string]any{"type": "boolean"},
			"default":     map[string]any{"type": "string"},
		},
		"required":             []string{"description", "name", "type"},
		"additionalProperties": false,
	}
	argSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"required":    map[string]any{"type": "boolean"},
		},
		"required":             []string{"description", "name", "type"},
		"additionalProperties": false,
	}
	commandSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":          map[string]any{"type": "string"},
			"summary":       map[string]any{"type": "string"},
			"input_schema":  map[string]any{"type": "string"},
			"output_schema": map[string]any{"type": "string"},
			"examples": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"flags": map[string]any{
				"type":  "array",
				"items": flagSchema,
			},
			"positional_args": map[string]any{
				"type":  "array",
				"items": argSchema,
			},
		},
		"required":             []string{"name", "summary"},
		"additionalProperties": false,
	}
	if depth > 0 {
		commandSchema["properties"].(map[string]any)["subcommands"] = map[string]any{
			"type":  "array",
			"items": capabilityCommandSchema(depth - 1),
		}
	}
	return commandSchema
}
