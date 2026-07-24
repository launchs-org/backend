# Handoff: デプロイウィザード（フォルダドロップ → ビルド完了まで）

## Overview
学生向けのシンプルなデプロイUI。トップページでプロジェクトフォルダをドロップ（またはクリックしてフォルダ選択）→ 4ステップのウィザード（ビルドとポート／環境変数／データの保存／確認）→ ビルド進行画面、という一連の流れを実装する。

**スコープはビルドが完了するところまで**。実際にアプリが稼働URLで公開される「apply（反映）」以降は別スコープ。下記「重要な注意」に理由と対応方針を明記しているので必読。

## About the Design Files
このバンドル内のHTMLは**デザイン参照用のプロトタイプ**（モックデータ・モック遷移で動く）であり、そのままプロダクションコードとして使うものではない。実装対象のコードベースの環境（React/Vue/その他フレームワーク、既存の認証・API呼び出し基盤）に合わせて、同等のUIと振る舞いを再構築すること。既存フロントエンド（`frontend/` ディレクトリ、Vite + React + TypeScript、`frontend/src/lib/api.ts` の認証実装）があるとのことなので、その基盤の上に実装するのが前提。

## Fidelity
**Hi-fi**（配色・タイポグラフィ・スペーシング・コピーはほぼそのまま採用可）。ただし機能面はすべてモック（setTimeoutでビルドログを進める等）なので、実APIへの差し替えが本タスクの本体。

## 重要な注意（実装前に必ず確認）

1. **Dockerfileビルダーは現状バックエンド未対応**
   プロトタイプの「ビルダー」選択には Railpack と Dockerfile の2択があるが、`PRODUCT_OVERVIEW_FOR_FRONTEND.md` によると `dockerfile` タイプは「GitHubリポジトリ + Dockerfileパス」を要求し、かつ**現状ビルドAPIは400を返す**（未対応）。フォルダアップロード（`archive`タイプ）は常に Railpack でビルドされる。
   → 対応方針の提案：フォルダアップロードのフローでは「ビルダー」選択自体を一旦非表示にし、常に `archive` + Railpack 固定にする。Dockerfile対応が必要なら別途GitHub連携フローとして設計し直す。ここは実装前にプロダクト側と要確認。

2. **「ビルド成功」≠「アプリが稼働URLで公開される」**
   プロトタイプは分かりやすさ優先で、ビルド成功後すぐに「デプロイが完了しました！」と稼働URLを表示している。しかし実際のバックエンドでは：
   - ビルド成功 → `pending_image_id` にセットされるだけ（まだ何も反映されていない）
   - 実際にKubernetesへ反映してURLが有効になるには `POST /api/v1/deployments/{id}/apply` を別途呼び、非同期の反映が完了するのを待つ必要がある
   → 本タスクは「ビルド完了まで」なので、ビルド成功画面は「ビルドが完了しました。次に公開（適用）します」のような文言にし、実際の apply 呼び出し・反映待ちポーリングは**次フェーズのタスクとして別途スコープする**ことを推奨。今回はUIとしてボタンだけ用意し、クリック時はTODOにするか、後続フェーズに繋ぐ形でよい。

3. **アップロード形式（zip / tar.gz）**
   `docs/openapi.yaml` 上は `build/upload` は zip/tar.gz どちらも受け付ける記載だが、プロダクト側からの直近の指示は「バックエンドは tar.gz のみを受け取る」。フロントエンドは folder を **zip化ではなく tar.gz 圧縮**してから multipart/form-data で送る必要がある。ブラウザに tar.gz 圧縮の標準APIはないため、クライアントサイドでの圧縮ライブラリ導入が必要（例：`pako`（gzip）+ 自前や `tar-stream`/`fflate` 等でtar化してからgzip）。実装前にバックエンド側の受け入れ形式を最終確認すること。

## Screens / Views（プロトタイプ: `Deploy Dashboard.dc.html`）

### 1. ランディング（フォルダドロップ）
- 全画面。中央に見出し「プロジェクトフォルダをドロップ」、その下に点線ボーダーの大きなドロップ枠（クリックでOS標準のフォルダ選択ダイアログを開く。実装は `<input type="file" webkitdirectory>` ）。
- ドラッグオーバー時は枠の背景色・ボーダー色をアクセントカラーに変化。
- フォルダを選択/ドロップしたら、選択したフォルダの中身（`File.webkitRelativePath`）からフォルダ名とトップレベルのサブフォルダ一覧を読み取り、ウィザードへ渡す。
- 右上に「既存のプロジェクトを見る →」リンク（今回のスコープ外の既存ダッシュボードへの導線。実装済みダッシュボードUIがあれば接続、なければ非表示でよい）。

### 2. ウィザード（フルスクリーンオーバーレイ、4ステップ）
共通レイアウト：上部にステップインジケーター（丸数字+ラベル）、中央に幅920px程度のフォームエリア、下部に「戻る」「次へ」ボタン。カードは横最大1400px、上寄せ配置（ステップ間で移動してもカード上端の位置が変わらないこと＝重要な既存の修正ポイント）。

**Step 1: ビルドとポート**
- アプリの名前（テキスト入力）
- ビルダー選択（※上記「重要な注意 1」参照。対応方針が決まるまでは非表示 or Railpack固定に）
- ビルドディレクトリ：`<input list="...">` + `<datalist>` によるネイティブのサジェスト付き入力。サジェストは選択したフォルダの実際のサブフォルダ名（`./`, `./src` など）を動的に生成する。空欄扱いは `./`。
- 「アプリをインターネットに公開する」トグル。ONの時のみ以下2つを表示：
  - ポート番号（数値入力）
  - 公開URLの名前（任意のテキスト入力）。未入力の場合はアプリ名から自動生成したスラッグをプレースホルダー表示し、実際の送信値も同じ規則で生成する（`IngressRoute` の `name` に相当。バックエンドは `{name}-{uuid8}.launchs.org` の形でホスト名を自動生成する）。
**Step 2: 環境変数**
- キー/値の行を複数追加・削除できるフォーム（任意項目）

**Step 3: データの保存**
- 「データを保存する場所を使う」トグル
- ONの場合：容量選択（1/2/3/5GB、最大5GB）＋保存先フォルダ（テキスト入力、ラベル「保存先のフォルダ」）

**Step 4: 確認**
- それまでの入力内容のサマリー一覧
- 「デプロイする」ボタン→ビルド進行画面へ

### 3. ビルド進行画面（フルスクリーンオーバーレイ）
- フェーズ1「アプリをビルドしています…」：ダークな背景のログパネル（monospace, 高さ420px程度）にログ行が順に追加されていく
- フェーズ2「アプリを反映しています…」：チェックリスト形式で項目が順に完了マークに変わる（※上記「重要な注意 2」により、本タスクではここは実際には呼ばずモックのままで良いか要確認）
- フェーズ3 完了画面：チェックマーク＋メッセージ＋（本来はここでURL表示だが、注意2の通り実際にはビルド成功時点ではURLはまだ有効ではない）

## Interactions & Behavior
- フォルダのドラッグ&ドロップ、クリックでのネイティブフォルダ選択ダイアログ
- ステップ間の「次へ」「戻る」でのフォーム状態保持
- ビルドディレクトリの候補は選択済みフォルダの実際の構成から生成（静的リストではない）
- ビルド進行画面はポーリングベースを想定（後述API参照）

## State Management（実装時に必要な状態）
- `wizard.name`, `wizard.builder`（要検討、注意1参照）, `wizard.buildDirectory`, `wizard.buildDirOptions`（フォルダ由来）
- `wizard.portEnabled`, `wizard.port`, `wizard.ingressName`（空なら送信時にアプリ名からスラッグを生成）
- `wizard.envRows: {key, value}[]`
- `wizard.volumeEnabled, volumeSize, mountPath`
- `deploymentId`（作成後に取得）
- `buildId`, `buildStatus`（pending/building/succeeded/failed/cancelled）, `buildLogs[]`
- 選択したフォルダの `File[]`（tar.gz圧縮前の元データ）

## API連携シーケンス（`PRODUCT_OVERVIEW_FOR_FRONTEND.md` §5.2, §5.3 参照。詳細スキーマは `docs/openapi.yaml`）

前提：全リクエストに `Authorization: <JWTそのまま>` ヘッダー（`Bearer` prefixなし）。ベースURL `http://localhost:8080`。

1. **デプロイメント作成**
   `POST /api/v1/projects/{projectId}/deployments`
   body例: `{ "type": "archive", "name": "<wizard.name>" }`
   → レスポンスの `id` を `deploymentId` として保持。ステータスは `not_init`。

2. **フォルダをtar.gzに圧縮**（クライアント側、上記注意3参照）

3. **アーカイブアップロード**
   `POST /api/v1/deployments/{deploymentId}/build/upload`（multipart/form-data、圧縮済みファイル）
   → `upload_token` を取得（15分有効・1回のみ使用）

4. **ビルド開始**
   `POST /api/v1/deployments/{deploymentId}/build`
   body: `{ "archive_upload_token": "<token>", "build_directory": "<wizard.buildDirectory>" }`
   → `build.id` を取得、status `pending`

5. **ビルド進行ポーリング**
   `GET /api/v1/builds/{buildId}/logs?since=<cursor>` を1〜2秒間隔で呼び、差分ログをビルド進行画面に追記
   `GET /api/v1/builds/{buildId}` でstatusを確認し、`succeeded` / `failed` / `cancelled` になったらポーリング終了
   失敗時はエラーメッセージ（§8のエラー形式）をそのまま表示

6. **ポート・イングレス・環境変数・ボリュームの設定を保存**（ビルドとは非同期に、次フェーズのapply用に永続化しておく。ビルド自体には影響しない）
   - `wizard.portEnabled` がONの場合のみ： `POST /api/v1/deployments/{deploymentId}/service`（ポート設定）
   - イングレス名：`wizard.ingressName` が空なら `wizard.name` をスラッグ化（英数字・ハイフンのみ、小文字）した値を使う。`POST /api/v1/projects/{projectId}/ingress-routes` の `name` として送信（実際のホスト名は `{name}-{uuid8}.launchs.org` の形でバックエンドが自動生成するので、フロントエンドはユーザー入力欄に完全なURLではなく「名前」部分のみを持たせる）。PathRuleでこのServiceに紐付ける。
   - `POST /api/v1/projects/{projectId}/env-vars` → `POST /api/v1/deployments/{deploymentId}/env-var-mounts`（環境変数）
   - `POST /api/v1/projects/{projectId}/volumes` → `POST /api/v1/deployments/{deploymentId}/volume-mounts`（永続化、5GB上限はプロダクト側方針。バックエンドのクォータ上限とは別に確認すること）

7. **（次フェーズ・今回は未実装でよい）** `POST /api/v1/deployments/{deploymentId}/apply` を呼び、`GET /api/v1/deployments/{deploymentId}` をポーリングして `status`/`app_status` が `running` になったら稼働URL（`Deployment` or `IngressRoute` のホスト名）を表示。

### エラーハンドリング
`{ "error": "文字列" }` はそのままトースト等に表示。`{ "error": "quota_exceeded", "resource": "..." }` は専用メッセージに変換（§5.12参照、例：「アプリの上限数に達しました」）。

## Design Tokens
- フォント: Roboto（Google Fonts）, フォールバック system-ui
- 背景: `#f8f9fa`（キャンバス）/ `#ffffff`（カード・サイドバー）
- テキスト: `#202124`（primary）/ `#5f6368`（secondary）/ `#9aa0a6`（tertiary）
- アクセント: `#1a73e8`（primary blue）/ ホバー `#1557b0` / 薄色背景 `#e8f0fe`
- ステータス色: 成功 `#188038`(bg `#e6f4ea`) / 警告・準備中 `#f29900`(bg `#fef7e0`) / エラー `#d93025`(bg `#fce8e6`)
- ボーダー: `#dadce0`
- 角丸: カード`8px`〜`12px`、バッジ・ピル`12px`〜`20px`
- コード/ログ表示: `ui-monospace, monospace`、ログ背景 `#202124` / 文字 `#e8eaed`

## Assets
アイコン・画像アセットなし（すべてCSS図形とテキストラベルで構成）。

## Files
- `Deploy Dashboard.dc.html` — 本体のデザインプロトタイプ（このフォルダにコピー同梱）
- 参照ドキュメント: `uploads/PRODUCT_OVERVIEW_FOR_FRONTEND.md`, `docs/openapi.yaml`（プロジェクトルートのバックエンドリポジトリ側）
