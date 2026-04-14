# marume

![CodeRabbit Pull Request Reviews](https://img.shields.io/coderabbit/prs/github/ochanuco/marume?utm_source=oss&utm_medium=github&utm_campaign=ochanuco%2Fmarume&labelColor=171717&color=FF570A&link=https%3A%2F%2Fcoderabbit.ai&label=CodeRabbit+Reviews)

`marume` は、診断群分類（DPC）ルールをローカルで評価する Go 製 CLI です。JSON ルールセットまたは SQLite スナップショットを読み込み、症例ごとの分類結果を返せます。

## For Application Users

`marume` はアプリケーションとして、単票・一括の DPC 分類、採用候補ルールと一致理由の説明、症例入力の基本検証、ルールや入出力スキーマの参照、サンプル入力や最小ルールの生成、ルールセット情報の確認ができます。

## Setup

```bash
mise run dev:setup
mise run go:build
```

## Usage

詳細な CLI の使い方は `./marume --help`、利用できるローカル task の一覧は `mise task ls --local` で確認できます。

## For Developers

開発者向けには、Python ベースのデータ処理が含まれています。厚労省公開データから評価用スナップショットを作るための補助スクリプトと、DPC コーディング事例 PDF からサンプル症例候補を生成する処理を管理します。

## More Details

- SQLite 取り込み元とスナップショット方針: [docs/sqlite-data-sourcing.md](docs/sqlite-data-sourcing.md)
- DPC コーディング事例の抽出と症例候補生成: [docs/dpc-coding-sample-cases.md](docs/dpc-coding-sample-cases.md)
