# DPC Coding Sample Cases

厚労省公開の `DPC/PDPS 傷病名コーディングテキスト` から、`marume` で扱いやすいサンプル症例候補を抽出するためのメモです。

このドキュメントが対象にするのは、生の DPC 個票ではなく、公開 PDF に載っているコーディング事例です。

## 目的

- `DPC/PDPS 傷病名コーディングテキスト` の事例集を機械的に抜く
- `marume` の入力 JSON に近い形へ落とし込む
- テストデータやデモ用の症例候補を大量に確保する

## 対象資料

- 厚労省 `DPC/PDPS 傷病名コーディングテキスト 改定版（第6版）`
  - https://www.mhlw.go.jp/content/12404000/001394024.pdf

抽出対象は、付録の `DPC上６桁別 注意すべきDPCコーディングの事例集` です。

## 追加したスクリプト

- `scripts/extract_dpc_coding_cases.py`
  - PDF から事例を抽出して JSON 化する
- `scripts/build_dpc_sample_cases.py`
  - 抽出済み JSON から `marume` 向けの症例候補 JSON を作る

ロジック本体:

- `src/marume_data/coding_text.py`
- `src/marume_data/sample_cases.py`

## 使い方

依存同期:

```bash
uv sync
```

### mise タスクを使った実行

```bash
mise run coding-cases
```

このコマンドで、`scripts/extract_dpc_coding_cases.py` と `scripts/build_dpc_sample_cases.py` が順に実行されます。
生成される `case-input` JSONL は、`marume validate` / `classify-batch` に渡せる縦切り確認用の候補です。
`main_diagnosis` が空の候補は JSONL から除外し、レポートに skip 件数として残します。

必要なら次の環境変数で対象 PDF や出力バージョン、会計年度を上書きできます（mise タスク実行時のみ有効）。

- `DPC_CODING_TEXT_URL`
- `DPC_CODING_TEXT_VERSION`
- `DPC_CODING_TEXT_START_PAGE`
- `DPC_FISCAL_YEAR`

**注意**: これらの環境変数は `mise run coding-cases` でタスクを実行する場合のみ有効です。`mise.toml` 内の `${DPC_CODING_TEXT_URL:-...}` などのシェルパラメータ展開により、環境変数値がスクリプトの CLI 引数に変換されます。Python スクリプトを `uv run` で直接実行する場合や、`scripts/extract_dpc_coding_cases.py` / `scripts/build_dpc_sample_cases.py` を直接実行する場合は、これらの環境変数は無視されるため、明示的にコマンドライン引数として渡す必要があります。

## 手動実行

### 1. PDF から事例を抽出

```bash
uv run python scripts/extract_dpc_coding_cases.py \
  --url https://www.mhlw.go.jp/content/12404000/001394024.pdf \
  --output .local/dpc-coding-cases-v6.json \
  --start-page 35
```

ローカル PDF を使う場合:

```bash
uv run python scripts/extract_dpc_coding_cases.py \
  --input-pdf /path/to/001394024.pdf \
  --output .local/dpc-coding-cases-v6.json \
  --start-page 35
```

### 2. `marume` 向け候補へ整形

```bash
uv run python scripts/build_dpc_sample_cases.py \
  --input .local/dpc-coding-cases-v6.json \
  --output .local/dpc-sample-case-candidates-v6.json \
  --case-input-jsonl .local/dpc-case-input-candidates-v6.jsonl \
  --report .local/dpc-case-input-candidates-v6-report.json \
  --fiscal-year 2026
```

## 出力

### 抽出結果

`.local/dpc-coding-cases-v6.json`

各要素の主な項目:

- `dpc_code`
- `dpc_name`
- `example_text`
- `guidance_text`
- `raw_text`
- `source_page`

### 整形後の症例候補

`.local/dpc-sample-case-candidates-v6.json`

各要素の主な項目:

- `case_id`
- `fiscal_year`
- `dpc_code_6`
- `dpc_name`
- `main_diagnosis`
- `diagnoses`
- `procedures`
- `comorbidities`
- `age`
- `sex`
- `source_page`
- `example_text`
- `guidance_text`
- `notes`

### `marume` 入力候補

`.local/dpc-case-input-candidates-v6.jsonl`

`marume` の `case-input` に合わせた JSONL です。上の症例候補から次の項目だけを取り出します。

- `case_id`
- `fiscal_year`
- `age`
- `sex`
- `main_diagnosis`
- `diagnoses`
- `procedures`
- `comorbidities`

`main_diagnosis` が空の候補は `marume validate` を通せないため、この JSONL からは除外します。
`age` は任意項目です。実装上は `src/marume_data/sample_cases.py` の `_case_input_payload` で、値が未設定のときは JSONL から省略します。

生成件数と除外理由は `.local/dpc-case-input-candidates-v6-report.json` に出します。

最低限の入力検証は次のように確認できます。

```bash
head -n 1 .local/dpc-case-input-candidates-v6.jsonl | ./marume validate --input -
```

## 現状の限界

- PDF のレイアウト崩れにより、`dpc_name` に事例文が一部食い込むケースがある
- `main_diagnosis` は、ガイダンス文中の「医療資源病名」や「本分類が該当」「を選択する」などのパターンに一致するコードを優先し、それらが無い場合のみ全 ICD コードの最後の要素をフォールバックとして採用している
- `procedures` は K コード等の単純抽出で、網羅性は高くない
- `age` と `sex` は元資料に十分な記述がないため空のまま

つまり、現状の JSON は「そのまま正解データ」ではなく、`marume` 用に人手で仕上げる前段の素材です。

## テスト

```bash
uv run pytest tests/test_coding_text.py tests/test_sample_cases.py tests/test_transform.py tests/test_extract_dpc_coding_cases_script.py
```

## 今後の改善候補

- `dpc_name` と `example_text` の境界補正を増やす
- `入院契機病名` / `医療資源病名` / `入院後発症疾患` を構造化して分離する
- `testdata/cases/` に直接展開できるように、候補 JSON から症例ファイル群を生成する
