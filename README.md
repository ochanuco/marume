# marume

DPC 分類の Go 製 CLI POC です。データ基盤より先に、ローカルで症例 JSON を評価できる最小構成を置いています。

現時点では SQLite ではなく JSON ルールセットを読んでいます。`RuleStore` 抽象を挟んでいるので、後から SQLite 実装に差し替える前提です。

## POC の範囲

- `classify`: 単票分類
- `explain`: 候補ルールと一致理由の確認
- `validate`: 入力の最低限検証
- `version`: CLI とルールセット情報の表示

## セットアップ

```bash
mise install
go build ./cmd/marume
```

## 使い方

```bash
./marume classify --input testdata/cases/case-ok.json
./marume explain --input testdata/cases/case-age-only.json
./marume validate --input testdata/cases/case-ok.json
./marume version
```

`stdin` から読む場合は `--input -` を使います。

## 現在の構造

- `cmd/marume`: エントリポイント
- `internal/cli`: コマンド処理
- `internal/evaluator`: ルール評価器
- `internal/store`: ルールセット取得層
- `testdata/rules`: POC 用ルールセット

## 次の差し替えポイント

1. `internal/store` に SQLite 実装を追加
2. `classify-batch` を追加
3. `validate` を JSON Schema か独自ルールで強化
4. Cobra ベースのコマンド体系に移行
