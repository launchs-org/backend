# ISSUE-043 GitHub Webhookレシーバー

## 親 Issue
ISSUE-041

## 概要
GitHub pushイベントを受信してApplyをトリガーするエンドポイントを実装する。HMAC-SHA256署名検証を行い、branchが一致した場合のみapplyを実行する。認証不要のエンドポイント（/api/v1グループ外）として登録する。

## 変更ファイル一覧

- `app/src/service/webhook_service.go`（編集）
    - **何を**: ReceiveGithubWebhookメソッドの追加。①X-Hub-Signature-256ヘッダーでHMAC-SHA256署名を検証、②deploymentのgithub_branchとpushのrefが一致するか確認、③一致する場合にDeploymentのpending_github_commit_shaをpushのSHAに更新してApplyServiceを呼び出す。
    - **なぜ**: セキュアなWebhookイベント処理をサービス層で実装するため
- `app/src/handler/webhook_handler.go`（編集）
    - **何を**: ReceiveGithubWebhookハンドラーの追加。リクエストボディとシグネチャヘッダーをサービスに渡す。
    - **なぜ**: WebhookレシーバーのHTTPエントリーポイントが必要なため
- `app/src/router/router.go`（編集）
    - **何を**: POST /webhooks/:deployment_id/githubエンドポイントの登録。RequireAuth適用なし（/api/v1グループ外）。
    - **なぜ**: GitHubからのWebhookは認証ヘッダーを持たないため認証ミドルウェアを除外する必要があるため

## テスト確認項目

- [ ] 正しいHMAC署名でapplyがトリガーされること
- [ ] 不正なHMAC署名で401が返ること
- [ ] branchが一致しないpushではapplyがトリガーされないこと
- [ ] pushのcommit SHAがpending_github_commit_shaに設定されること
### repository 層テスト

- [ ] WebhookRepository.FindByDeploymentIDでシークレットが取得できること
- [ ] DeploymentRepository.Updatesでpending_github_commit_shaが更新できること
