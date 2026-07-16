# backend-mini — フロントエンド実装者向けプロダクト概要

このドキュメントは、既存バックエンド（実装済み）に対してフロントエンドを新規実装するために書かれている。デザイン・UIコンポーネント・見た目についての言及は意図的に含めない。「何ができるか」「どのAPIをどう呼べばいいか」「画面がどんな状態を表現する必要があるか」を、`docs/openapi.yaml` と合わせて読めば実装できるレベルまで具体化する。

---

## 1. このプロダクトは何か

**ユーザーが自分のアプリケーションをコンテナとしてクラウド上で動かせる、簡易版 PaaS（Platform as a Service）のバックエンド。** Heroku や Railway、Vercel のようなサービスの管理API部分に相当する。

ユーザーは以下のいずれかの方法でアプリケーションのソースを指定し、バックエンドがビルド・コンテナイメージ化・Kubernetesへのデプロイ・外部公開・ログ/メトリクス監視までを代行する：

1. 既存のDockerイメージを直接指定する
2. GitHubリポジトリを指定してRailpack（自動言語検出ビルドツール）でビルドする
3. ローカルのソースコードをzip/tar.gzでアップロードしてRailpackでビルドする（Dockerfileベースのビルドは現状未サポート）

フロントエンドが実装すべきは、この一連のリソース管理（プロジェクト作成→デプロイメント作成→ビルド→公開設定→apply→稼働監視）をユーザーが操作できるダッシュボードUIである。

---

## 2. システム構成（フロントエンドが直接話す相手）

フロントエンドが呼ぶAPIは **`handler` サービスの一つだけ**。ベースURLは `http://localhost:8080`（本番ではリバースプロキシ配下）。

バックエンド内部は4つのGoサービスに分かれているが、フロントエンドから見ると関係ない：

| サービス | 役割 | フロントエンドとの関係 |
|---|---|---|
| `handler` | HTTP API本体。DB読み書き・Temporal Workflow起動 | **これだけを呼ぶ** |
| `controller` | Temporal Worker。k8s操作の実処理 | 非同期で裏側で動く。結果はDeploymentの`status`/`app_status`/`k8s_status`に反映される |
| `builder` | Temporal Worker。ビルド処理・Harbor(コンテナレジストリ)へのpush | 同上。結果は`DeploymentBuild.status`に反映される |
| `watcher` | k8sリソースの状態をポーリングしDBに同期 | 同上。ステータス系フィールドが裏で更新される |

**重要な設計原則**: 多くの更新系APIは即座に処理を完了させない。「DBに変更を保存してWorkflowを起動した」時点でレスポンスを返し、実際のKubernetes反映は非同期に進行する。フロントエンドはポーリング（またはページ再訪時の再取得）でステータスの変化を追う必要がある。WebSocketやSSEのようなリアルタイム通知APIは現状存在しない。

---

## 3. 認証

- 認証はJWTトークンで行う。**`Authorization` ヘッダーにJWT文字列をそのまま設定する**（`Bearer <token>` 形式ではない）。
- トークンの発行元は本リポジトリの外にある別サービス（外部認証基盤）。フロントエンドはログイン画面/フローを別途外部認証サービスと連携して実装し、得られたJWTを全APIリクエストのヘッダーに付与する。
- JWTのクレームには `userID`（所有権チェックに使う）、`labels`（`"admin"` を含むと管理者専用APIが使える）、`provCode`、`provUid` が入っている。フロントエンド側でJWTをデコードしてUIの出し分け（管理者機能の表示/非表示など）に使ってよい。
- 認証失敗・ヘッダー欠如は全エンドポイントで `401` を返す。
- 例外1: `POST /webhooks/{deployment_id}/*` の4エンドポイントはJWT不要。代わりに `X-Webhook-Secret` ヘッダーでデプロイメント固有のシークレットを検証する（CI/CD連携用途、後述）。
- 例外2: 管理者専用（Deployment Templateの作成/更新/削除）は `labels` に `"admin"` がないユーザーには `403` を返す。

---

## 4. リソースの階層とマルチテナンシー

```
User（外部認証基盤が発行するID。バックエンドにUserテーブルはない）
 └─ Project（1ユーザーがN個持てる。k8s Namespace 1つ + Harborプロジェクト1つに対応）
     ├─ UserQuota / ResolvedQuota（プロジェクト数・デプロイメント数などの上限）
     ├─ Deployment（1プロジェクトがN個持てる。アプリケーション1つに対応）
     │   ├─ Service（1デプロイメントに1つ。外部公開ポート設定）
     │   ├─ EnvVarMount（EnvVarをこのデプロイメントに紐付ける中間テーブル）
     │   ├─ VolumeMount（Volumeをこのデプロイメントに紐付ける中間テーブル）
     │   ├─ DeploymentBuild（ビルド履行履歴。1デプロイメントに複数）
     │   ├─ DeploymentWebhook（CI/CD連携用シークレット）
     │   └─ ApplyHistory（apply実行履歴のスナップショット）
     ├─ EnvVar（プロジェクト共有の環境変数プール。複数デプロイメントにマウントできる）
     ├─ Volume（プロジェクト共有のPVC。複数デプロイメントにマウントできる）
     ├─ IngressRoute（1プロジェクトがN個。外部公開用ドメインルーティング）
     │   └─ PathRule（IngressRoute配下のパスベースルーティング）
     └─ Image（ビルド成果物 or 手動登録した外部イメージの参照）
```

所有権チェックは全APIで「リクエストのJWTの`userID`」と「対象リソースが属するProjectの`user_id`」の一致を見ている。他ユーザーのリソースを触ると `403`。フロントエンドは常に「今ログインしているユーザーが所有するProject一覧」からナビゲーションを始める設計にする。

---

## 5. コア機能一覧（画面単位で考える材料）

### 5.1 プロジェクト管理
- `GET/POST /api/v1/projects`, `GET/PUT/DELETE /api/v1/projects/{id}`
- プロジェクトは作成時にk8s Namespaceの作成が非同期で走る（`status`: `provisioning` → `active`、削除時は `deleting`）。
- `GET /api/v1/projects/{id}/quota` でこのプロジェクトが使えるクォータ残量を取得できる。

### 5.2 デプロイメント管理（アプリケーションの本体）
- `GET/POST /api/v1/projects/{id}/deployments`, `GET/PUT/DELETE /api/v1/deployments/{id}`
- `type` は作成後変更不可。4種類:
  - `image_url` — 既存Dockerイメージを直接指定
  - `dockerfile` — GitHubリポジトリ + Dockerfileパス（**現状ビルド非対応、APIは400を返す**。作成はできるがビルドできない）
  - `railpack` — GitHubリポジトリ。Railpackが言語を自動検出してビルド
  - `archive` — zip/tar.gzアップロードしてRailpackでビルド
- 更新は `pending_*` フィールド経由（後述セクション6）。
- `POST /api/v1/deployments/{id}/from-template` 相当として `POST /api/v1/projects/{id}/deployments/from-template` があり、事前定義された `DeploymentTemplate` から一括作成できる（ワンクリックデプロイ用途）。
- `command`/`args` でコンテナのENTRYPOINT/CMDを上書きできる（配列）。
- `instance_size`（例: `small`）と `replicas`（レプリカ数）で規模を指定。

### 5.3 ビルド（railpack / archive タイプ）
- **GitHubビルド**: `POST /api/v1/deployments/{id}/build` を呼ぶだけでよい（`github_repo_url`等はDeployment更新時に事前設定済み）。
- **アーカイブアップロードビルド**: 2段階。
  1. `POST /api/v1/deployments/{id}/build/upload` に zip/tar.gz を multipart/form-data で送信 → `upload_token` を取得（有効期限15分、1回のみ使用可）
  2. `POST /api/v1/deployments/{id}/build` に `archive_upload_token` と `build_directory` を渡してビルド開始
  3. 再ビルドする場合は毎回アップロードからやり直す（サーバー側でアーカイブを保持しない）
- ビルド進行状況は `GET /api/v1/builds/{id}` または `GET /api/v1/builds/{id}/logs`（`since`パラメータで差分ポーリング）で追う。
- ビルド履歴一覧: `GET /api/v1/deployments/{id}/builds`（デプロイメント単位）、`GET /api/v1/projects/{id}/builds`（プロジェクト単位）。
- ビルドのキャンセル: `DELETE /api/v1/builds/{id}`。
- ビルド状態(`status`): `pending` → `building` → `succeeded` / `failed` / `cancelled`。
- ビルド成功後、成果物イメージが自動的に`pending_image_id`へセットされる。**ビルドが成功しても即座には反映されない。次のapplyで実際にデプロイされる。**

### 5.4 反映（apply）— このプロダクトの中心的な操作
- `POST /api/v1/deployments/{id}/apply`：`pending_*` フィールドの内容を本フィールドへ昇格し、Kubernetesへ実際に反映する。非同期（Temporal Workflow起動のみで即レスポンス）。
- `POST /api/v1/deployments/{id}/discard-pending`：pendingの変更を破棄（applyせずに取り消す）。
- `POST /api/v1/projects/{id}/apply`：プロジェクト内の複数リソースを一括apply（IngressRoute系の変更を含む場合に使う）。
- `GET /api/v1/deployments/{id}/apply-histories`：過去のapply実行履歴（誰が・いつ・何を変更したかのスナップショット）。
- Deploymentが`not_init`ステータスのままapplyしようとすると`400`が返る（railpack/archiveタイプで初回ビルドが完了していない場合）。

### 5.5 外部公開設定
- **Service**（ポート公開）: `GET/POST/PUT/DELETE /api/v1/deployments/{id}/service`。`type`は`ClusterIP`/`NodePort`/`LoadBalancer`。
- **IngressRoute + PathRule**（ドメイン・パスルーティング）: `GET/POST /api/v1/projects/{id}/ingress-routes`、`DELETE /api/v1/ingress-routes/{id}`、`PATCH /api/v1/ingress-routes/{id}/name`、`GET/POST /api/v1/ingress-routes/{id}/path-rules`。ホスト名は`{name}-{uuid8}.launchs.org`形式で自動生成される。PathRuleで特定パスを特定Serviceへ転送し、`strip_prefix`でプレフィックス除去も設定できる。

### 5.6 環境変数
- プロジェクト共有プール: `GET/POST /api/v1/projects/{id}/env-vars`、`PUT/DELETE /api/v1/env-vars/{id}`。
- 個別デプロイメントへのマウント: `GET/POST /api/v1/deployments/{id}/env-var-mounts`、`DELETE /api/v1/env-var-mounts/{id}`。
- キー重複時はランダムサフィックスで自動リネームされる仕様（エラーにはならない）。

### 5.7 ボリューム（永続ストレージ）
- プロジェクト共有: `GET/POST /api/v1/projects/{id}/volumes`、`DELETE /api/v1/volumes/{id}`（PVCに対応）。
- デプロイメントへのマウント: `GET/POST /api/v1/deployments/{id}/volume-mounts`、`DELETE /api/v1/volume-mounts/{id}`。

### 5.8 コンテナイメージ管理
- `GET /api/v1/projects/{id}/images`、`GET /api/v1/images/{imageId}`、`DELETE /api/v1/projects/{id}/images/{imageId}`。
- ビルド成果物（`build_id`が紐づく）と手動登録イメージ（`image_url`タイプのデプロイメントが参照）の両方がここに一覧される。サイズ情報（`size_bytes`）も持つ。

### 5.9 ログ・メトリクス（運用監視）
- Podログ: `GET /api/v1/deployments/{id}/logs`
- CPU/メモリメトリクス: `GET /api/v1/deployments/{id}/metrics`
- ビルドログ: `GET /api/v1/builds/{id}/logs`（差分ポーリング対応、`since`パラメータ）

### 5.10 Webhook（CI/CD連携）
- 管理: `POST/GET /api/v1/deployments/{id}/webhooks`、`DELETE /api/v1/webhooks/{id}`。作成時にシークレット文字列が発行される。
- 外部トリガー（JWT不要、`X-Webhook-Secret`ヘッダーで認証）:
  - `POST /webhooks/{deployment_id}/build` — ビルドをトリガー
  - `GET /webhooks/{deployment_id}/builds/{build_id}` — ビルド状態確認
  - `POST /webhooks/{deployment_id}/apply` — applyを実行
  - `POST /webhooks/{deployment_id}/update-image` — `image_url`タイプ専用。イメージURLを更新して即apply
- 想定用途: GitHub ActionsなどのCIパイプラインの最終ステップからこれらを呼び、push→ビルド→デプロイを自動化する。フロントエンドはこの設定画面（シークレット表示・呼び出しコマンド例の提示）を用意するとよい。

### 5.11 デプロイメントテンプレート（ワンクリックデプロイ）
- `GET/POST/PUT/DELETE /api/v1/deployment-templates`、`GET /api/v1/deployment-templates/{id}`（作成/更新/削除は管理者のみ）。
- あらかじめ用意された構成（例: WordPress、Redis等）から `POST /api/v1/projects/{id}/deployments/from-template` でワンクリックにデプロイメントを作成できる。マーケットプレイス的なUIに使える。

### 5.12 クォータ・プラン
- `GET/PUT /api/v1/users/quota`（自分のクォータ確認・上限申請的な更新）
- `GET /api/v1/projects/{id}/quota`（プロジェクト単位の残量）
- 管理対象: 最大プロジェクト数、最大デプロイメント数、最大レプリカ数、最大ボリューム数、ボリューム総容量。
- 超過時は専用のエラーレスポンス形式（`error: "quota_exceeded"`, `resource: "..."`）が返るため、フロントエンドはこれを検出して分かりやすいメッセージに変換する必要がある。

### 5.13 CLIトークン
- `POST/GET /api/v1/cli-tokens`、`DELETE /api/v1/cli-tokens/{id}`。
- ユーザーが自分用のAPIトークンを発行・失効できる機能（CLIツールや自動化スクリプトからAPIを呼ぶための長期トークン）。設定画面の一項目として実装する想定。

---

## 6. `pending_*` フィールドパターン（フロントエンドが必ず理解すべき設計）

Deployment・Service・EnvVarMount・VolumeMountなど多くのリソースは以下の二重フィールド構造を持つ：

- 本フィールド（例: `image_id`, `github_repo_url`, `replicas`）— **現在実際にKubernetesへ反映されている値**
- `pending_*` フィールド（例: `pending_image_id`, `pending_github_repo_url`, `pending_replicas`）— **まだ反映されていない変更予定値**

`PUT` 系APIは本フィールドを直接書き換えず、`pending_*` に変更を積む。この状態でUIは「未反映の変更があります」というインジケータを出すべきで、それには本フィールドと`pending_*`フィールドを比較すればよい（値が異なる＝差分がある）。

`POST /api/v1/deployments/{id}/apply` を呼ぶと、Temporal Workflowが非同期に起動し、`pending_*` の値が本フィールドへ「昇格」してKubernetesに反映される。`POST /api/v1/deployments/{id}/discard-pending` で変更を捨てることもできる。

**UIパターンの推奨**: 設定変更フォーム → 保存（`pending_*`が更新される）→ 差分プレビュー表示 → 「適用」ボタン（`apply`を呼ぶ）→ 反映状況をポーリングして表示、という2段階フローをどの設定画面でも一貫させる。

---

## 7. 状態遷移（UIのステータス表示に必須）

### Deployment.status
| 値 | 意味 |
|---|---|
| `not_init` | railpack/archiveタイプで初回ビルド未完了（この状態でapplyすると400） |
| `pending` | 作成済み・未apply、またはpending差分がある状態 |
| `running` | 正常稼働中 |
| `failed` | 直前のWorkflowが失敗 |
| `deleting` | 削除処理中 |

### Deployment.app_status（アプリケーションの実際の動作状態、より粒度が細かい）
| 値 | 意味 |
|---|---|
| `pending` | 未apply |
| `building` | ビルド中 |
| `deploying` | k8sへデプロイ中 |
| `running` | 正常稼働中 |
| `error` | エラー状態 |

`status` と `app_status` は両方表示し、`k8s_status`（jsonb、Pod詳細等の生データ）は詳細画面でのみ展開表示する想定でよい。`delete_progress` フィールドは削除処理中のステップ名が入るので、削除中はこれを進捗表示に使える。

### DeploymentBuild.status
`pending` → `building` → `succeeded` / `failed` / `cancelled`

### Project.status
`provisioning` → `active` → （削除時）`deleting`

---

## 8. エラーレスポンスの共通形式

```json
{ "error": "リソースが見つかりません" }
```

日本語のメッセージがそのまま返る想定（`error`キー1つのフラットなオブジェクト）。クォータ超過だけ形式が異なる：

```json
{ "error": "quota_exceeded", "resource": "deployments" }
```

フロントエンドはこの2形式を判定し、`error`が`"quota_exceeded"`の場合は`resource`を見てクォータ超過の専用UIを出す、それ以外は`error`文字列をそのままトースト等で表示する、という分岐にするのが最も単純。

HTTPステータスコードは概ねREST的（`400`不正リクエスト/バリデーション、`401`未認証、`403`権限なし、`404`不存在、`409`競合（例: ビルド中に再ビルド不可）、`413`ペイロード過大、`500`サーバーエラー、`502`外部サービス連携失敗）。

---

## 9. 実装の出発点として推奨する画面構成

デザインには踏み込まないが、機能の依存関係上、以下の順で実装すると手戻りが少ない：

1. **認証**: 外部認証サービスとの連携、JWTの保存・付与
2. **プロジェクト一覧・作成・詳細**
3. **デプロイメント一覧・作成（タイプ選択）・詳細**（`image_url`から始めると最も単純。次に`railpack`）
4. **apply操作とpending差分表示**（システムの中核パターンなので早期に固めるべき）
5. **ビルド機能**（`railpack`→`archive`の順。archiveは2段階アップロードのUIが必要）
6. **公開設定**（Service → IngressRoute/PathRule）
7. **環境変数・ボリューム管理**
8. **ログ・メトリクス表示**
9. **Webhook設定・CLIトークン管理**（後回しでよい設定系機能）
10. **管理者機能・デプロイメントテンプレート**（最後でよい）

---

## 10. 参照

- 全エンドポイントの詳細なリクエスト/レスポンススキーマ: `docs/openapi.yaml`（本ドキュメントと併せて読むこと。フィールドの型・必須/任意・enum値はすべてそこに定義されている）
- OpenAPIの `info.description` にも認証・pendingパターン・典型フローの説明があるので二重に参照可能
