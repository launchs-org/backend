# ISSUE-067: バックエンドをマイクロサービスに分割する

## 親 Issue

なし（本 Issue がトップレベル）

## 概要

現在のモノリシックな `app/` を以下の4つの独立したコンポーネントに分割し、Temporal をワークフローエンジンとしてコンポーネント間を疎結合につなぐ。

| コンポーネント | 役割 |
|---|---|
| **handler** | HTTP APIゲートウェイ（旧 `app/`） |
| **controller** | K8s操作ワーカー（Temporal Worker） |
| **watcher** | K8s状態監視プロセス |
| **builder** | ビルド管理ワーカー（Temporal Worker） |

全コンポーネントは同一の PostgreSQL を参照する。共通コード（models, repository, dto, logger, config）は `shared/` モジュールに切り出す。

## ブランチ

`feat/microservice-split`

## 子 Issue 一覧

- ISSUE-068: 基盤整備・shared モジュール作成
- ISSUE-069: watcher コンポーネント作成
- ISSUE-070: controller コンポーネント作成
- ISSUE-071: builder コンポーネント作成
- ISSUE-072: テスト & 統合

## アーキテクチャ概要

```
Frontend
  ↓
[handler] ─── Temporal.StartWorkflow ──► [controller Worker]
  │ (202 Accepted / 201 Created)              ├─ DB pending→current 昇格
  │                                           ├─ K8s Apply
  │                                           └─ ApplyHistory 記録
  │
  │ ─── Temporal.StartWorkflow ──► [builder Worker]
  │                                           ├─ BuildLogChunk → DB
  │                                           └─ pending_image_url セット
  │
[watcher] ─── K8s Watch Loop ──────────────► DB (直接更新)

[全コンポーネント] ──────────────────────────► PostgreSQL（共有）
```

## Temporal ワークフロー一覧

### controller が担当

| Workflow ID | トリガー |
|---|---|
| `apply-{deploymentID}` | `POST /deployments/:id/apply` |
| `delete-deployment-{deploymentID}` | `DELETE /deployments/:id` |
| `create-project-{projectID}` | `POST /projects` |
| `delete-project-{projectID}` | `DELETE /projects/:id` |
| `create-volume-{volumeID}` | `POST /volumes` |
| `delete-volume-{volumeID}` | `DELETE /volumes/:id` |

### builder が担当

| Workflow ID | トリガー |
|---|---|
| `build-{buildID}` | `POST /deployments/:id/build` |
| `cancel-build-{buildID}` | `DELETE /builds/:id` |
