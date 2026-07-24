# PaaS 設計書 — API エンドポイント定義

## 共通仕様

- ベース URL: `/api/v1`
- Content-Type: `application/json`
- エラーレスポンス: `{ "error": "メッセージ", "code": "ERROR_CODE" }`
- リクエストは常に `pending_` なしのフィールド名で送る。サーバー側で `pending_***` に書き込む
- `k8s_status` は未同期時 `null` で返す

---

## Projects

### `GET /projects`
```json
[{ "id": "uuid", "name": "my-app", "namespace": "my-app", "status": "active", "k8s_status": null }]
```

### `POST /projects`
```json
// Request
{ "name": "my-app" }
// Response 201
{ "id": "uuid", "name": "my-app", "namespace": "my-app", "status": "provisioning" }
```

### `GET /projects/:id` — 詳細

### `PUT /projects/:id`
```json
{ "name": "new-name" }
```

### `DELETE /projects/:id` — `status → deleting` で削除開始。Response 202

---

## Deployments

### `GET /projects/:id/deployments`
```json
[{
  "id": "uuid", "name": "web", "type": "image_url",
  "status": "running", "app_status": "running",
  "image_url": "nginx:latest", "pending_image_url": "nginx:1.25",
  "instance_size": "small", "pending_instance_size": null,
  "replicas": 2, "pending_replicas": null,
  "current_build_id": "uuid",
  "k8s_status": null, "applied_at": "2024-01-01T00:00:00Z"
}]
```

### `POST /projects/:id/deployments`
全フィールドを `pending_***` に格納。`status = pending`、`app_status = pending` で返る。

**Request (type=image_url)**
```json
{
  "name": "web", "type": "image_url",
  "image_url": "nginx:latest",
  "instance_size": "small", "replicas": 1
}
```

**Request (type=dockerfile)**
```json
{
  "name": "api", "type": "dockerfile",
  "github_repo_url": "https://github.com/org/repo",
  "github_branch": "main",
  "github_commit_sha": "HEAD",
  "dockerfile_path": "./Dockerfile",
  "build_directory": "./",
  "instance_size": "small", "replicas": 1
}
```

**Request (type=railpack)**
```json
{
  "name": "api", "type": "railpack",
  "github_repo_url": "https://github.com/org/repo",
  "github_branch": "main",
  "github_commit_sha": "HEAD",
  "build_directory": "./",
  "instance_size": "small", "replicas": 1
}
```

**Response 201**
```json
{ "id": "uuid", "status": "pending", "app_status": "pending" }
```

### `GET /deployments/:id` — 詳細（`pending_***` 含む）

### `PUT /deployments/:id`
`pending_***` のみ更新。k8s への反映は行わない。

```json
// image_url 変更
{ "image_url": "nginx:1.25" }

// GitHub ブランチ変更
{ "github_branch": "feature/new", "github_commit_sha": "HEAD", "build_directory": "./" }
```

**Response 200** — 更新後の deployment

### `POST /deployments/:id/apply`
`pending_***` を k8s に適用する。

**処理順序:**
1. `SELECT FOR UPDATE` でロック取得
2. env_var 実効キーの重複チェック（重複あり → 400）
3. `github_commit_sha = "HEAD"` の場合、GitHub API で最新 SHA を取得して上書き
4. ビルド要否判定（type が dockerfile/railpack かつ GitHub 情報が変化している場合）
   - building 中の k8s Job をキャンセル（Job 削除）
   - 新しい `deployment_builds` レコードを INSERT
   - k8s Job を作成してビルドを非同期実行
   - `app_status = building`、`current_build_id = ""`（ビルド中を示す）
5. ビルド不要の場合: manifest を生成して k8s apply
6. `apply_history` に manifest スナップショットを INSERT
7. 成功: `pending_***` を空に、current 値に昇格、env_var_mounts.status = applied
8. 失敗: `apply_history.status = failed`、`error_message` 記録。`pending_***` はそのまま
9. ロック解放

**Response 200**
```json
{ "apply_history_id": "uuid", "status": "applied" }
```

**Response 400**
```json
{ "error": "duplicate env key: DATABASE_URL", "code": "DUPLICATE_ENV_KEY" }
```

### `DELETE /deployments/:id` — `status → deleting`。Response 202

---

## Deployment Builds

### `GET /deployments/:id/builds`
```json
[{
  "id": "uuid", "status": "succeeded",
  "built_image_url": "registry.launchs.org/proj/app:sha-abc123",
  "commit_sha": "abc123", "commit_message": "fix: update", "branch": "main",
  "directory": "./", "author": "octocat",
  "started_at": "2024-01-01T00:00:00Z", "finished_at": "2024-01-01T00:05:00Z"
}]
```

### `GET /deployments/:id/builds/:build_id/logs`
**Query:** `since` / `until`（ISO8601）

```json
{ "log": "Step 1/5: FROM node:18\n..." }
```

---

## Apply History

### `GET /deployments/:id/apply-history`
```json
[{ "id": "uuid", "status": "applied", "error_message": null, "applied_at": "2024-01-01T00:00:00Z" }]
```

### `GET /deployments/:id/apply-history/:history_id` — 詳細（manifests 含む）

---

## Pod Logs

### `GET /deployments/:id/logs`
**Query:** `since` / `until`（ISO8601）、`container`（省略時はメインコンテナ）

```json
{ "logs": "2024-01-01T00:00:00Z [INFO] Server started\n..." }
```

---

## Services

### `GET /deployments/:id/service`
```json
{
  "id": "uuid", "deployment_id": "uuid",
  "ports": [{"protocol": "TCP", "port": 8080}],
  "pending_ports": [{"protocol": "TCP", "port": 9090}],
  "status": "active", "k8s_status": null
}
```

### `PUT /deployments/:id/service`
クライアントは `ports` で送る。サーバーが `pending_ports` に書き込む。

```json
{ "ports": [{"protocol": "TCP", "port": 8080}, {"protocol": "UDP", "port": 9090}] }
```

**バリデーション:**
- port は 1–65535
- 同一 port + protocol の重複不可
- ports は最低1つ必要

---

## IngressRoutes

Service 作成とは独立して任意タイミングで作成する。Service と 1:1。作成後変更不可。

### `GET /deployments/:id/ingress`
```json
{
  "id": "uuid", "service_id": "uuid",
  "host": "web-abc12345.launchs.org", "path_prefix": "/",
  "port": 8080, "status": "active", "k8s_status": null
}
```

### `POST /deployments/:id/ingress`
ドメインを払い出し、指定した TCP ポートへ転送する IngressRoute を作成する。

```json
{ "service_id": "uuid", "port": 8080 }
```

**バリデーション:**
- `port` は service.ports の TCP ポートのうちいずれか
- 既に IngressRoute が存在する場合は 409

**Response 201**
```json
{ "id": "uuid", "host": "web-abc12345.launchs.org", "status": "pending" }
```

### `DELETE /deployments/:id/ingress` — `status → deleting`。Response 202

---

## Env Vars

### `GET /projects/:id/env-vars`
`is_secret = true` の場合 `value` は `"***"` でマスク。

```json
[
  { "id": "uuid", "key": "DATABASE_URL", "value": "postgres://...", "is_secret": false, "status": "active" },
  { "id": "uuid", "key": "API_SECRET", "value": "***", "is_secret": true, "status": "active" }
]
```

### `POST /projects/:id/env-vars`
```json
{ "key": "DATABASE_URL", "value": "postgres://localhost:5432/db", "is_secret": false }
```

### `PUT /env-vars/:id`
```json
{ "value": "postgres://new-host:5432/db" }
```

### `DELETE /env-vars/:id`
mount されている deployment がある場合は 409。`status → deleting`。

---

## Env Var Mounts

### `GET /deployments/:id/env-mounts`
```json
[{
  "id": "uuid", "env_var_id": "uuid", "env_var_key": "DATABASE_URL",
  "override_key": null, "pending_override_key": "DB_URL", "status": "applied"
}]
```

### `POST /deployments/:id/env-mounts`
`status = pending`。

```json
{ "env_var_id": "uuid", "override_key": null }
```

**バリデーション:** 同一 deployment に同一 `env_var_id` の重複不可

### `PUT /env-mounts/:id`
クライアントは `override_key` で送る。サーバーが `pending_override_key` に書き込む。

```json
{ "override_key": "DB_URL" }
```

### `DELETE /env-mounts/:id` — `status → deleting`

---

## Volumes

### `GET /projects/:id/volumes`
```json
[{ "id": "uuid", "name": "data", "size_mb": 1024, "status": "bound", "k8s_status": null }]
```

### `POST /projects/:id/volumes`
`status = pending`。quota チェックあり。

```json
{ "name": "data", "size_mb": 1024 }
```

**バリデーション:**
- `size_mb` > 0
- user 全体の volume 合計 MB が `max_volume_mb` を超えないこと

### `DELETE /volumes/:id`
mount されている場合は 409。`status → deleting`。

---

## Volume Mounts

### `GET /deployments/:id/volume-mounts`
```json
[{
  "id": "uuid", "volume_id": "uuid", "volume_name": "data",
  "mount_path": "/data", "pending_mount_path": null, "status": "mounted"
}]
```

### `POST /deployments/:id/volume-mounts`
`status = pending`。

```json
{ "volume_id": "uuid", "mount_path": "/data" }
```

### `PUT /volume-mounts/:id`
クライアントは `mount_path` で送る。サーバーが `pending_mount_path` に書き込む。

```json
{ "mount_path": "/mnt/data" }
```

### `DELETE /volume-mounts/:id` — `status → deleting`

---

## Webhooks

### `GET /deployments/:id/webhook`
```json
{ "id": "uuid", "deployment_id": "uuid", "webhook_url": "https://api.launchs.org/webhooks/{deployment_id}/github" }
```

### `POST /deployments/:id/webhook`
Webhook を作成する。Secret を自動生成して返す（初回のみ表示）。

**Response 201**
```json
{
  "id": "uuid",
  "webhook_url": "https://api.launchs.org/webhooks/{deployment_id}/github",
  "secret": "whsec_xxxxxxxxxxxx"
}
```

### `DELETE /deployments/:id/webhook` — Webhook を削除

### `POST /webhooks/:deployment_id/github`
GitHub からの push イベントを受け取る。

**挙動:**
1. `X-Hub-Signature-256` で HMAC-SHA256 署名を検証
2. push された branch が `deployment.github_branch` と一致するか確認
3. 一致する場合、`pending_github_commit_sha` を push された commit SHA に更新して apply をトリガー
4. 一致しない場合は 200 OK を返してスキップ

---

## User Quotas

`user_id` は JWT から取得する。認証は別サービスが担当し、このサービスには user_id 文字列のみが渡ってくる。

### `GET /users/:user_id/quota`
```json
{
  "user_id": "uuid",
  "max_projects": 5, "max_deployments": 20,
  "max_replicas_per_deployment": 5, "max_volume_mb": 10240,
  "current_projects": 2, "current_deployments": 3, "current_volume_mb": 2048
}
```

### `PUT /users/:user_id/quota`
```json
{ "max_projects": 10, "max_deployments": 30, "max_volume_mb": 20480 }
```
