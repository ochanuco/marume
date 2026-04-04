# marume

DPC診断群分類の Go 製 CLI POC です。データ基盤より先に、ローカルで症例 JSON を評価できる最小構成を置いています。

現時点では SQLite ではなく JSON ルールセットを読んでいます。`RuleStore` 抽象を挟んでいるので、後から SQLite 実装に差し替える前提です。

## POC の範囲

- `classify`: 単票分類
- `classify-batch`: JSONL による一括分類
- `explain`: 候補ルールと一致理由の確認
- `validate`: 入力の最低限検証
- `version`: CLI とルールセット情報の表示

## 前提条件

- Go 1.26.1
- `mise` がインストール済みであること
- `mise install` でこのリポジトリ用の Go ツールチェインを揃えること

## セットアップ

```bash
mise install
go build ./cmd/marume
```

Python でデータ収集・整形を進める場合は、以下を使います。

```bash
uv venv --python 3.13
uv sync
```

workflow JSON を使う場合は、`workflows/dpc_2026_mhlw.json` を起点にします。

```bash
uv run python scripts/run_workflow.py --workflow workflows/dpc_2026_mhlw.json
```

最小の Python データパイプラインは以下の 4 段階です。

```bash
uv run python scripts/fetch_mhlw.py --url https://www.mhlw.go.jp/stf/newpage_67729.html --output-dir .local/raw/mhlw
uv run python scripts/extract_dpc_pdf.py --manifest .local/raw/mhlw/manifest.json --output .local/raw/mhlw/dpc_rules.csv
uv run python scripts/transform_dpc.py --manifest .local/raw/mhlw/manifest.json --fiscal-year 2026 --output .local/intermediate/dpc-2026.json
uv run python scripts/build_sqlite.py --input .local/intermediate/dpc-2026.json --output .local/sqlite/rules-2026.sqlite
```

`transform_dpc.py` は `--fiscal-year` を必須にしています。
`--source-url` は未指定時、`--manifest` に含まれる `page_url` を使います。

## 使い方

```bash
./marume classify --rules testdata/rules/rules-2026.json --input testdata/cases/case-ok.json
./marume classify-batch --rules testdata/rules/rules-2026.json --input testdata/cases/cases.jsonl --output result.jsonl
./marume explain --rules testdata/rules/rules-2026.json --input testdata/cases/case-age-diagnosis-without-procedure.json
./marume validate --input testdata/cases/case-ok.json
./marume version --rules testdata/rules/rules-2026.json
```

`stdin` から読む場合は `--input -` を使います。
`classify-batch` は `JSONL` を 1 行ずつ読み、結果も `JSONL` で返します。
`explain` は分類不能でも候補ルールの JSON を返し、`selected_rule` は空文字、終了コードは `0` です。

### `classify` の出力例

```json
{
  "case_id": "123",
  "dpc_code": "040080xx99x0xx",
  "version": "2026.0.0-poc",
  "matched_rule_id": "R-2026-00010",
  "reasons": [
    {
      "code": "MAIN_DIAGNOSIS_MATCH",
      "message": "主傷病名が I219 に一致しました",
      "message_en": "main diagnosis matched I219"
    },
    {
      "code": "PROCEDURE_MATCH",
      "message": "手術・処置コードに K549 が含まれています",
      "message_en": "procedures contains K549"
    }
  ]
}
```

### `classify-batch` の出力例

```json
{"line_no":1,"case_id":"123","status":"ok","result":{"case_id":"123","dpc_code":"040080xx99x0xx","version":"2026.0.0-poc","matched_rule_id":"R-2026-00010","reasons":[{"code":"MAIN_DIAGNOSIS_MATCH","message":"主傷病名が I219 に一致しました","message_en":"main diagnosis matched I219"},{"code":"PROCEDURE_MATCH","message":"手術・処置コードに K549 が含まれています","message_en":"procedures contains K549"}]}}
{"line_no":2,"case_id":"999","status":"error","error":{"code":"NO_CLASSIFICATION","message":"症例 999 に一致する分類が見つかりません","message_en":"no classification matched for case 999"}}
{"line_no":3,"status":"error","error":{"code":"INVALID_JSON","message":"3 行目のJSONが不正です: <decoder error>","message_en":"invalid JSON at line 3: <decoder error>"}}
```

## 現在の構造

- `cmd/marume`: エントリポイント
- `internal/cli`: コマンド処理
- `internal/evaluator`: ルール評価器
- `internal/store`: ルールセット取得層
- `testdata/rules`: POC 用ルールセット

## 次の差し替えポイント

1. `internal/store` に SQLite 実装を追加
2. `validate` を JSON Schema か独自ルールで強化
3. Dataform / BigQuery 由来のスナップショット生成に接続する
4. Cobra ベースのコマンド体系に移行

## SQLite 化メモ

- 基礎データの収集元と最小スキーマ案は `docs/sqlite-data-sourcing.md` を参照
