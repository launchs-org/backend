# ISSUE-048 Podログ取得エンドポイント

## 親 Issue
ISSUE-047

## 概要
running状態のDeploymentのPodログを取得するエンドポイントを実装する。app=deployment_nameラベルでPodを特定し、sinceパラメータで時間フィルタリングをサポートする。

## 変更ファイル一覧

- `app/src/handler/log_handler.go`（新規作成）
    - **何を**: GetPodLogsハンドラーの実装。URLパラメータからdeploymentIDを取得してDeploymentとProjectを解決する。k8s CoreV1 Pods APIでapp=deployment_nameラベルのPodを検索してログを取得する。sinceクエリパラメータ（RFC3339形式）でSinceTimeを設定する。containerクエリパラメータで対象コンテナを指定できる。DeploymentのProjectIDからProjectを取得してUserIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: ユーザーがDeploymentの実行ログを確認するためのエンドポイントが必要なため。また、他ユーザーのDeploymentログへの不正アクセスを防ぐため

- `app/src/router/router.go`（編集）
    - **何を**: GET /api/v1/deployments/:id/logsエンドポイントの登録。
    - **なぜ**: Podログエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] running状態のDeploymentのPodログが取得できること
- [ ] sinceパラメータでログがフィルタされること
- [ ] containerパラメータで特定コンテナのログが取得できること
- [ ] Podが存在しない場合に空文字列が返ること
- [ ] 他ユーザーのDeploymentのGET /logsすると403が返ること

### repository 層テスト

- [ ] DeploymentRepository.FindByIDでstatus=runningのレコードが取得できること
