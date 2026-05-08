# Plan: Monolith → Controller / Builder / Watcher マイクロサービス分割

## Context

現在の `backend/` は Echo + Go の単一プロセスで、API・ビルド・デプロイ・K8s監視を全部持っている。
`builder/` コンテナは既にあるが中身は "Hello, World!" のスタブ。
これを 3 サービスに分割し、**各コンテナが複数レプリカで動作できる**ようにする。

---

## 現状の相関関係

### DB エンティティ ↔ K8s リソース対応表

| DBエンティティ | K8sリソース | 名前空間 | 対応ルール |
|--------------|-----------|---------|----------|
| Project | Namespace | cluster | `ns-{projectID}` |
| Project | NetworkPolicy | `ns-{projectID}` | `allow-traefik-cloudflared-local` (固定1件) |
| Container | Deployment | `ns-{projectID}` | 名前 = `container.name` |
| Service (DB) | Service (K8s) | `ns-{projectID}` | `is_active=true` かつ ports が空でない場合のみ存在 |
| Ingress (DB) | IngressRoute (Traefik CRD) | `ns-{projectID}` | 名前 = `container.name` |
| Volume | PersistentVolumeClaim | `ns-{projectID}` | `pvc-{volumeID}` |
| BuildJob | Job | `buildkit` | `railpack-{buildJobID}` (完了後 600秒で自動削除) |
| Image | — (Harbor レジストリ) | — | `{host}/{project}/{containerID}:{imageID}` |

### DB リレーション図

```
Project
 ├─ id, name, namespace (ns-{id}), owner_id
 └─[1:N]─ Container
            ├─ id, project_id, name, image_id, status, replicas, env_vars, resources
            ├─[1:1]─ Service    (container_id UNIQUE)
            │         └─ type, ports(JSON), is_active, internal_ip, external_ip
            ├─[1:1]─ Ingress   (container_id UNIQUE)
            │         └─ subdomain, http_port, tls_enabled, custom_domain
            ├─[1:N]─ Volume    (container_id nullable)
            │         └─ name, size_mb, mount_path, status (Available/Deleting)
            ├─[1:N]─ Image
            │         └─ type (user/system), name, registry
            └─[1:N]─ BuildJob
                      └─ status (Queued/Running/Succeeded/Failed), build_log(bytea)
```

### フロントエンド画面 ↔ API エンドポイント対応

| 画面 | 主な API 呼び出し | 表示データ |
|------|----------------|-----------|
| プロジェクト一覧 | `GET /v1/projects` | Project 一覧 |
| プロジェクト詳細 | `GET /v1/projects/:id` | Project + Container[] + Service + Ingress + Volume[] |
| コンテナ詳細 | `GET /v1/containers/:id` | Container + Service + Ingress + Volume[] |
| コンテナ作成 | `POST /v1/projects/:id/containers` | フォーム送信 → BuildJob 開始 |
| ビルドログ | `WS /v1/ws/build-jobs/:id` | BuildJob.build_log をリアルタイム表示 |
| コンテナログ | `WS /v1/ws/containers/:id/logs` | Deployment Pod ログ |
| ビルド履歴 | `GET /v1/containers/:id/build-jobs` | BuildJob[] |
| ネットワーク設定 | `PATCH /v1/containers/:id/service` `POST/PATCH/DELETE /v1/containers/:id/ingress` | Service + Ingress 設定 |
| ボリューム管理 | `POST/GET /v1/containers/:id/volumes` `DELETE /v1/volumes/:id` | Volume[] |

### エンティティ操作 ↔ K8s 副作用

```
プロジェクト作成 → Namespace + NetworkPolicy 作成
プロジェクト削除 → Namespace 削除（配下の全リソースが連鎖削除）

コンテナ作成    → BuildJob 作成 → K8s Job(buildkit) → Harbor Push → K8s Deployment 作成
コンテナ更新    → 新 Image + BuildJob → K8s Job → Harbor Push → K8s Deployment 更新
コンテナ削除    → K8s Deployment + Service + IngressRoute 削除
Rebuild         → 新 BuildJob → K8s Job → Harbor Push → K8s Deployment 更新
Redeploy        → K8s Deployment 更新（ビルドなし、現在の image_id を使用）

Service 有効化  → K8s Service 作成/更新（ClusterIP・外部IP を DB に同期）
Service 無効化  → K8s Service 削除
Ingress 作成    → Traefik IngressRoute 作成
Ingress 削除    → Traefik IngressRoute 削除

Volume 作成     → PVC 作成 → 次回 Deploy 時に Deployment にマウント
Volume 削除     → PVC 削除（非同期）→ PVC が消えたら DB レコードも削除
```

### K8s 監視 → DB/クライアント反映（現状の k8slogwatcher）

```
K8s Job 監視     → BuildJob.build_log 追記 + BuildJob.status 更新 → WebSocket 配信
K8s Deployment 監視 → Container.status 更新 → Pod ログを WebSocket 配信
PVC 監視（polling） → Volume.status = Deleting の PVC が消えたら DB レコード削除
```

---

## サービス責務の定義

| サービス | 責務 | スケーリング方式 |
|---------|------|----------------|
| **backend (controller)** | フロントエンドの受け口 + **全 K8s API 操作** + タスク処理（K8s Job 作成・Deployment 作成/削除） | 水平スケール可（`SELECT FOR UPDATE SKIP LOCKED` でタスク競合回避） |
| **builder** | tar アップロード受け取り + Harbor Push/Delete のみ | 水平スケール可（ステートレス） |
| **watcher** | K8s Job/Deployment の監視・ログ取得・DB 同期・Redis Pub/Sub 配信 | **リーダー選出あり**（1台が監視、他はスタンバイ） |

**設計原則:**
- K8s API を呼ぶのは **controller のみ**
- Harbor レジストリを操作するのは **builder のみ**
- 監視・状態同期をするのは **watcher のみ**

---

## タスクキュー設計（PostgreSQL ベース）

### tasks テーブル

```sql
CREATE TABLE tasks (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_type     VARCHAR NOT NULL,
  status        VARCHAR NOT NULL DEFAULT 'pending',
  payload       JSONB NOT NULL,
  timeout_at    TIMESTAMPTZ NOT NULL,        -- この時刻までに done にならなければキャンセル
  created_at    TIMESTAMPTZ DEFAULT now(),
  started_at    TIMESTAMPTZ,
  finished_at   TIMESTAMPTZ,
  error_message TEXT
);
CREATE INDEX idx_tasks_status_type ON tasks(status, task_type, created_at);
CREATE INDEX idx_tasks_timeout ON tasks(status, timeout_at);   -- タイムアウト監視用
```

### タスクのステータス遷移

```
pending
  │
  │（ワーカーが取得）
  ▼
running
  │
  ├──（正常完了）──▶ done
  ├──（処理失敗）──▶ failed
  └──（タイムアウト）─▶ cancelled（キャンセル処理後）
```

**ステータスの意味:**

| ステータス | 意味 |
|-----------|------|
| `pending` | 取得待ち |
| `running` | ワーカーが処理中 |
| `done` | 正常完了 |
| `failed` | 処理エラー |
| `cancelled` | タイムアウトまたは手動キャンセル → ロールバック実施済み |

### ワーカーのタスク取得（複数レプリカで競合なし）

```sql
-- タスク取得
UPDATE tasks SET status = 'running', started_at = now()
WHERE id = (
  SELECT id FROM tasks
  WHERE status = 'pending' AND task_type = $1
  ORDER BY created_at
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

### タスクのデフォルトタイムアウト

| タスク種別 | timeout_at の設定値 |
|-----------|-------------------|
| `build` | 作成時刻 + 30分 |
| `deploy` | 作成時刻 + 10分 |
| `delete_container` | 作成時刻 + 5分 |
| `delete_project` | 作成時刻 + 5分 |
| `delete_image` | 作成時刻 + 5分 |

新規タスク到着の即時通知: `pg_notify("task_created", task_type)` + `LISTEN`
（通知がなくても 5秒ポーリングでバックアップ）

---

## タイムアウト監視とキャンセル処理

controller と builder はそれぞれタイムアウト監視ゴルーチンを持つ。

### タイムアウト監視ループ（30秒ごとに実行）

```sql
SELECT * FROM tasks
WHERE status = 'running' AND timeout_at < now()
FOR UPDATE SKIP LOCKED;
```

取得できたタスクに対してキャンセル処理を実行し、`status → 'cancelled'` に更新。

### タスク種別ごとのキャンセル処理

| タスク種別 | キャンセル時のロールバック |
|-----------|------------------------|
| `build` | K8s Job を削除（`kubectl delete job railpack-{jobID} -n buildkit`）<br>BuildJob.status → キャンセル<br>Container.status → エラー |
| `deploy` | K8s Deployment を直前バージョンにロールバック（`kubectl rollout undo`）<br>Container.status → エラー |
| `delete_container` | 削除途中のリソースをログに記録、手動確認が必要な旨を DB に記録<br>task.error_message にロールバック不可の旨を保存 |
| `delete_project` | 同上（Namespace は部分削除の可能性あり） |
| `delete_image` | ログ記録のみ（Harbor のイメージはそのまま残る） |

---

## ステート管理（Redis キャッシュ + PostgreSQL）

### 書き込み時（DB がソース・オブ・トゥルース）

```
① PostgreSQL に書き込み（確定）
② Redis のキャッシュを削除または更新（TTL: 60秒）
```

### 読み出し時

```
① Redis から取得を試みる
② キャッシュミス → PostgreSQL から取得
③ Redis にキャッシュして返す（TTL: 60秒）
```

### Redis キー一覧

| キー | 内容 | TTL |
|------|------|-----|
| `cache:container:{id}` | Container レコード JSON | 60秒 |
| `cache:build_job:{id}` | BuildJob レコード JSON | 60秒 |
| `cache:project:{id}` | Project レコード JSON | 60秒 |
| `stream:job:{namespace}:{jobName}` | ビルドログ Pub/Sub | — |
| `stream:pod:{namespace}:{podName}` | コンテナログ Pub/Sub | — |
| `watcher:leader` | リーダー選出ロック（SETNX） | 15秒（定期更新） |

---

## タスク一覧

### controller が作成・処理するタスク

| タスク名 | task_type | 作成トリガー | K8s 操作 |
|---------|-----------|------------|---------|
| ビルドタスク | `build` | コンテナ作成・Rebuild | K8s Job 作成（buildkit 名前空間） |
| デプロイタスク | `deploy` | builder から通知 / Redeploy | K8s Deployment 作成・更新 |
| コンテナ削除タスク | `delete_container` | コンテナ削除 | Deployment・Service・Ingress・PVC 削除 |
| プロジェクト削除タスク | `delete_project` | プロジェクト削除 | Namespace 削除 |

### controller が直接実行する同期 K8s 操作

| 操作 | タイミング |
|------|-----------|
| Namespace 作成 | プロジェクト作成時 |
| Service 作成・更新・削除 | ネットワーク設定変更時 |
| Ingress 作成・更新・削除 | Ingress 設定変更時 |

### builder が処理するタスク

| タスク名 | task_type | トリガー | Harbor 操作 |
|---------|-----------|---------|------------|
| イメージ削除タスク | `delete_image` | controller が delete_container 処理時に作成 | Harbor レジストリからイメージ削除 |

> builder は tar アップロードを受け取った後、deploy タスクを **tasks テーブルに INSERT** するだけで、K8s は触らない。

### watcher が監視・同期する対象

| 対象 | DB 同期内容 | Redis 配信 |
|------|------------|-----------|
| K8s Job（buildkit ns） | BuildJob.Status（実行中/成功/失敗） | `stream:job:{ns}:{name}` |
| K8s Job の Pod ログ | BuildJob.BuildLog に追記 | `stream:job:{ns}:{name}` |
| K8s Deployment | Container.Status（起動中/稼働中/エラー） | `stream:pod:{ns}:{name}` |
| K8s Pod ログ（Deployment） | DB 保存なし | `stream:pod:{ns}:{name}` |
| K8s Service（LoadBalancer） | Service.ExternalIP を DB に同期 | — |

---

## 全体フロー

```
[Frontend]         [controller]         [PostgreSQL tasks]   [builder]          [K8s]            [watcher]
    │                    │                      │                 │                 │                  │
    │─ POST /containers ─▶│                      │                 │                 │                  │
    │                    │ BuildJob 作成          │                 │                 │                  │
    │                    │─ INSERT build task ──▶│                 │                 │                  │
    │                    │ K8s Job 作成           │                 │                 │                  │
    │                    │────────────────────────────────────────────────────────▶│                  │
    │                    │                       │                 │                 │◀─── watcher 監視 ─│
    │                    │                       │                 │                 │     ログ取得       │
    │                    │◀── Redis Pub/Sub ─────────────────────────────────────────── 配信            │
    │◀─ WebSocket ────────│                      │                 │                 │                  │
    │                    │                       │                 │◀── tar upload ──│(tar-push コンテナ)│
    │                    │                       │                 │ Harbor Push      │                  │
    │                    │                       │◀─ INSERT deploy task ──────────────│                  │
    │                    │ LISTEN / ポーリング     │                 │                 │                  │
    │                    │◀──────────────────────│                 │                 │                  │
    │                    │ K8s Deployment 作成    │                 │                 │                  │
    │                    │────────────────────────────────────────────────────────▶│                  │
    │                    │                       │                 │                 │◀─── watcher 監視 ─│
    │                    │                       │                 │                 │     Status 同期    │
    │                    │◀── DB + Redis キャッシュ更新 ───────────────────────────────────────────────│
```

---

## タスクペイロード定義（tasks.payload の JSONB）

**ビルドタスク（task_type: "build"）:**
```json
{
  "build_job_id":    "...",
  "container_id":    "...",
  "image_id":        "...",
  "project_id":      "...",
  "project_name":    "...",
  "container_name":  "...",
  "namespace":       "ns-...",

  "repository_url":  "https://github.com/...",
  "branch":          "main",
  "directory":       ".",

  "build_type":      "railpack",
  "dockerfile_path": "Dockerfile",

  "build_args": {
    "NODE_ENV": "production"
  },
  "resources": {
    "build_cpu":    "2",
    "build_memory": "2Gi",
    "build_disk":   "1Gi"
  }
}
```

> `build_type`: `"railpack"` → Railpack フロントエンドを使用。`"dockerfile"` → 指定した `dockerfile_path` を使用。

**デプロイタスク（task_type: "deploy"）:**
```json
{
  "container_id":  "...",
  "image_ref":     "172.33.0.1/launchs/{イメージ名}:{イメージタグ}",
  "namespace":     "ns-...",
  "build_job_id":  "..."
}
```

**コンテナ削除タスク（task_type: "delete_container"）:**
```json
{
  "container_id": "...",
  "namespace":    "ns-...",
  "image_name":   "..."
}
```

**プロジェクト削除タスク（task_type: "delete_project"）:**
```json
{
  "project_id": "...",
  "namespace":  "ns-..."
}
```

**イメージ削除タスク（task_type: "delete_image"）:**
```json
{
  "image_name":    "...",
  "image_tags":    ["tag1", "tag2"]
}
```

---

## コンテナビルドの詳細フロー

### Step 1 — controller がビルドタスクを投入し K8s Job を作成

```
フロントエンドから POST /v1/projects/:id/containers を受信
 ↓
BuildJob レコードを DB に作成（ステータス: 待機中）
 ↓
tasks テーブルに build タスクを INSERT（status: pending）
 ↓
controller の task ワーカーが LISTEN または 5秒ポーリングで検出
 ↓
SELECT FOR UPDATE SKIP LOCKED でタスク取得（複数レプリカ安全）
 ↓
JWT（JobToken）を生成: { ジョブID、イメージ名（コンテナID）、イメージタグ（imageID）}
 ↓
K8s Job を作成（名前空間: buildkit）
BuildJob.status → 実行中、started_at を記録
task.status → running
Redis キャッシュを削除
```

---

### Step 2 — K8s Job 内のコンテナ処理

| コンテナ | Image | 役割 | 出力先 |
|---------|-------|------|------|
| 初期化: `git-clone` | `alpine/git:latest` | リポジトリをクローン | `/workspace/repo` |
| 初期化: `railpack` | `ghcr.io/launchs-org/railpack-container:latest` | ビルドプランを生成 | `/workspace/railpack-plan.json` |
| メイン: `buildctl` | `moby/buildkit:master-rootless` | コンテナイメージをビルド | `/workspace/output.tar` + `/workspace/build.done` |
| メイン: `tar-push` | `alpine:latest` | tar ファイルを builder に送信 | HTTP POST → `/internal/upload` |

**`tar-push` の送信先:**
```
UPLOAD_URL = http://10.10.11.8:8091/internal/upload   # K8s Pod からホスト IP 経由
```

---

### Step 3 — watcher が K8s Job を監視

```
リーダー watcher が K8s Job の開始を検知（`watcher:leader` を Redis SETNX で保持）
 ↓
各コンテナのログをストリーミングで取得
 ↓
Redis PUBLISH stream:job:buildkit:railpack-{jobID}
 ↓
BuildJob.BuildLog に追記（AppendBuildLog）
Redis キャッシュを削除
 ↓
Job 完了時 → BuildJob.status を「成功」または「失敗」に更新
```

非リーダーの watcher は Redis を SUBSCRIBE するのみ（K8s には触らない）。

---

### Step 4 — builder が tar を受け取り Harbor に Push

```
builder の Echo サーバー（ポート 8091）で POST /internal/upload を受信
 ↓
JWT を検証してジョブID・イメージ名・イメージタグを取得
 ↓
tar ファイルを ./launchs-tar/{jobID}.tar に一時保存
 ↓
Harbor に Push（最大 5 回リトライ）
  送信先: {REGISTRY_HOST}/buildkit/{イメージ名}:{イメージタグ}
 ↓
成功: 一時 tar ファイルを削除
BuildJob.status → 成功、FinishedAt を記録
Redis キャッシュを削除
 ↓
tasks テーブルに deploy タスクを INSERT
pg_notify("task_created", "deploy") で通知
```

---

### Step 5 — controller が deploy タスクを処理

```
LISTEN "task_created" で通知を受信
 ↓
SELECT FOR UPDATE SKIP LOCKED で deploy タスクを取得
 ↓
K8s Deployment を作成または更新（名前空間: ns-{projectID}）
Container.status → デプロイ中
task.status → running
Redis キャッシュを削除
```

---

### Step 6 — watcher が Deployment を監視し DB 同期

```
リーダー watcher が K8s Deployment のステータス変化を Watch
 ↓
Container.status を DB に同期（起動中 → 稼働中 / エラー）
Redis キャッシュを削除
 ↓
Pod のログをストリーミングで取得
 ↓
Redis PUBLISH stream:pod:{名前空間}:{Pod名}
```

controller が Redis SUBSCRIBE → WebSocket でクライアントに配信。

---

### Step 7 — コンテナ削除フロー（Harbor イメージ削除含む）

```
フロントエンドから DELETE /v1/containers/:id を受信
 ↓
controller: tasks テーブルに delete_container タスクを INSERT
            tasks テーブルに delete_image タスクを INSERT（builder 向け）
 ↓
【controller が処理】delete_container タスク
  K8s Deployment・Service・Ingress・PVC を削除
  Container レコードを削除（DB）
  Redis キャッシュを削除
 ↓
【builder が処理】delete_image タスク
  Harbor レジストリからイメージを削除
  task.status → 完了
```

---

## スケーリング設計

### controller（水平スケール）

- REST API はステートレス
- task ワーカーは `SELECT FOR UPDATE SKIP LOCKED` で競合なし
- 複数レプリカが同じ tasks テーブルを参照しても安全

### builder（水平スケール）

- アップロードハンドラはステートレス
- tar ファイルは `./launchs-tar/{jobID}.tar` に一時保存（同一ジョブは同一 builder に届く）
- delete_image タスクも `SELECT FOR UPDATE SKIP LOCKED` で競合なし

### watcher（リーダー選出あり）

```
Redis SETNX watcher:leader {podID} PX 15000
 ↓
取得成功: リーダー → K8s Watch 開始、10秒ごとに TTL 更新
取得失敗: スタンバイ → Redis SUBSCRIBE で受信転送のみ
 ↓
リーダーが落ちた場合: TTL 切れ後（最大 15秒）に他のインスタンスがリーダーに昇格
```

---

## watcher 新規実装方針

既存の `k8slogwatcher` パッケージは**再利用しない**。シンプルな実装を新規作成する。

```go
// watcher/src/watcher/job.go
func WatchJobs(ctx context.Context, k8sClient, dbClient, redisClient) {
    // K8s Job の Watch ストリームを開く
    // イベントごとに BuildJob DB 更新 + Redis Publish
    // Pod ログは goroutine でストリーミング
}

// watcher/src/watcher/deployment.go
func WatchDeployments(ctx context.Context, k8sClient, dbClient, redisClient) {
    // K8s Deployment の Watch ストリームを開く
    // ステータス変化を Container DB に同期 + Redis キャッシュ削除
    // Pod ログは goroutine でストリーミング → Redis Publish
}

// watcher/src/leader/leader.go
func RunWithLeaderElection(ctx context.Context, redisClient, fn func(ctx context.Context)) {
    // Redis SETNX でリーダー選出
    // リーダーなら fn() を実行、TTL を定期更新
    // TTL が切れたら再選出を試みる
}
```

---

## ディレクトリ構成

```
backend/
├── shared/                         ← 新規: 共有 Go モジュール
│   ├── go.mod                      (module launchs/shared)
│   ├── model/                      (BuildJob, Container, Project, ...)
│   ├── database/                   (DB/K8s/Redis 初期化)
│   ├── utils/jwt.go
│   └── logger/
│
├── backend/                        ← controller サービス
│   └── src/
│       ├── go.mod                  (replace launchs/shared => ../../shared)
│       ├── service/
│       │   ├── project.go          ← Namespace 作成（K8s 直接）
│       │   ├── container.go        ← タスク INSERT に変更
│       │   ├── networking.go       ← Service/Ingress パッチ（K8s 直接）
│       │   ├── volume.go           ← PVC 作成・StartVolumeSync
│       │   └── build.go            ← StreamBuildJobLogs のみ残す
│       ├── worker/
│       │   ├── build_worker.go     ← build タスクを取得 → K8s Job 作成
│       │   ├── deploy_worker.go    ← deploy タスクを取得 → K8s Deployment 作成
│       │   ├── delete_worker.go    ← delete タスクを取得 → K8s リソース削除
│       │   └── timeout_worker.go   ← タイムアウト監視 → K8s Job 削除・Deployment ロールバック
│       └── middlewares/
│
├── builder/                        ← builder サービス（スタブから実装）
│   └── src/
│       ├── go.mod                  (replace launchs/shared => ../../shared)
│       ├── main.go                 (Echo :8091 + delete_image ワーカー起動)
│       ├── worker/
│       │   ├── delete_image_worker.go ← delete_image タスクを取得 → Harbor 削除
│       │   └── timeout_worker.go      ← delete_image タイムアウト監視
│       ├── service/
│       │   └── image.go            ← PushToRegistry, DeleteFromRegistry (backend から移動)
│       └── controller/
│           └── upload.go           ← /internal/upload ハンドラ (backend から移動)
│
└── watcher/                        ← 新規サービス
    └── src/
        ├── go.mod                  (replace launchs/shared => ../../shared)
        ├── main.go                 (リーダー選出 → Watch 開始)
        ├── watcher/
        │   ├── job.go              ← K8s Job 監視（新規シンプル実装）
        │   └── deployment.go       ← K8s Deployment 監視（新規シンプル実装）
        └── leader/
            └── leader.go           ← Redis SETNX によるリーダー選出
```

---

## 共有コード（shared モジュール）

`backend/shared/` を Go モジュール (`launchs/shared`) として作成。  
各サービスの `go.mod` に `replace launchs/shared => ../../shared` を追加。  
Docker Compose では `./shared:/shared` をボリュームマウント。

**shared に含むもの（k8slogwatcher は含めない）:**
- `model/` — 全 GORM モデル + DB 操作メソッド
- `database/` — database.go, k8s.go, redis.go
- `utils/jwt.go` — JobToken 生成・検証
- `logger/`

---

## Docker Compose 変更

```yaml
services:
  backend:
    environment:
      UPLOAD_ENDPOINT: http://10.10.11.8:8091/internal/upload
    volumes:
      - ./shared:/shared

  builder:
    ports: ["8091:8091"]
    volumes:
      - ./builder/src:/app
      - ./shared:/shared
    env_file:
      - ./config/app.env

  watcher:
    build:
      context: ./watcher
    volumes:
      - ./watcher/src:/app
      - ./shared:/shared
      - ~/.kube/config:/root/.kube/config
    environment:
      POD_NAME: watcher-1
    env_file:
      - ./config/app.env
```

---

## 環境変数マッピング

| 変数 | backend | builder | watcher |
|------|---------|---------|---------|
| `DATABASE_DSN` | ✓ | ✓ | ✓ |
| `REDIS_ADDR` | ✓ | ✓ | ✓ |
| `UPLOAD_ENDPOINT` | ✓（K8s Job に渡す値） | — | — |
| `BUILD_NAMESPACE` | ✓（K8s Job 作成） | — | ✓（Job 監視） |
| `REGISTRY_HOST` | — | ✓ | — |
| `REGISTRY_PROJECT` | — | ✓ | — |
| `REGISTRY_INSECURE` | — | ✓ | — |
| `POD_NAME` | — | — | ✓（リーダー選出） |
| `GRPC_SERVER`（auth） | ✓ | — | — |

---

## 実装ステップ

### Phase 1 — shared モジュール抽出（動作変更なし）
1. `shared/go.mod` 作成（k8slogwatcher は含めない）
2. model/, database/, utils/, logger/ を shared にコピー
3. `backend/src/go.mod` に replace 追加・import パス更新
4. backend がコンパイル・動作することを確認

### Phase 2 — watcher サービス新規実装
5. `watcher/src/` に新規シンプル実装（job.go, deployment.go, leader.go）
6. docker-compose に watcher 追加
7. backend/src/main.go から `k8slogwatcher.Init()` を削除
8. watcher がリーダー選出・ログ配信・DB 同期できることを確認

### Phase 3 — builder サービス構築
9. `builder/src/go.mod` 作成
10. service/image.go, controller/upload.go を builder に移動
11. `worker/delete_image_worker.go` 作成
12. `builder/src/main.go` — Echo :8091 + ワーカー起動
13. docker-compose で builder port 8091 公開

### Phase 4 — controller にタスクワーカーを実装
14. `worker/build_worker.go` — build タスクを取得して K8s Job 作成（railpack を backend 内に残す）
15. `worker/deploy_worker.go` — deploy タスクを取得して K8s Deployment 作成
16. `worker/delete_worker.go` — delete タスクを取得して K8s リソース削除 + delete_image タスクを INSERT
17. `worker/timeout_worker.go` — タイムアウト超過タスクを検出し K8s Job 削除 / Deployment ロールバック実行

### Phase 5 — controller の API をタスク投入に切り替え
17. `container.go` の `go startRailpackBuild(...)` を tasks INSERT に変更
18. `redeploy` も tasks INSERT に変更
19. `container.go` の削除処理も tasks INSERT に変更
20. `UPLOAD_ENDPOINT` を builder アドレスに変更
21. backend から移動済みファイルを削除・go.mod 整理

### Phase 6 — 整理・確認
22. 各 go.mod から不要依存を削除
23. Dockerfile に shared マウント対応を追加

---

## 変更対象ファイル

**新規作成:**
- `shared/` (go.mod + model, database, utils, logger)
- `watcher/src/watcher/job.go`, `watcher/src/watcher/deployment.go`
- `watcher/src/leader/leader.go`, `watcher/src/main.go`
- `builder/src/worker/delete_image_worker.go`
- `builder/src/main.go`（実装）
- `backend/src/worker/build_worker.go`
- `backend/src/worker/deploy_worker.go`
- `backend/src/worker/delete_worker.go`

**移動（backend → builder）:**
- `backend/src/service/image.go` → `builder/src/service/image.go`（PushToRegistry + 削除追加）
- `backend/src/controller/upload.go` → `builder/src/controller/upload.go`

**controller 内に残す:**
- `backend/src/railpack/` — K8s Job 作成は controller が担当するため残す
- `backend/src/service/deploy.go` — K8s Deployment 作成は controller 担当

**変更:**
- `backend/src/service/container.go` — build/redeploy/delete を tasks INSERT に切り替え
- `docker-compose.yaml` — watcher 追加、builder ポート追加、shared マウント追加
- `config/app.env` — UPLOAD_ENDPOINT 変更

---

## 検証方法

```bash
# 全サービス起動
docker compose up backend builder watcher

# コンテナ作成後タスクが DB に作成されることを確認
psql -c "SELECT task_type, status, payload FROM tasks ORDER BY created_at DESC LIMIT 5;"

# controller が K8s Job を作成
kubectl get jobs -n buildkit

# watcher がリーダー選出されていることを確認
redis-cli GET "watcher:leader"

# watcher がログを Redis に publish
redis-cli SUBSCRIBE "stream:job:buildkit:railpack-{jobID}"

# WebSocket でログ受信
wscat -c ws://localhost:8090/v1/ws/build-jobs/{id}

# builder が deploy タスクを作成
psql -c "SELECT task_type, status FROM tasks WHERE task_type='deploy';"

# K8s Deployment が作成される
kubectl get deployments -n ns-{projectId}

# コンテナ削除時に Harbor イメージ削除タスクが作成される
psql -c "SELECT task_type, status FROM tasks WHERE task_type='delete_image';"

# Redis キャッシュ確認
redis-cli GET "cache:container:{id}"
```
