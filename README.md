# marume

`marume` は、診断群分類（DPC）ルールをローカルで評価する Go 製 CLI です。現在は POC 段階で、JSON ルールセットまたは SQLite スナップショットを読み込み、単票分類・一括分類・説明表示・入力検証を行えます。

## Current Capabilities

- 単票分類: `marume classify`
- 一括分類: `marume classify-batch`
- 候補ルールと一致理由の確認: `marume explain`
- CLI の機能一覧を JSON で表示: `marume capabilities`
- JSON Schema の表示: `marume schema`
- 入力の最低限検証: `marume validate`
- サンプル入力・最小ルールの生成: `marume testdata write`
- CLI とルールセット情報の表示: `marume version`
- 厚労省公開データからのスナップショット生成補助スクリプト
- DPC コーディング事例 PDF からのサンプル症例候補生成

## Setup

```bash
mise run dev:setup
mise run go:build
```

Python ベースのデータ準備スクリプトを使う場合:

```bash
uv sync
```

## Available Tasks

Go / CLI:

- `mise run go:build`
- `mise run go:test`
- `mise run go:sample`
- `mise run dev:setup`

Python / データ基盤:

- `mise run py:data:fetch-mhlw`
- `mise run py:data:extract-dpc-pdf`
- `mise run py:data:transform-dpc`
- `mise run py:data:build-sqlite`
- `mise run py:data:workflow`

Python / サンプルデータ生成:

- `mise run py:samples:coding-cases-extract`
- `mise run py:samples:coding-cases-build`
- `mise run py:samples:coding-cases-generate`
- `mise run py:samples:coding-cases`

## CLI Usage

サンプル一式を生成:

```bash
mise run go:sample
```

生成されたサンプルを使って CLI を試す:

```bash
./marume classify --rules .local/marume-sample/rules-minimal.json --input .local/marume-sample/case-ok.json
./marume classify-batch --rules .local/marume-sample/rules-minimal.json --input .local/marume-sample/cases-basic.jsonl --output result.jsonl
./marume explain --rules .local/marume-sample/rules-minimal.json --input .local/marume-sample/case-ok.json
./marume capabilities
./marume schema case-input
./marume validate --input .local/marume-sample/case-ok.json
./marume version --rules .local/marume-sample/rules-minimal.json
```

`--rules` には JSON と SQLite の両形式を渡せます。

AI エージェントなどの機械呼び出しでは、事前に `capabilities` で機能一覧を取得できます。

```bash
./marume capabilities
```

失敗時も JSON で受けたい場合は、グローバル `--json-errors` を使います。

```bash
./marume --json-errors classify --input bad.json
```

## Data Workflow

既定の workflow 設定は [workflows/dpc_2026_mhlw.json](workflows/dpc_2026_mhlw.json) です。

```bash
mise run py:data:workflow
```

手順を分けて実行する場合:

```bash
mise run py:data:fetch-mhlw
mise run py:data:extract-dpc-pdf
mise run py:data:transform-dpc
mise run py:data:build-sqlite
```

## More Details

- SQLite 取り込み元とスナップショット方針: [docs/sqlite-data-sourcing.md](docs/sqlite-data-sourcing.md)
- DPC コーディング事例の抽出と症例候補生成: [docs/dpc-coding-sample-cases.md](docs/dpc-coding-sample-cases.md)