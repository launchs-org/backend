# ISSUE-071: builder コンポーネント作成

## 親 Issue

ISSUE-067: バックエンドをマイクロサービスに分割する

## 概要

ビルド管理を担う Temporal Worker コンポーネント `builder/` を作成する。
現行 `handler/src/service/build_service.go` のビルドトリガーロジックと `handler/src/k8s/build.go` の WatchBuildJobs を Temporal Workflow + Activity として移植する。

ビルド完了後に自動で Apply は**呼ばない**（現行通り、フロントエンドが手動で Apply を叩く）。

## 実装手順

### 1. builder/ ディレクトリ構造

```
builder/
└── src/
    ├── go.mod          # module builder; require app/shared, go.temporal.io/sdk
    ├── main.go         # Temporal Worker 起動
    ├── workflow/
    │   ├── build_workflow.go
    │   ├── build_workflow_test.go
    │   ├── cancel_build_workflow.go
    │   └── cancel_build_workflow_test.go
    └── activity/
        ├── build_activity.go       # Harbor確認・BuildJob作成・ログ収集・pending_image_urlセット
        ├── build_activity_test.go
        ├── cancel_build_activity.go
        └── cancel_build_activity_test.go
```

### 2. Workflow 定義

**BuildWorkflow** (`build-{buildID}`):
1. `Activity: VerifyHarborCredentialActivity` — Harbor 認証情報確認
2. `Activity: CreateBuildJobActivity` — BuildJob（K8s Job）作成
3. `Activity: StreamBuildLogsActivity` — Job ログストリーム → BuildLogChunk を PostgreSQL に書く（Heartbeat付き）
4. `Activity: SetPendingImageURLActivity` — ビルド成功時: `BuiltImageURL` 組み立て・`pending_image_url` セット・`pending_github_*` 更新（DB）
5. `Activity: UpdateBuildStatusActivity` — `BuildStatus = succeeded/failed`・`Deployment.Status = pending`（DB）

**CancelBuildWorkflow** (`cancel-build-{buildID}`):
1. 実行中の `build-{buildID}` workflow を CancelWorkflow でキャンセル
2. `Activity: DeleteBuildJobActivity` — K8s Job 削除（Job 名が設定されている場合のみ）
3. `Activity: SetBuildCancelledActivity` — `BuildStatus = cancelled`（DB）

### 3. handler 側の変更

`handler/src/service/build_service.go`:
- `TriggerBuild()` のビルドジョブ起動ロジック（`k8s.CreateBuildJob` 以降）を削除
- DeploymentBuild レコードを作成して `temporal.StartWorkflow("build-{buildID}", ...)` を呼ぶだけに変更
- レスポンスは現行通り `201 Created + DeploymentBuild` を維持

- `CancelBuild()` の K8s Job 削除ロジックを削除
- `temporal.StartWorkflow("cancel-build-{buildID}", ...)` 呼び出しに置換

## 変更予定ファイル

- `builder/src/`（新規作成）
  - 何を: ビルド管理を担う Temporal Worker
  - なぜ: handler からビルド実行責務を分離するため

- `handler/src/service/build_service.go`（編集）
  - 何を: `TriggerBuild` のジョブ起動ロジックを `temporal.StartWorkflow` に置換。`CancelBuild` の K8s 操作を `temporal.StartWorkflow` に置換
  - なぜ: ビルド実行を builder に移譲するため

- `docker-compose.yml`（編集）
  - 何を: `builder` サービスを追加
  - なぜ: 独立コンテナとして起動するため

## テスト確認項目

- [ ] `docker compose exec builder go build ./...` でビルドが通る
- [ ] `POST /api/v1/deployments/:id/build` → 201 Created + DeploymentBuild が返る
- [ ] Temporal Web UI で BuildWorkflow が起動し、ログが収集される
- [ ] `GET /api/v1/builds/:id/logs` でビルドログが取得できる
- [ ] ビルド完了後 `DeploymentBuild.Status = succeeded`、`Deployment.pending_image_url` がセットされる
- [ ] `DELETE /api/v1/builds/:id` でビルドがキャンセルされる（`BuildStatus = cancelled`）
- [ ] キャンセル完了後に次のビルドが起動できる
