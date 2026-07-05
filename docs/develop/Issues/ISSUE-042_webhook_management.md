# ISSUE-042 Webhook管理エンドポイント

## 親 Issue
ISSUE-041

## 概要
GitHub Webhookの登録・取得・削除エンドポイントを実装する。WebhookシークレットをDBに保存して受信時の署名検証に使用する。

## 変更ファイル一覧

- `app/src/models/deployment_webhook.go`（編集）
    - **何を**: DeploymentWebhookモデルの定義。DeploymentIDへの外部キー、secret（署名検証用）、github_repo_url、is_activeフィールドを持つ。
    - **なぜ**: Webhook設定とシークレットをDBで安全に管理するため
- `app/src/repository/webhook_repository.go`（新規作成）
    - **何を**: WebhookRepositoryインターフェースと実装。Create・FindByDeploymentID・Deleteメソッドを持つ。
    - **なぜ**: Webhook設定のDB操作を抽象化するため
- `app/src/service/webhook_service.go`（新規作成）
    - **何を**: WebhookServiceインターフェースと実装。CreateWebhook（シークレット自動生成）・GetWebhook・DeleteWebhookのCRUD。すべての操作でDeploymentのProjectIDからProjectを取得してUserIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: Webhook管理のビジネスロジックをハンドラーから分離するため。また、他ユーザーのデプロイメントへの不正アクセスを防ぐため
- `app/src/handler/webhook_handler.go`（新規作成）
    - **何を**: CreateWebhook・GetWebhook・DeleteWebhookハンドラーの実装。
    - **なぜ**: Webhook管理のHTTPエントリーポイントが必要なため
- `app/src/router/router.go`（編集）
    - **何を**: GET/POST /api/v1/deployments/:id/webhooks、DELETE /api/v1/webhooks/:idエンドポイントの登録。
    - **なぜ**: Webhook管理エンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/deployments/:id/webhooksでWebhookが作成されること
- [ ] 作成時にシークレットが自動生成されること
- [ ] GET /api/v1/deployments/:id/webhooksでWebhook設定が取得できること
- [ ] DELETE /api/v1/webhooks/:idでWebhookが削除されること
- [ ] 他ユーザーのDeploymentにPOST /webhooksすると403が返ること
- [ ] 他ユーザーのDeploymentのGET /webhooksすると403が返ること
- [ ] 他ユーザーのWebhookをDELETEすると403が返ること
### repository 層テスト

- [ ] WebhookRepository.FindByDeploymentIDでWebhook設定が取得できること
