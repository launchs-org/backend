# launchs-org 完全実装設計書

> v5.12 準拠 | railpack 統合 | K8s 連携・ビルドキュー・SSE ログ詳細仕様

---

## 目次

1. [共通仕様](#1-共通仕様)
2. [環境変数一覧](#2-環境変数一覧)
3. [Projects API](#3-projects-api)
4. [Containers API](#4-containers-api)
5. [Build Jobs API](#5-build-jobs-api)
6. [Networking API](#6-networking-api)
7. [Volume API](#14-volume-api)
8. [History / Rollback API](#7-history--rollback-api)
9. [SSE ストリーミング仕様](#8-sse-ストリーミング仕様)
10. [ビルドキューシステム詳細](#9-ビルドキューシステム詳細)
11. [railpack 統合仕様](#10-railpack-統合仕様)
12. [K8s 状態同期仕様](#11-k8s-状態同期仕様)
13. [tar 受取・レジストリプッシュ仕様](#12-tar-受取レジストリプッシュ仕様)
14. [ログローテートバッチ](#13-ログローテートバッチ)
15. [永続化ボリューム（Volume）詳細仕様](#15-永続化ボリュームvolume詳細仕様)

---

## 1. 共通仕様

### リクエストヘッダー（全エンドポイント共通）

```
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

### レスポンスラッパー

```jsonc
// 単一リソース
{ "data": { ... } }

// 一覧
{ "data": { "items": [...], "total": 3 } }

// エラー
{ "code": "NOT_FOUND", "message": "container not found" }
```

### フィールド命名規則

全フィールドは **snake_case** 統一（例: `created_at`, `project_id`, `build_log`）

### HTTP ステータスコード方針

| 操作 | Status |
|------|--------|
| POST（リソース作成） | `201 Created` |
| GET / PATCH / POST（アクション） | `200 OK` |
| DELETE | `200 OK` + `{ "data": { "id": "..." } }` |

### エラーコード一覧

| HTTP Status | code | 説明 |
|-------------|------|------|
| 400 | `BAD_REQUEST` | リクエストパラメータ不正 |
| 401 | `UNAUTHORIZED` | JWT 認証失敗 |
| 403 | `FORBIDDEN` | 他ユーザーリソースへのアクセス |
| 404 | `NOT_FOUND` | リソースが存在しない |
| 409 | `CONFLICT` | 名前の重複など |
| 500 | `INTERNAL_ERROR` | サーバー内部エラー |

---

## 2. 環境変数一覧

サーバー起動時に読み込む環境変数の完全一覧。

```bash
# ── JWT ────────────────────────────────────────────────────
JWT_SECRET=your-secret-key

# ── データベース ─────────────────────────────────────────────
DATABASE_URL=postgres://user:pass@localhost:5432/launchs

# ── Kubernetes ───────────────────────────────────────────────
# buildkit Job を発行する Namespace
BUILD_NAMESPACE=buildkit
# ワーカー並列数（デフォルト: 5）
BUILD_WORKER_CONCURRENCY=5
# DB ポーリング間隔（秒）（デフォルト: 5）
BUILD_WORKER_POLL_INTERVAL_SECONDS=5
# ビルドタイムアウト（デフォルト: 10m）
BUILD_TIMEOUT=10m

# ── tar 受取サーバー ──────────────────────────────────────────
# K8s Job Pod がこのサーバーに tar を送信してくる URL
# K8s Service 経由の内部 DNS を使う
# 例: http://launchs-backend.default.svc.cluster.local:8080/internal/upload
UPLOAD_ENDPOINT=https://10.10.11.8:8090/app/internal/upload
# tar 受取時の Bearer 認証トークン（Job Pod → このサーバー間の認証）
UPLOAD_TOKEN=internal-secret-token
# tar 一時保存ディレクトリ（プッシュ後に削除）
TAR_SAVE_DIR=/tmp/launchs-tar

# ── プライベートレジストリ ────────────────────────────────────
REGISTRY_HOST=172.33.0.1
REGISTRY_PROJECT=launchs
REGISTRY_USERNAME=robot$launchs+builder
REGISTRY_PASSWORD=your-registry-password
# TLS 検証をスキップするか（開発環境向け）
REGISTRY_INSECURE=true

# ── ログローテート ────────────────────────────────────────────
LOG_ROTATE_INTERVAL_HOURS=24
BUILD_LOG_RETENTION_DAYS=30
CONTAINER_LOG_RETENTION_DAYS=7
```

---

## 3. Projects API

### GET /v1/projects

プロジェクト一覧取得（認証ユーザーが owner のもの全件）

**Request**

```
Headers: Authorization: Bearer <token>
Body: なし
```

**Response 200**

```json
{
  "data": {
    "items": [
      {
        "id": "proj_abc123",
        "name": "my-app",
        "k8s_resource_name": "my-app",
        "namespace": "ns-my-app",
        "owner_id": "user_xyz",
        "created_at": "2025-01-01T00:00:00Z"
      }
    ],
    "total": 1
  }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Project を owner_id でフィルタして全件取得（DeletedAt IS NULL）
3. レスポンスを返す
```

---

### POST /v1/projects

プロジェクト作成（K8s Namespace 初期化を含む）

**Request**

```json
{
  "name": "my-app"
}
```

| フィールド | 必須 | 説明 |
|-----------|------|------|
| name | ✅ | プロジェクト名。英小文字・数字・ハイフンのみ。K8s リソース名に直接使用される |

**Response 201**

```json
{
  "data": {
    "id": "proj_abc123",
    "name": "my-app",
    "k8s_resource_name": "my-app",
    "namespace": "ns-my-app",
    "owner_id": "user_xyz",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**エラーケース**

| Status | code | 条件 |
|--------|------|------|
| 400 | `BAD_REQUEST` | name が空 or 不正文字を含む |
| 409 | `CONFLICT` | name が既存と重複 |

**フロー**

```
1. JWT から owner_id を取得
2. name のバリデーション（空・不正文字）→ 不正: 400
3. name の重複チェック → 重複: 409
4. DB に Project レコードを作成
   - id       : UUID 生成
   - namespace: "ns-{name}"
5. client-go で K8s Namespace を作成
   - 名前: "ns-{name}"
   - Labels: { "managed-by": "launchs", "project-id": id }
6. 作成した Project をレスポンスとして返す
```

---

### GET /v1/projects/:id

プロジェクト詳細取得（Container の id/name 一覧をネスト）

**Response 200**

```json
{
  "data": {
    "id": "proj_abc123",
    "name": "my-app",
    "k8s_resource_name": "my-app",
    "namespace": "ns-my-app",
    "owner_id": "user_xyz",
    "created_at": "2025-01-01T00:00:00Z",
    "containers": [
      { "id": "cont_xyz789", "name": "frontend" },
      { "id": "cont_def456", "name": "backend" }
    ]
  }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Project を id で取得（DeletedAt IS NULL）→ なし: 404
3. owner_id 一致チェック → 不一致: 403
4. 紐づく Container の id, name 一覧を取得
5. レスポンスを返す
```

---

### DELETE /v1/projects/:id

プロジェクト削除（K8s Namespace 以下の全リソースを非同期削除）

**Response 200**

```json
{
  "data": { "id": "proj_abc123" }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Project を id で取得 → なし: 404
3. owner_id 一致チェック → 不一致: 403
4. 当該プロジェクトに紐づく BuildJob（Queued/Running）を全て Cancelled に更新
5. DB の Project レコードに DeletedAt をセット（ソフトデリート）
6. client-go で K8s Namespace 削除を非同期キック
   - Namespace 削除により Deployment/Service/Ingress/Job が K8s 側で自動削除される
7. 即座に 200 を返す

【クライアントの削除完了確認方法】
  GET /v1/projects/:id が 404 を返すまでポーリング
  ※ K8s Namespace の削除は非同期のため数秒～数十秒かかる場合がある
```

---

## 4. Containers API

### POST /v1/projects/:id/containers

コンテナ作成 + ビルド自動開始

**Request**

```json
{
  "name": "frontend",
  "repository_url": "https://github.com/org/repo",
  "branch": "main",
  "directory": "/",
  "env_vars": {
    "NODE_ENV": "production",
    "PORT": "3000"
  },
  "replicas": 1,
  "resources": {
    "requests": { "cpu": "100m", "memory": "128Mi" },
    "limits":   { "cpu": "500m", "memory": "512Mi" }
  }
}
```

| フィールド | 必須 | デフォルト | 説明 |
|-----------|------|-----------|------|
| name | ✅ | - | コンテナ名（Project 内でユニーク） |
| repository_url | ✅ | - | GitHub パブリックリポジトリ URL |
| branch | ❌ | `"main"` | チェックアウトするブランチ |
| directory | ❌ | `"/"` | Dockerfile のあるサブディレクトリ |
| env_vars | ❌ | `{}` | 環境変数（K8s Deployment に注入） |
| replicas | ❌ | `1` | Pod レプリカ数 |
| resources | ❌ | 省略可 | CPU/Memory のリクエスト・リミット |

**Response 201**

```json
{
  "data": {
    "container": {
      "id": "cont_xyz789",
      "project_id": "proj_abc123",
      "name": "frontend",
      "image_id": "img_aaa111",
      "repository_url": "https://github.com/org/repo",
      "branch": "main",
      "directory": "/",
      "version": null,
      "replicas": 1,
      "env_vars": { "NODE_ENV": "production", "PORT": "3000" },
      "resources": {
        "requests": { "cpu": "100m", "memory": "128Mi" },
        "limits":   { "cpu": "500m", "memory": "512Mi" }
      },
      "status": "Stopped",
      "created_at": "2025-01-01T00:00:00Z"
    },
    "build_job": {
      "id": "bj_111aaa",
      "project_id": "proj_abc123",
      "container_id": "cont_xyz789",
      "repository_url": "https://github.com/org/repo",
      "branch": "main",
      "directory": "/",
      "status": "Queued",
      "started_at": null,
      "finished_at": null,
      "created_at": "2025-01-01T00:00:00Z"
    }
  }
}
```

**エラーケース**

| Status | code | 条件 |
|--------|------|------|
| 404 | `NOT_FOUND` | Project が存在しない |
| 403 | `FORBIDDEN` | owner_id 不一致 |
| 409 | `CONFLICT` | 同 Project 内で name が重複 |

**フロー**

```
1. JWT から owner_id を取得
2. Project を id で取得 → なし: 404 / owner 不一致: 403
3. 同 Project 内で name 重複チェック → 重複: 409
4. DB に以下のレコードを順次作成:
   a. Image レコード（type: "user", name: "{project.name}-{container.name}"）
   b. Container レコード（status: "Stopped", image_id: 上記 Image の id）
   c. Service レコード（type: "LoadBalancer", ports: []）
   d. BuildJob レコード（status: "Queued"）
      - repository_url / branch / directory を BuildJob にスナップショット保存
5. Container + BuildJob をレスポンスとして返す
   ※ 実際のビルドはキューワーカーが非同期で処理
```

---

### PATCH /v1/containers/:id

コンテナ設定更新 + 再ビルド実行

**Request**（全フィールド省略可能、変更したいものだけ指定）

```json
{
  "repository_url": "https://github.com/org/repo",
  "branch": "develop",
  "directory": "/apps/frontend",
  "env_vars": { "NODE_ENV": "staging" },
  "replicas": 2,
  "resources": {
    "requests": { "cpu": "200m", "memory": "256Mi" },
    "limits":   { "cpu": "1000m", "memory": "1Gi" }
  }
}
```

**Response 200**

```json
{
  "data": {
    "container": {
      "id": "cont_xyz789",
      "project_id": "proj_abc123",
      "name": "frontend",
      "image_id": "img_aaa111",
      "repository_url": "https://github.com/org/repo",
      "branch": "develop",
      "directory": "/apps/frontend",
      "version": "bj_111aaa",
      "replicas": 2,
      "env_vars": { "NODE_ENV": "staging" },
      "resources": {
        "requests": { "cpu": "200m", "memory": "256Mi" },
        "limits":   { "cpu": "1000m", "memory": "1Gi" }
      },
      "status": "Running",
      "created_at": "2025-01-01T00:00:00Z"
    },
    "build_job": {
      "id": "bj_222bbb",
      "project_id": "proj_abc123",
      "container_id": "cont_xyz789",
      "repository_url": "https://github.com/org/repo",
      "branch": "develop",
      "directory": "/apps/frontend",
      "status": "Queued",
      "started_at": null,
      "finished_at": null,
      "created_at": "2025-01-01T00:01:00Z"
    }
  }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Container を id で取得 → なし: 404
3. 紐づく Project の owner_id チェック → 不一致: 403
4. 現在の Project 全 Container 設定を ProjectHistory に自動スナップショット保存
   - version_name: 現在時刻（ISO8601）
   - config_data: 全 Container の JSON スナップショット
5. 指定されたフィールドのみ Container レコードを更新
6. BuildJob レコードを Queued で新規作成
   - 更新後の repository_url / branch / directory をスナップショット保存
7. Container + BuildJob をレスポンスとして返す
```

---

### POST /v1/containers/:id/rebuild

ソースコードから最新のイメージをビルドし直し、デプロイを実行する。

**Request**

```
Body: なし
```

**Response 200**

```json
{
  "data": {
    "container": { ... },
    "build_job": { ... }
  }
}
```

**フロー**

1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. 新しい `ImageID` を生成し Container レコードを更新
4. `BuildJob` レコードを `Queued` で作成
5. ビルドワーカーをキック
6. Container + BuildJob を返す

---

### POST /v1/containers/:id/redeploy

ビルド（イメージ作成）は行わず、**現在のイメージを使用して** Kubernetes へのデプロイのみをやり直す。

**Request**

```
Body: なし
```

**Response 200**

```json
{
  "data": { "container": { ... } }
}
```

**フロー**

1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. `Container.Status` を `Deploying` に更新
4. 現在の `ImageID` を元に `imageRef` を構築
5. `DeployToKubernetes` を非同期で実行
6. Container を返す

---

### DELETE /v1/containers/:id

コンテナ削除（K8s Deployment + 関連リソースのハードデリート）

**Response 200**

```json
{
  "data": { "id": "cont_xyz789" }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Container を id で取得 → なし: 404
3. 紐づく Project の owner_id チェック → 不一致: 403
4. 当該 Container の BuildJob（Queued/Running）を全て Cancelled に更新
   - Running のものは先に K8s Job を削除してから Cancelled に更新
5. K8s リソースを削除（client-go）
   - Deployment（存在する場合）
   - Service（存在する場合）
   - Ingress（存在する場合）
6. DB レコードをハードデリート
   - ContainerLog / BuildJob / Ingress / Service / Image / Container の順
7. id を返す
```

---

### GET /v1/containers/:id/build-jobs

コンテナのビルド履歴一覧（ログ本体は含まない）

**Response 200**

```json
{
  "data": {
    "items": [
      {
        "id": "bj_222bbb",
        "project_id": "proj_abc123",
        "container_id": "cont_xyz789",
        "repository_url": "https://github.com/org/repo",
        "branch": "develop",
        "directory": "/apps/frontend",
        "status": "Success",
        "started_at": "2025-01-01T00:01:00Z",
        "finished_at": "2025-01-01T00:03:00Z",
        "created_at": "2025-01-01T00:01:00Z"
      }
    ],
    "total": 5
  }
}
```

> `build_log` フィールドはこの一覧に含まない。ログ取得は SSE エンドポイントを使用。

**フロー**

```
1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. container_id で BuildJob を created_at DESC で全件取得
4. レスポンスを返す
```

---

### GET /v1/containers/:id/logs

コンテナ実行ログ一覧取得（ContainerLog テーブルから過去 7 日分）

**Response 200**

```json
{
  "data": {
    "items": [
      {
        "id": "cl_aaa111",
        "container_id": "cont_xyz789",
        "pod_name": "frontend-pod-abc12",
        "log": "Server listening on :3000",
        "collected_at": "2025-01-01T01:00:00Z"
      }
    ],
    "total": 120
  }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. ContainerLog を container_id でフィルタし collected_at ASC で全件取得
4. レスポンスを返す
```

---

## 5. Build Jobs API

### POST /v1/build-jobs/:id/cancel

ビルドキャンセル（Queued / Running 両対応）

**Request**

```
Body: なし
```

**Response 200**

```json
{
  "data": {
    "id": "bj_111aaa",
    "project_id": "proj_abc123",
    "container_id": "cont_xyz789",
    "repository_url": "https://github.com/org/repo",
    "branch": "main",
    "directory": "/",
    "status": "Cancelled",
    "started_at": "2025-01-01T00:01:00Z",
    "finished_at": "2025-01-01T00:02:30Z",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**エラーケース**

| Status | code | 条件 |
|--------|------|------|
| 404 | `NOT_FOUND` | BuildJob が存在しない |
| 403 | `FORBIDDEN` | owner_id 不一致 |
| 400 | `BAD_REQUEST` | すでに `Success` / `Failed` / `Cancelled` 状態 |

**フロー**

```
1. JWT から owner_id を取得
2. BuildJob を id で取得 → なし: 404
3. 紐づく Project の owner_id チェック → 不一致: 403
4. Status チェック
   - Success / Failed / Cancelled → 400
   - Queued  → BuildJob.Status を Cancelled に更新（K8s Job 操作不要）
   - Running → railpackClient.Cancel(ctx, railpackJobID) で K8s Job を削除
               → BuildJob.Status を Cancelled に更新
               → Container.Status を Failed に更新
5. BuildJob.finished_at に現在時刻をセット
6. 更新後の BuildJob を返す
```

> `railpackJobID` は BuildJob 作成時に `railpack.Client.Build()` が返す jobID を
> `BuildJob` テーブルの専用カラム（`railpack_job_id`）に保存しておく。

---

## 6. Networking API

### PATCH /v1/containers/:id/service

Service のポート設定変更（ports 配列を丸ごと差し替え）

**Request**

```json
{
  "type": "LoadBalancer",
  "ports": [
    { "name": "http",    "port": 80,   "target": 3000 },
    { "name": "metrics", "port": 9090, "target": 9090 }
  ]
}
```

**Response 200**

```json
{
  "data": {
    "id": "svc_aaa111",
    "container_id": "cont_xyz789",
    "type": "LoadBalancer",
    "ports": [
      { "name": "http",    "port": 80,   "target": 3000 },
      { "name": "metrics", "port": 9090, "target": 9090 }
    ],
    "internal_ip": "10.96.0.10",
    "external_ip": null,
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. DB の Service レコードを更新（ports / type を丸ごと上書き）
4. client-go で K8s Service をパッチ
   - spec.ports を新しい ports 配列で差し替え
5. 更新後の Service を返す
```

---

### POST /v1/containers/:id/ingress

外部公開（Ingress 有効化 + サブドメイン自動払い出し）

**Request**

```json
{
  "http_port": 80
}
```

**Response 201**

```json
{
  "data": {
    "id": "ing_bbb222",
    "container_id": "cont_xyz789",
    "subdomain": "my-app-frontend.launchs.org",
    "http_port": 80,
    "tls_enabled": false,
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**エラーケース**

| Status | code | 条件 |
|--------|------|------|
| 409 | `CONFLICT` | すでに Ingress が存在する |

**フロー**

```
1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. 既存 Ingress チェック → あり: 409
4. サブドメインを自動生成: "{project.name}-{container.name}.launchs.org"
5. DB に Ingress レコードを作成（tls_enabled: false 固定）
6. client-go で K8s Ingress リソースを作成
7. 作成した Ingress を返す
```

---

### DELETE /v1/containers/:id/ingress

外部公開停止

**Response 200**

```json
{
  "data": { "id": "ing_bbb222" }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. Ingress を取得 → なし: 404
4. client-go で K8s Ingress リソースを削除
5. DB の Ingress レコードをハードデリート
6. id を返す
```

---

## 7. History / Rollback API

### GET /v1/projects/:id/histories

プロジェクトの自動スナップショット一覧

**Response 200**

```json
{
  "data": {
    "items": [
      {
        "id": "ph_ccc333",
        "project_id": "proj_abc123",
        "version_name": "2025-01-01T00:05:00Z",
        "created_at": "2025-01-01T00:05:00Z"
      }
    ],
    "total": 10
  }
}
```

> `config_data`（全 Container 設定の JSON）はサイズが大きいためこの一覧には含まない。

**フロー**

```
1. JWT から owner_id を取得
2. Project を取得 → なし: 404 / owner 不一致: 403
3. ProjectHistory を project_id でフィルタし created_at DESC で全件取得
4. レスポンスを返す
```

---

### POST /v1/projects/:id/rollback/:phid

指定履歴へのロールバック実行

**Request**

```
Body: なし
```

**Response 200**

```json
{
  "data": {
    "history": {
      "id": "ph_ccc333",
      "project_id": "proj_abc123",
      "version_name": "2025-01-01T00:05:00Z",
      "created_at": "2025-01-01T00:05:00Z"
    },
    "build_jobs": [
      {
        "id": "bj_333ccc",
        "container_id": "cont_xyz789",
        "status": "Queued",
        "created_at": "2025-01-01T01:00:00Z"
      },
      {
        "id": "bj_444ddd",
        "container_id": "cont_def456",
        "status": "Queued",
        "created_at": "2025-01-01T01:00:00Z"
      }
    ]
  }
}
```

**フロー**

```
1. JWT から owner_id を取得
2. Project を取得 → なし: 404 / owner 不一致: 403
3. ProjectHistory を phid で取得 → なし: 404
4. config_data から全 Container 設定を復元
5. 各 Container レコードを復元設定で更新
   （branch / directory / env_vars / resources / replicas）
6. 各 Container に対して BuildJob を Queued で発行
   - 保存済みイメージタグ（version = BuildJob.ID）を使って
     K8s Deployment をローリングアップデート
7. 復元した History + 発行した BuildJob 一覧を返す
   ※ 既存 Deployment はロールバック完了まで稼働し続ける
```

---

## 8. SSE ストリーミング仕様

### 共通ヘッダー（SSE レスポンス）

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

### 再接続ポリシー

- 切断時は**常に先頭から全ログを再送信**（Last-Event-ID 非対応、シンプル方式）
- 再接続ロジックはクライアント責任

---

### GET /v1/stream/build-jobs/:id

ビルドログのリアルタイムストリーミング

**イベント種別**

| event 名 | タイミング | data の内容 |
|----------|-----------|------------|
| `history` | 接続直後（過去ログ全件） | BuildJob.BuildLog の全文字列 |
| `live` | ビルド中のリアルタイム行 | 新規ログ行 1 行（railpack コンテナプレフィックス付き） |
| `status` | BuildJob の状態変化時 | BuildJob の status |
| `done` | ビルド完了時（Success / Failed） | 最終 status と finished_at |
| `error` | サーバーエラー時 | エラーメッセージ |

**SSE イベントフォーマット**

```
// 接続直後：過去ログを一括送信（BuildJob.BuildLog の全バイト）
event: history
data: {"log": "[git-clone] Cloning into '/workspace/repo'...\n[railpack] Preparing build plan...\n"}

// リアルタイム追従（railpack.StreamLogs の logCh から 1 行ずつ）
event: live
data: {"log": "[buildctl] Step 3/5 : COPY package.json .\n"}

// ステータス変化通知
event: status
data: {"status": "Running"}

// 完了通知
event: done
data: {"status": "Success", "finished_at": "2025-01-01T00:03:00Z"}

// エラー
event: error
data: {"message": "internal server error"}
```

**フロー**

```
1. JWT から owner_id を取得
2. BuildJob を取得 → なし: 404 / owner 不一致: 403
3. SSE ヘッダーを返し接続を確立
4. BuildJob.BuildLog の全データを history イベントとして送信
5-A. BuildJob.Status が Success / Failed / Cancelled（完了済み）の場合:
   - history 送信後、即座に done イベントを送信して接続を閉じる
5-B. BuildJob.Status が Queued / Running の場合:
   - DB を 1 秒ポーリングして BuildJob.BuildLog の差分を live イベントとして送信
     ※ BuildJob.BuildLog への追記はワーカー側が行う（後述）
   - BuildJob.Status 変化を検知したら status イベントを送信
   - Status が Success / Failed / Cancelled になったら done イベントを送信して接続を閉じる
```

---

### GET /v1/stream/containers/:id/logs

コンテナ実行ログのリアルタイムストリーミング

**イベント種別**

| event 名 | タイミング | data の内容 |
|----------|-----------|------------|
| `history` | 接続直後（ContainerLog テーブル全件） | 過去ログ配列 |
| `live` | 稼働中 Pod の新規ログ行 | pod_name + timestamp + message |
| `error` | サーバーエラー時 | エラーメッセージ |

**SSE イベントフォーマット**

```
// 過去ログ一括送信（ContainerLog テーブルから全件、collected_at ASC）
event: history
data: {
  "logs": [
    {
      "pod_name": "frontend-6d9b7c8f4-xk2pj",
      "timestamp": "2025-01-01T00:59:00Z",
      "message": "Server listening on :3000"
    },
    {
      "pod_name": "frontend-6d9b7c8f4-xk2pj",
      "timestamp": "2025-01-01T01:00:00Z",
      "message": "GET /health 200"
    }
  ]
}

// リアルタイムログ行（複数 Replica は全 Pod を集約）
event: live
data: {
  "pod_name": "frontend-6d9b7c8f4-xk2pj",
  "timestamp": "2025-01-01T01:05:00Z",
  "message": "GET /api/users 200"
}

// エラー
event: error
data: {"message": "container not running"}
```

**フロー**

```
1. JWT から owner_id を取得
2. Container を取得 → なし: 404 / owner 不一致: 403
3. Container.Status チェック
   - Stopped: history イベント（空）送信後、接続を閉じる
   - Failed:  ContainerLog の history を送信後、接続を閉じる
   - Building / Running: 以下のストリーミングを実施
4. SSE ヘッダーを返し接続を確立
5. ContainerLog テーブルの全件（collected_at ASC）を history イベントとして一括送信
6. client-go で全 Pod のログを Watch 開始（複数 Replica は並行 goroutine で集約）
7. 新規ログ行を live イベントとして送信しつつ ContainerLog テーブルに保存
8. クライアント切断 or Container 削除で Watch を停止して接続を閉じる
```

---

## 9. ビルドキューシステム詳細

### ワーカー設定（環境変数）

```bash
BUILD_WORKER_CONCURRENCY=5        # 同時実行ワーカー数
BUILD_WORKER_POLL_INTERVAL_SECONDS=5  # DB ポーリング間隔（秒）
BUILD_TIMEOUT=10m                 # railpack ビルドタイムアウト
```

### BuildJob テーブルへの追加カラム

既存仕様に加え、以下のカラムを追加する。

```go
type BuildJob struct {
    // ... 既存フィールド ...

    // railpack.Client.Build() が返す jobID（K8s Job のキャンセルに使用）
    RailpackJobID string `gorm:"index"`
}
```

### DB ポーリング + SELECT FOR UPDATE によるジョブ取り出し

```sql
-- ContainerID 単位で FIFO を守りながら、実行可能な Queued ジョブを取得
BEGIN;

SELECT * FROM build_jobs
WHERE status = 'Queued'
  AND container_id NOT IN (
    SELECT container_id FROM build_jobs WHERE status = 'Running'
  )
ORDER BY created_at ASC
LIMIT 1  -- ワーカー 1 スロット分ずつ取得
FOR UPDATE SKIP LOCKED;

-- 取得できたジョブを Running に更新
UPDATE build_jobs
SET status = 'Running', started_at = NOW()
WHERE id = ?;

COMMIT;
```

> 複数ワーカーが同時にポーリングしても `SKIP LOCKED` により同一ジョブを二重取得しない。

### ワーカー起動時のリカバリ処理

```
サーバー起動時（main 関数）:
1. BuildJob.Status = 'Running' のレコードを全件取得
2. 各ジョブに対して:
   a. RailpackJobID が空でない場合:
      → railpackClient.Cancel(ctx, railpackJobID) で K8s Job を削除
   b. BuildJob.Status を Failed に更新
   c. Container.Status を Failed に更新
   d. BuildJob.BuildLog に以下を追記:
      "[launchs] ビルドがサーバー再起動により強制終了されました"
3. ワーカーループを開始
```

### ビルドジョブ実行フロー（ワーカー内の詳細）

```
【ステップ 1: ジョブ取り出し】
SELECT FOR UPDATE SKIP LOCKED でジョブを 1 件取得
→ Container.Status を "Building" に更新

【ステップ 2: railpack.Client の初期化】
client, err := railpack.New(clientset, railpack.BuildConfig{
    GitRepo:        buildJob.RepositoryURL,
    GitBranch:      buildJob.Branch,
    Subdir:         buildJob.Directory,
    ImageName:      container.Name,          // コンテナ名
    ImageTag:       buildJob.ID,             // BuildJob の UUID をタグとして使用
    UploadEndpoint: os.Getenv("UPLOAD_ENDPOINT"),
    UploadToken:    os.Getenv("UPLOAD_TOKEN"),
    Namespace:      os.Getenv("BUILD_NAMESPACE"),
    Timeout:        parseDuration(os.Getenv("BUILD_TIMEOUT")),
})

【ステップ 3: ビルド開始】
railpackJobID, err := client.Build(ctx)
→ BuildJob.RailpackJobID = railpackJobID を DB に保存

【ステップ 4: ログ収集と Wait を並行実行】
// goroutine A: StreamLogs でログを逐次 BuildJob.BuildLog に追記
go func() {
    logCh, errCh := client.StreamLogs(ctx, railpackJobID)
    for line := range logCh {
        appendToBuildLog(buildJob.ID, line + "\n")  // DB に追記
    }
    if err := <-errCh; err != nil {
        appendToBuildLog(buildJob.ID, "[error] " + err.Error())
    }
}()

// goroutine B: Wait でビルド完了を待機
status, err := client.Wait(ctx, railpackJobID)

【ステップ 5-A: ビルド成功（status == StatusComplete）】
- Container.Version を buildJob.ID（= ImageTag）に更新
- K8s Deployment をローリングアップデート（イメージタグを新タグに差し替え）
  ※ Deployment は削除せず spec.template.spec.containers[].image のみ更新
- BuildJob.Status を "Success" に更新、finished_at をセット
- Container.Status を "Running" に更新

【ステップ 5-B: ビルド失敗（status == StatusFailed または err != nil）】
- BuildJob.BuildLog にエラー内容を追記
- BuildJob.Status を "Failed" に更新、finished_at をセット
- Container.Status を "Failed" に更新
- K8s Deployment は直前の成功イメージのまま維持（ロールアウトしない）
```

### キャンセル処理フロー

```
POST /v1/build-jobs/:id/cancel を受信:

【Queued の場合】
1. BuildJob.Status を Cancelled に更新（K8s Job はまだ存在しない）
2. finished_at をセット

【Running の場合】
1. BuildJob.RailpackJobID を取得
2. railpackClient.Cancel(ctx, railpackJobID) で K8s Job を削除
3. BuildJob.Status を Cancelled に更新
4. finished_at をセット
5. Container.Status を Failed に更新
   ※ 直前の成功イメージが維持される
```

---

## 10. railpack 統合仕様

### railpack ライブラリの使用箇所まとめ

| 処理 | 使用する railpack メソッド |
|------|--------------------------|
| ビルド開始 | `client.Build(ctx)` → `railpackJobID` を取得して `BuildJob.RailpackJobID` に保存 |
| ログ収集 | `client.StreamLogs(ctx, railpackJobID)` → `logCh` を goroutine で読み取り DB に追記 |
| 完了待機 | `client.Wait(ctx, railpackJobID)` → `StatusComplete` or `StatusFailed` で分岐 |
| キャンセル | `client.Cancel(ctx, railpackJobID)` → K8s Job を強制削除 |

### ImageName / ImageTag の決め方

| フィールド | 値 | 例 |
|-----------|----|----|
| `ImageName` | `container.Name`（コンテナ名） | `"frontend"` |
| `ImageTag` | `buildJob.ID`（BuildJob の UUID） | `"550e8400-e29b-41d4-a716-446655440000"` |

ビルド成功後、`Container.Version = buildJob.ID` として保存。
これがそのまま K8s Deployment のイメージタグになる。

### BuildConfig の組み立て方

```go
cfg := railpack.BuildConfig{
    // Git ソース（BuildJob のスナップショット値を使う）
    GitRepo:   buildJob.RepositoryURL,
    GitBranch: buildJob.Branch,
    Subdir:    buildJob.Directory,  // "/" → "." に変換して渡す

    // 成果物（イメージ名とタグ）
    ImageName: container.Name,
    ImageTag:  buildJob.ID,

    // tar 送信先（環境変数から）
    UploadEndpoint: os.Getenv("UPLOAD_ENDPOINT"),
    UploadToken:    os.Getenv("UPLOAD_TOKEN"),

    // K8s
    Namespace: os.Getenv("BUILD_NAMESPACE"),

    // リソース（Container.Resources から変換）
    Resources: railpack.ResourceConfig{
        BuildCPU:    "2",
        BuildMemory: "2Gi",
        BuildDisk:   "3Gi",
        InitCPU:     "500m",
        InitMemory:  "512Mi",
        PushCPU:     "500m",
        PushMemory:  "512Mi",
    },

    Timeout: parseDuration(os.Getenv("BUILD_TIMEOUT")),
}

// ※ Subdir の変換: railpack は "." をルートとして扱う
if cfg.Subdir == "/" || cfg.Subdir == "" {
    cfg.Subdir = "."
}
```

---

## 11. K8s 状態同期仕様

### Deployment Informer による Container.Status 自動同期

```
サーバー起動時に client-go SharedInformer を起動:
  対象: 全 Namespace の Deployment リソース
  ResyncPeriod: 30s

Deployment 変化イベント（Add / Update）を受信するたびに:
1. Deployment.Labels["container-id"] から container_id を特定
   ※ Deployment 作成時に Labels: {"managed-by": "launchs", "container-id": id} を付与
2. Deployment.Status.ReadyReplicas を確認
   - ReadyReplicas >= 1 かつ Container.Status != "Building"
     → Container.Status を "Running" に更新
   - ReadyReplicas == 0 かつ DesiredReplicas > 0 かつ Container.Status == "Running"
     → Container.Status を "Failed" に更新
     （CrashLoopBackOff 等の全 Pod 停止を検知）
3. DB の Container.Status を更新
```

### Container.Status 遷移まとめ

```
[作成時]
  POST /containers → Status: Stopped

[ビルド開始]
  ワーカーがジョブを Running に変えると → Status: Building

[ビルド成功]
  ワーカーが K8s Deployment をロールアウト → Informer が ReadyReplicas >= 1 を検知
  → Status: Running

[ビルド失敗 / K8s Job クラッシュ]
  ワーカーが Failed を検知 → Status: Failed（直前イメージで K8s Deployment 維持）

[稼働中に K8s 異常]
  Informer が ReadyReplicas == 0 を検知 → Status: Failed

[サーバー再起動時のリカバリ]
  孤立した Running BuildJob → Status: Failed（起動時に一括リセット）
```

### K8s リソースと DB レコードの対応表

| DB レコード | K8s リソース | 作成タイミング | 削除タイミング |
|------------|------------|-------------|-------------|
| Project | Namespace | POST /projects | DELETE /projects（非同期） |
| Container | Deployment | BuildJob 成功後（ワーカーが作成） | DELETE /containers |
| Service | K8s Service | POST /containers | DELETE /containers |
| Ingress | K8s Ingress | POST /ingress | DELETE /ingress |
| BuildJob | K8s Job（railpack） | ワーカーが Running に変えた時 | ビルド完了後に自動削除（TTL 600s） |

---

## 12. tar 受取・レジストリプッシュ仕様

### エンドポイント

```
POST /internal/upload
```

> このエンドポイントは外部公開しない。K8s Job Pod（同クラスタ内）からのみアクセスされる。
> K8s Service 経由の内部 DNS で到達する。
> URL: `http://launchs-backend.default.svc.cluster.local:8080/internal/upload`

### リクエスト（K8s Job Pod → バックエンド）

```
Method: POST
Headers:
  Authorization: Bearer {UPLOAD_TOKEN}
  Content-Type: application/octet-stream
  X-Job-Id:     {railpackJobID}   ← railpack 内部の jobID（K8s Job の UUID）
  X-Image-Name: {container.Name}
  X-Image-Tag:  {buildJob.ID}     ← BuildJob の UUID = ImageTag
Body: tar バイナリ
```

### 受取ハンドラーの処理フロー

```
POST /internal/upload を受信:

1. Authorization ヘッダーを検証
   - "Bearer {UPLOAD_TOKEN}" と一致しない → 401

2. X-Job-Id / X-Image-Name / X-Image-Tag ヘッダーの存在確認
   - いずれか欠落 → 400

3. tar を一時ファイルに保存
   - 保存先: {TAR_SAVE_DIR}/{X-Job-Id}.tar
   - ディレクトリが存在しない場合は自動作成

4. crane を使ってレジストリにプッシュ
   image, err := crane.Load(tarPath)

   opts := []crane.Option{
       crane.WithAuth(&authn.Basic{
           Username: os.Getenv("REGISTRY_USERNAME"),
           Password: os.Getenv("REGISTRY_PASSWORD"),
       }),
   }
   if os.Getenv("REGISTRY_INSECURE") == "true" {
       opts = append(opts, crane.WithTransport(&http.Transport{
           TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
       }))
   }

   imageRef := fmt.Sprintf("%s/%s/%s:%s",
       os.Getenv("REGISTRY_HOST"),
       os.Getenv("REGISTRY_PROJECT"),
       imageName,   // X-Image-Name
       imageTag,    // X-Image-Tag
   )
   // 例: 172.33.0.1/launchs/frontend:550e8400-e29b-41d4-a716-446655440000

   err = crane.Push(image, imageRef, opts...)

5. tar ファイルを削除
   os.Remove(tarPath)

6. レスポンス
   - プッシュ成功 → 202 Accepted
   - プッシュ失敗 → 500（BuildJob.BuildLog にエラー追記はワーカー側で行う）
```

### ワーカーとの連携

`/internal/upload` でプッシュが完了した後、ワーカー側の `client.Wait()` が `StatusComplete` を返す。
これはワーカーが `railpack.Client.Wait()` でポーリングしているためで、tar-push コンテナの正常終了 = K8s Job の成功 = `Wait()` の完了を意味する。

```
tar-push コンテナの curl 成功
    ↓
K8s Job.Status.Succeeded = 1
    ↓
railpack.Client.Wait() が StatusComplete を返す
    ↓
ワーカーが K8s Deployment をロールアウト
    ↓
Container.Status = Running
```

---

## 13. ログローテートバッチ

### 設定（環境変数）

```bash
LOG_ROTATE_INTERVAL_HOURS=24       # 実行間隔（デフォルト: 24時間）
BUILD_LOG_RETENTION_DAYS=30        # ビルドログ保持日数
CONTAINER_LOG_RETENTION_DAYS=7     # コンテナログ保持日数
```

### バッチ処理内容

```
サーバー起動時に time.Ticker でバックグラウンド実行（UTC 03:00 相当）:

1. BuildJob.BuildLog のクリア
   UPDATE build_jobs
   SET build_log = NULL
   WHERE finished_at < NOW() - INTERVAL '{BUILD_LOG_RETENTION_DAYS} days'
     AND build_log IS NOT NULL;

2. ContainerLog の物理削除
   DELETE FROM container_logs
   WHERE collected_at < NOW() - INTERVAL '{CONTAINER_LOG_RETENTION_DAYS} days';
```

---

## 14. Volume API

### POST /v1/containers/:id/volumes

コンテナに紐づく永続化ボリューム（PVC）を作成する。

> [!IMPORTANT]
> ボリュームの作成・削除を実際のマウント状態に反映させるには、コンテナの **「再デプロイ」** または **「再ビルド」** が必要です。作成しただけではコンテナ内からは見えません。

**Request**

```json
{
  "name": "uploads-storage",
  "size_mb": 1024,
  "mount_path": "/uploads"
}
```

| フィールド | 必須 | デフォルト | 説明 |
|-----------|------|-----------|------|
| name | ✅ | - | ボリューム識別名（英数字・ハイフン） |
| size_mb | ✅ | - | サイズ（MB単位）。最大 5120 (5GB) |
| mount_path | ✅ | - | コンテナ内の絶対パス |

**Response 201**

```json
{
  "data": {
    "id": "vol-abc123",
    "project_id": "proj_xyz",
    "container_id": "cont_123",
    "name": "uploads-storage",
    "size_mb": 1024,
    "mount_path": "/uploads",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**フロー**

1. JWT から owner_id を取得
2. Container/Project を取得し権限チェック
3. `size_mb` が 5120 以下であることを確認
4. `volumeID`（`vol-{uuid}`）を生成
5. client-go で `PersistentVolumeClaim` (PVC) を作成
   - 名前: `pvc-{volumeID}`
   - 属性: ReadWriteOnce
6. DB に Volume レコードを作成
7. 成功レスポンスを返す
   ※ 実際にコンテナにマウントされるのは次回のデプロイ（Rebuild/Redeploy）時。

---

### GET /v1/containers/:id/volumes

コンテナに紐づくボリューム一覧を取得する。

**Response 200**

```json
{
  "data": {
    "items": [...],
    "total": 1
  }
}
```

---

### DELETE /v1/volumes/:id

ボリュームを削除する（非同期処理）。

**Response 200**

```json
{ "data": { "id": "vol-abc123" } }
```

**フロー**

1. ボリュームとプロジェクトを取得し権限チェック
2. DB のボリュームステータスを `Deleting` に更新
3. client-go で `pvc-{volumeID}` を削除
4. レスポンスを即座に返す
5. バックグラウンドの同期プロセスが、K8s上から実際にPVCが消えたことを確認した後、DBレコードを物理削除する

---

## 15. 永続化ボリューム（Volume）詳細仕様

### K8s 連携

デプロイ時（`DeployToKubernetes`）に以下の処理を動的に行う：

1. `GetVolumesByContainerID(id)` でマウント対象を取得
   - ※ `Status = 'Available'` のもののみを対象とする
2. Deployment の `spec.template.spec.containers[0].volumeMounts` に追加：
   ```yaml
   - name: vol-{volumeID}
     mountPath: {volume.mount_path}
   ```
3. Deployment の `spec.template.spec.volumes` に追加：
   ```yaml
   - name: vol-{volumeID}
     persistentVolumeClaim:
       claimName: pvc-{volumeID}
   ```

### 制限事項

*   **単一ノード限定**: `AccessMode: ReadWriteOnce` のため、マルチノードでの共有は考慮しない（今回のユースケースに準拠）。
*   **クォータ**: プロジェクト全体ではなく、ボリューム単位で最大 5GB の制限を設ける。
*   **反映タイミング**: ボリュームの作成・削除は、コンテナの再起動（デプロイ）を伴うまでファイルシステムには反映されない。
