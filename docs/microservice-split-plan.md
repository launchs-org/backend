# マイクロサービス分割 実行計画書

## 概要

現在のモノリシックな `app/` を4つの独立したコンポーネントに分割し、Temporal をワークフローエンジンとして疎結合に接続する。

**ブランチ:** `feat/microservice-split`

---

## 決定済み設計方針

| 項目 | 方針 |
|------|------|
| Apply レスポンス | **非同期**（202 Accepted + workflow_id）← 現行は同期・200 OK |
| Build レスポンス | 現行通り（201 Created + DeploymentBuild） |
| Apply の DB昇格 | **controller** が担当（pending→current 昇格も Activity） |
| Watcher→DB更新 | Watcher が直接 PostgreSQL を更新 |
| ビルドログ | builder が直接 PostgreSQL（BuildLogChunk）に書く |
| ビルド完了後 Apply | **呼ばない**（フロントが手動で POST /apply） |
| 整合性保証 | deployment_id を Temporal WorkflowID として冪等起動 |
| デプロイ単位 | 4つの独立した Docker イメージ・Pod |

---

## コンポーネント責務

### handler（APIゲートウェイ）
- Echo v4 HTTPサーバー、JWT認証・クォータチェック
- DB への設定変更（pending フィールド更新）は引き続き handler の Service 層が担当
- K8s 操作・ビルドは `temporal.StartWorkflow` を呼んで即座にレスポンスを返す

### controller（K8s操作ワーカー）
- Temporal Worker として動作
- 全K8s操作を Workflow + Activity として実装
- WorkflowID を `"apply-{deploymentID}"` 等に固定して冪等性を保証

### watcher（状態監視）
- 現行の Watch ループをそのまま移植（Temporal Worker ではない）
- PostgreSQL Advisory Lock ベースのリーダーエレクション維持
- K8s イベント検知 → 直接 PostgreSQL 更新

### builder（ビルド管理）
- Temporal Worker として動作
- BuildJob 作成・ログ収集・`pending_image_url` セットを担当
- ビルド完了後に Apply は呼ばない

---

## サービス間の通信フロー

```
Frontend
  ↓
[handler] ─── temporal.StartWorkflow ──► [controller Worker]
  │ (202 Accepted + workflow_id)              ├─ DB pending→current 昇格
  │                                           ├─ K8s Apply
  │                                           └─ ApplyHistory 記録
  │
  │ ─── temporal.StartWorkflow ──► [builder Worker]
  │ (201 Created + DeploymentBuild)           ├─ BuildLogChunk → DB
  │                                           └─ pending_image_url セット
  │
[watcher] ─── K8s Watch Loop ──────────────► DB (直接更新)

[全コンポーネント] ──────────────────────────► PostgreSQL（共有）
```

---

## リポジトリ構造

```
backend-mini/
├── shared/               # 全コンポーネント共通（Go モジュール: app/shared）
│   ├── go.mod
│   ├── models/           # GORM モデル定義
│   ├── dto/              # コンポーネント間共有構造体
│   ├── repository/       # Repository インターフェース + GORM 実装
│   ├── logger/
│   └── config/
│
├── handler/              # 旧 app/ をリネーム
│   └── src/
│       ├── go.mod        # replace app/shared => ../../shared
│       ├── main.go
│       ├── handler/      # HTTPハンドラー層（テストあり）
│       ├── service/      # K8s操作は temporal.StartWorkflow に置換
│       ├── middlewares/
│       └── router/
│
├── controller/           # Temporal Worker（K8s操作）
│   └── src/
│       ├── go.mod
│       ├── main.go
│       ├── workflow/     # Workflow 定義（テストあり）
│       ├── activity/     # Activity 実装（テストあり）
│       └── k8s/          # K8s クライアントラッパー
│
├── watcher/              # Watch ループプロセス
│   └── src/
│       ├── go.mod
│       ├── main.go
│       ├── leader/       # リーダーエレクション
│       └── k8s/          # Watch 実装（テストあり）
│
├── builder/              # Temporal Worker（ビルド管理）
│   └── src/
│       ├── go.mod
│       ├── main.go
│       ├── workflow/     # BuildWorkflow / CancelBuildWorkflow（テストあり）
│       ├── activity/     # Activity 実装（テストあり）
│       └── k8s/          # BuildJob 操作
│
└── docker-compose.yml    # handler / controller / watcher / builder / Temporal / PostgreSQL
```

---

## Temporal ワークフロー一覧

### controller（task queue: `"controller-queue"`）

| Workflow ID | トリガー | Activity の連鎖 |
|---|---|---|
| `apply-{deploymentID}` | `POST /deployments/:id/apply` | (1) pending→current昇格 → (2) Manifest生成 → (3) K8s Deployment適用 → (4) K8s Service適用 → (5) K8s IngressRoute適用 → (6) ApplyHistory記録 → (7) app_status=deploying |
| `delete-deployment-{deploymentID}` | `DELETE /deployments/:id` | (1) DB status=deleting → (2) K8s Deployment削除 → (3) K8s Service削除 → (4) DB レコード削除 |
| `create-project-{projectID}` | `POST /projects` | (1) Harbor Project作成 → (2) Harbor Robot作成 → (3) K8s Namespace作成 → (4) DB status=active |
| `delete-project-{projectID}` | `DELETE /projects/:id` | (1) K8s Namespace削除 → (2) Harbor Project削除 → (3) DB レコード削除 |
| `create-volume-{volumeID}` | `POST /volumes` | (1) K8s PVC作成 → (2) DB ステータス更新 |
| `delete-volume-{volumeID}` | `DELETE /volumes/:id` | (1) K8s PVC削除 → (2) DB レコード削除 |

### builder（task queue: `"builder-queue"`）

| Workflow ID | トリガー | Activity の連鎖 |
|---|---|---|
| `build-{buildID}` | `POST /deployments/:id/build` | (1) Harbor認証情報確認 → (2) BuildJob作成 → (3) ログストリーム+DB書き込み → (4) pending_image_urlセット → (5) BuildStatus更新 |
| `cancel-build-{buildID}` | `DELETE /builds/:id` | build workflow をキャンセル → (1) K8s Job削除 → (2) BuildStatus=cancelled |

### Temporal を使わないもの（handler Service 層で同期処理）

Deployment設定変更・EnvVar/IngressRoute/VolumeMount/Service/Webhook/Template CRUD・Quota操作

---

## Issue 一覧

| Issue | 作業内容 |
|---|---|
| ISSUE-068 | 基盤整備・shared モジュール作成・app→handler リネーム・Temporal docker-compose 追加 |
| ISSUE-069 | watcher コンポーネント作成・Watch ループ移植 |
| ISSUE-070 | controller コンポーネント作成・全 K8s Workflow 実装 |
| ISSUE-071 | builder コンポーネント作成・Build/CancelBuild Workflow 実装 |
| ISSUE-072 | テスト実装 & docker-compose 統合・E2E確認 |
