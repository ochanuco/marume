# SQLite Data Sourcing

`marume` の SQLite スナップショットを作るときの、基礎データの収集元と最小スキーマ案をまとめる。

## 結論

- ルール本体の正本は厚生労働省の DPC 電子点数表を起点にする
- 病名の補助マスタは ICD-10 / 社会保険表章用疾病分類 / 標準病名マスター系を使う
- 手術・処置コードの補助マスタは診療報酬改定で公開される公的マスタ群を使う
- CLI 配布物の SQLite は、上記の公的データから BigQuery / Dataform 側で正規化・展開したスナップショットにする

つまり、「SQLite に入れるデータは厚労省から集める」で概ね正しい。ただし実務上は 1 ソースでは足りず、DPC 本体と病名・処置の補助マスタを分けて管理する。

## 収集元

### 1. DPC ルール本体

最優先の正本。DPC コード、診断群分類定義、版管理の起点に使う。

- 厚生労働省「令和8年度診療報酬改定について」
  - 2026-04-05 時点で「診断群分類（DPC）電子点数表（正式版）」は 2026-03-18 更新
  - https://www.mhlw.go.jp/stf/newpage_67729.html
- 厚生労働省「診断群分類（DPC）電子点数表について」
  - 年度ごとの過去版を遡る用途
  - https://www.mhlw.go.jp/stf/seisakunitsuite/bunya/0000198757.html

### 2. 傷病名・ICD 系

主傷病名や併存症のコード正規化、説明表示、将来の入力検証に使う。

- 厚生労働省「疾病、傷害及び死因の統計分類」
  - ICD-10 準拠の基本分類表・内容例示表
  - https://www.mhlw.go.jp/toukei/sippei/index.html
- 厚生労働省「社会保険表章用疾病分類」
  - ICD 準拠の社会保険向け分類
  - 索引表は標準病名マスター由来で、病名から分類コードを引く補助資料
  - https://www.mhlw.go.jp/stf/seisakunitsuite/bunya/kenkou_iryou/iryouhoken/database/hokensippei.html

補足:

- 厚労省ページ上でも、最新の病名・ICD コードについては MEDIS の標準病名マスター最新版参照とされている
- そのため、病名名称の補助表示や病名文字列からの解決までやるなら、将来的に標準病名マスターも別取得対象に含める

### 3. 手術・処置コード系

POC では `procedures` を配列で受けているため、K コードや関連コード体系の名称・有効期間・分類を持つ補助マスタが必要になる。

- 起点は診療報酬改定時の公的マスタ群
- DPC 電子点数表だけで処置コード名称や細かな補助属性が足りない場合は、診療報酬点数表系のマスタで補完する

注意:

- `marume` の分類ロジックに必要なのは「請求用マスタ全部」ではなく、「ルール評価に出てくるコードを解釈できる最小集合」
- まずは DPC ルール内に出てくるコードだけを抽出してサブセット化する方が現実的

### 4. 退院患者調査資料

- 厚生労働省「DPCの評価・検証等に係る調査（退院患者調査）」
  - https://www.mhlw.go.jp/stf/newpage_67729.html

これは運用説明や検証観点では有用だが、SQLite の分類ルール正本そのものではない。初手の収集対象としては優先度を下げる。

## `marume` で最初に持つべき最小テーブル

初版は、分類ロジックに必要な最小構成に絞る。

### `rule_sets`

- `rule_set_id`
- `fiscal_year`
- `rule_version`
- `source_url`
- `source_published_at`
- `build_id`
- `built_at`

役割:

- どの年度・どの版の DPC 定義から作った SQLite かを追跡する

### `rules`

- `rule_id`
- `rule_set_id`
- `priority`
- `dpc_code`
- `mdc_code`
- `label`

役割:

- 評価順付きの採用候補ルール本体

### `rule_conditions`

- `condition_id`
- `rule_id`
- `condition_type`
- `operator`
- `value_text`
- `value_num`
- `value_json`
- `negated`

`condition_type` 例:

- `main_diagnosis`
- `diagnosis`
- `procedure`
- `comorbidity`
- `age`
- `sex`

役割:

- CLI の評価器が読む完全展開済み条件

### `icd_master`

- `icd_code`
- `name_ja`
- `classification_code`
- `source_system`
- `valid_from`
- `valid_to`

役割:

- ICD コードの名称表示
- 入力検証
- `explain` の理由文補助

### `procedure_master`

- `procedure_code`
- `name_ja`
- `code_system`
- `valid_from`
- `valid_to`

役割:

- 処置コードの名称表示
- 入力検証
- `explain` の理由文補助

### `metadata`

- `key`
- `value`

役割:

- SQLite ファイル全体のビルド情報や注意事項を保持

## BigQuery / Dataform でやるべきこと

SQLite に直接生データを詰めるのではなく、先に BigQuery 側で正規化する。

1. 厚労省公開物を raw として保持する
2. 年度・更新日・版をキーに正規化する
3. DPC 定義を CLI が評価しやすい `rules` / `rule_conditions` 形に展開する
4. ICD / 処置マスタは名称表示に必要な列だけに絞る
5. SQLite にエクスポートする

これで [README.md](/Users/chanu/ghq/github.com/ochanuco/marume/README.md#L85) の「Dataform / BigQuery 由来のスナップショット生成に接続する」と整合する。

## POC の進め方

最初から全量マスタを集めない方がよい。以下の順で十分。

1. 2026 年度の DPC 電子点数表を取得する
2. POC ルールで必要な DPC コードと条件を抽出する
3. その条件で参照する ICD コードと処置コードだけを補助マスタ化する
4. SQLite スキーマにロードする
5. `internal/store` に SQLite 実装を追加する

## 収集優先度

優先度 A:

- DPC 電子点数表

優先度 B:

- ICD-10 基本分類表
- 社会保険表章用疾病分類

優先度 C:

- 標準病名マスター
- 処置コード補助マスタ
- 退院患者調査資料

## 今の判断

- 「厚労省から集める」は正しい
- ただし実装上は「厚労省の DPC 電子点数表を主正本にし、病名と処置は補助マスタで補う」と言い換えた方が正確
- POC の次アクションは、2026 年度 DPC 電子点数表から `rules` / `rule_conditions` に落とす変換仕様を先に決めること
