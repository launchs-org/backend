# ISSUE-014 Apply履歴エンドポイント

## 親 Issue
ISSUE-009

## 概要
デプロイメントのApply履歴一覧を取得するエンドポイントを実装する。

## 変更ファイル一覧

- `app/src/repository/apply_history_repository.go`（編集）
    - **何を**: FindAllByDeploymentIDメソッドの追加。デプロイメントIDに紐づく履歴を時系列順で取得する。
    - **なぜ**: Apply履歴一覧取得のDB操作が必要なため

- `app/src/service/apply.go`（編集）
    - **何を**: ListApplyHistoriesメソッドの追加。リポジトリ経由で履歴一覧を取得する。
    - **なぜ**: サービス層経由での履歴一覧取得が必要なため

- `app/src/handler/deployment_handler.go`（編集）
    - **何を**: ListApplyHistoriesハンドラーの追加。URLパラメータからdeploymentIDを取得して履歴一覧を返す。
    - **なぜ**: Apply履歴一覧取得のHTTPエントリーポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET /api/v1/deployments/:id/apply-historiesエンドポイントの登録。
    - **なぜ**: Apply履歴エンドポイントをルーターに接続するため

## テスト確認項目

- [ ] GET /api/v1/deployments/:id/apply-historiesでApply履歴一覧が取得できること
- [ ] 他ユーザーのDeploymentのGET /apply-historiesすると403が返ること
- [ ] 履歴が新しい順で返ること
- [ ] Apply履歴が存在しない場合は空配列が返ること

### repository 層テスト

- [ ] ApplyHistoryRepository.FindAllByDeploymentIDで履歴一覧が取得できること
