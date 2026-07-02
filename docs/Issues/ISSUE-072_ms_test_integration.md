# ISSUE-072: テスト & 統合

## 親 Issue

ISSUE-067: バックエンドをマイクロサービスに分割する

## 概要

各コンポーネント（handler/controller/watcher/builder）のテストを実装し、`docker-compose.yml` で全コンポーネントを統合して E2E 動作を確認する。

## テスト実装方針

### handler テスト（handler/src/）

- `handler/*_handler_test.go`: service インターフェースをモックし、Echo の `httptest` で HTTP リクエスト/レスポンスを検証
- `service/*_service_test.go`: repository インターフェースをモックし、ビジネスロジックを検証

新規で変更した部分のテストを重点的に実装する:
- `handler/deployment_handler_test.go`: ApplyDeployment が 202 Accepted + workflow_id を返すことを確認
- `service/apply_test.go`: `temporal.StartWorkflow` が正しい引数で呼ばれることを確認（Temporal クライアントをモック）

### controller テスト（controller/src/）

- `workflow/*_workflow_test.go`: `go.temporal.io/sdk/testsuite` の `TestWorkflowEnvironment` を使って Workflow の Activity 連鎖を検証
- `activity/*_activity_test.go`: repository/k8s をモックして Activity 単体を検証

### watcher テスト（watcher/src/）

- `k8s/*_watcher_test.go`: `k8s.io/client-go/kubernetes/fake` を使って K8s イベントに対する DB 更新を検証

### builder テスト（builder/src/）

- `workflow/*_workflow_test.go`: Temporal testsuite で BuildWorkflow / CancelBuildWorkflow を検証
- `activity/*_activity_test.go`: k8s/repository をモックして Activity 単体を検証

## docker-compose.yml 最終構成

```yaml
services:
  handler:
    build: ./handler
    environment:
      - TEMPORAL_HOST=temporal:7233
    depends_on: [db, temporal]

  controller:
    build: ./controller
    environment:
      - TEMPORAL_HOST=temporal:7233
    depends_on: [db, temporal]

  watcher:
    build: ./watcher
    depends_on: [db]

  builder:
    build: ./builder
    environment:
      - TEMPORAL_HOST=temporal:7233
    depends_on: [db, temporal]

  temporal:
    image: temporalio/auto-setup:latest
    ports:
      - "7233:7233"
      - "8088:8088"
    depends_on: [db]

  db:
    image: postgres:16
    # ... 既存設定
```

## テスト確認項目

### handler テスト
- [ ] `docker compose exec handler go test ./...` が全件 PASS する
- [ ] ApplyDeployment が 202 Accepted + workflow_id を返すテストが通る

### controller テスト
- [ ] `docker compose exec controller go test ./...` が全件 PASS する
- [ ] ApplyWorkflow の全 Activity が正しい順序で実行されるテストが通る

### watcher テスト
- [ ] `docker compose exec watcher go test ./...` が全件 PASS する
- [ ] K8s Deployment が Running になると DB の app_status が更新されるテストが通る

### builder テスト
- [ ] `docker compose exec builder go test ./...` が全件 PASS する
- [ ] BuildWorkflow がログ収集後に pending_image_url をセットするテストが通る
- [ ] CancelBuildWorkflow が build workflow をキャンセルするテストが通る

### E2E 確認
- [ ] `docker compose up` で全サービスが起動する
- [ ] Temporal Web UI（http://localhost:8088）にアクセスできる
- [ ] Apply フロー: POST /apply → 202 → Temporal Workflow 完了 → app_status = running
- [ ] Build フロー: POST /build → 201 → ログ取得 → BuildStatus = succeeded → pending_image_url セット
- [ ] キャンセルフロー: DELETE /builds/:id → BuildStatus = cancelled
