# ISSUE-070: controller コンポーネント作成

## 親 Issue

ISSUE-067: バックエンドをマイクロサービスに分割する

## 概要

K8s操作を担う Temporal Worker コンポーネント `controller/` を作成する。
現行 `handler/src/service/apply.go` などに実装されている K8s 適用ロジックを Temporal Workflow + Activity として移植し、handler 側は `Temporal.StartWorkflow` 呼び出しに置換する。

Apply 操作は **非同期化**（現行: 同期・200 OK → 変更後: 202 Accepted + workflow_id）。

## 実装手順

### 1. controller/ ディレクトリ構造

```
controller/
└── src/
    ├── go.mod          # module controller; require app/shared, go.temporal.io/sdk
    ├── main.go         # Temporal Worker 起動
    ├── workflow/
    │   ├── apply_workflow.go
    │   ├── apply_workflow_test.go
    │   ├── delete_deployment_workflow.go
    │   ├── create_project_workflow.go
    │   ├── delete_project_workflow.go
    │   ├── create_volume_workflow.go
    │   └── delete_volume_workflow.go
    └── activity/
        ├── apply_activity.go       # pending昇格・Manifest生成・K8s Apply・ApplyHistory記録
        ├── apply_activity_test.go
        ├── project_activity.go     # Harbor・Namespace 操作
        ├── volume_activity.go      # PVC 操作
        └── deployment_activity.go  # Deployment 削除
```

### 2. Workflow 定義（各 Workflow の Activity 連鎖）

**ApplyWorkflow** (`apply-{deploymentID}`):
1. `Activity: PendingToCurrentActivity` — pending→current 昇格（DB, SELECT FOR UPDATE）
2. `Activity: GenerateManifestActivity` — Manifest 生成
3. `Activity: ApplyK8sDeploymentActivity` — K8s Deployment 適用
4. `Activity: ApplyK8sServiceActivity` — K8s Service 適用
5. `Activity: ApplyK8sIngressRouteActivity` — K8s IngressRoute 適用
6. `Activity: RecordApplyHistoryActivity` — ApplyHistory 記録（DB）
7. `Activity: UpdateAppStatusActivity` — app_status = deploying（DB）

**DeleteDeploymentWorkflow** (`delete-deployment-{deploymentID}`):
1. `Activity: SetDeploymentDeletingActivity` — DB ステータス = deleting
2. `Activity: DeleteK8sDeploymentActivity` — K8s Deployment 削除
3. `Activity: DeleteK8sServiceActivity` — K8s Service 削除
4. `Activity: DeleteDeploymentRecordActivity` — DB レコード削除

**CreateProjectWorkflow** (`create-project-{projectID}`):
1. `Activity: CreateHarborProjectActivity` — Harbor プロジェクト作成
2. `Activity: CreateHarborRobotActivity` — Harbor Robot 作成
3. `Activity: CreateK8sNamespaceActivity` — K8s Namespace 作成
4. `Activity: ActivateProjectActivity` — DB ステータス = active

**DeleteProjectWorkflow** (`delete-project-{projectID}`):
1. `Activity: DeleteK8sNamespaceActivity` — K8s Namespace 削除（PVC/Deployment 含む）
2. `Activity: DeleteHarborProjectActivity` — Harbor プロジェクト削除
3. `Activity: DeleteProjectRecordActivity` — DB レコード削除

**CreateVolumeWorkflow** (`create-volume-{volumeID}`):
1. `Activity: CreateK8sPVCActivity` — K8s PVC 作成
2. `Activity: UpdateVolumeStatusActivity` — DB ステータス更新

**DeleteVolumeWorkflow** (`delete-volume-{volumeID}`):
1. `Activity: DeleteK8sPVCActivity` — K8s PVC 削除
2. `Activity: DeleteVolumeRecordActivity` — DB レコード削除

### 3. handler 側の変更

`handler/src/service/apply.go`:
- `Apply()` メソッド内の K8s 適用ロジックを削除
- `temporal.StartWorkflow("apply-{deploymentID}", ...)` 呼び出しに置換
- 戻り値を `ApplyResult` → `{workflow_id: string}` に変更

`handler/src/handler/deployment_handler.go`:
- `ApplyDeployment` のレスポンスを `200 OK` → `202 Accepted + {workflow_id}` に変更

`handler/src/service/project_service.go`:
- `CreateProject()` の K8s/Harbor 操作ロジックを `temporal.StartWorkflow` に置換

`handler/src/service/deployment_service.go`:
- `DeleteDeployment()` の K8s 削除ロジックを `temporal.StartWorkflow` に置換

`handler/src/service/volume_service.go`:
- `CreateVolume()` / `DeleteVolume()` の K8s PVC 操作を `temporal.StartWorkflow` に置換

## 変更予定ファイル

- `controller/src/`（新規作成）
  - 何を: K8s操作を担う Temporal Worker
  - なぜ: handler から K8s 操作責務を分離するため

- `handler/src/service/apply.go`（編集）
  - 何を: K8s 適用ロジックを `temporal.StartWorkflow` に置換、レスポンス形式変更
  - なぜ: Apply の実体を controller に移譲するため

- `handler/src/handler/deployment_handler.go`（編集）
  - 何を: `ApplyDeployment` を `202 Accepted` に変更
  - なぜ: 非同期化に伴うレスポンス形式変更のため

- `handler/src/service/project_service.go`（編集）
  - 何を: Harbor/Namespace 操作を `temporal.StartWorkflow` に置換
  - なぜ: K8s 操作を controller に移譲するため

- `handler/src/service/deployment_service.go`（編集）
  - 何を: DeleteDeployment の K8s 削除を `temporal.StartWorkflow` に置換
  - なぜ: K8s 操作を controller に移譲するため

- `handler/src/service/volume_service.go`（編集）
  - 何を: PVC 操作を `temporal.StartWorkflow` に置換
  - なぜ: K8s 操作を controller に移譲するため

- `docker-compose.yml`（編集）
  - 何を: `controller` サービスを追加
  - なぜ: 独立コンテナとして起動するため

## テスト確認項目

- [ ] `docker compose exec controller go build ./...` でビルドが通る
- [ ] `POST /api/v1/deployments/:id/apply` → 202 Accepted + workflow_id が返る
- [ ] Temporal Web UI でワークフローが完了ステータスになる
- [ ] `GET /api/v1/deployments/:id` の `app_status` が `running` になる
- [ ] `POST /api/v1/projects` → controller が Harbor + Namespace を作成する
- [ ] `DELETE /api/v1/projects/:id` → controller が K8s Namespace と Harbor を削除する
- [ ] `POST /api/v1/volumes` → controller が PVC を作成する
- [ ] 同一 deployment への二重 Apply → Temporal が WorkflowID 重複で弾く（409相当）
