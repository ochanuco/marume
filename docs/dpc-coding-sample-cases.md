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

必要なら次の環境変数で対象 PDF や出力バージョン、会計年度を上書きできます。

- `DPC_CODING_TEXT_URL`
- `DPC_CODING_TEXT_VERSION`
- `DPC_CODING_TEXT_START_PAGE`
- `DPC_FISCAL_YEAR`

### 手動実行

### 1. PDF から事例を抽出

```bash
uv run python scripts/extract_dpc_coding_cases.py \
  --url https://www.mhlw.go.jp/content/12404000/001394024.pdf \
  --output .local/dpc-coding-cases-v6.json \
  --start-page 36
```

ローカル PDF を使う場合:

```bash
uv run python scripts/extract_dpc_coding_cases.py \
  --input-pdf /path/to/001394024.pdf \
  --output .local/dpc-coding-cases-v6.json \
  --start-page 36
```

### 2. `marume` 向け候補へ整形

```bash
uv run python scripts/build_dpc_sample_cases.py \
  --input .local/dpc-coding-cases-v6.json \
  --output .local/dpc-sample-case-candidates-v6.json \
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
- `example_text`
- `guidance_text`
- `notes`

## 現状の限界

- PDF のレイアウト崩れにより、`dpc_name` に事例文が一部食い込むケースがある
- `main_diagnosis` は、本文・対応文から見つかった ICD コードの先頭を仮採用している
- `procedures` は K コード等の単純抽出で、網羅性は高くない
- `age` と `sex` は元資料に十分な記述がないため空のまま

つまり、現状の JSON は「そのまま正解データ」ではなく、`marume` 用に人手で仕上げる前段の素材です。

## テスト

```bash
uv run pytest tests/test_coding_text.py tests/test_sample_cases.py tests/test_transform.py
```

## 今後の改善候補

- `dpc_name` と `example_text` の境界補正を増やす
- `入院契機病名` / `医療資源病名` / `入院後発症疾患` を構造化して分離する
- `testdata/cases/` に直接展開できるように、候補 JSON から症例ファイル群を生成する
