# ISSUE-013 Applyエンドポイント

## 親 Issue
ISSUE-009

## 概要
POST /api/v1/deployments/:id/apply エンドポイントを実装する。ApplyServiceを呼び出してk8s同期を実行し、ApplyHistoryを返す。

## 変更ファイル一覧

- `app/src/handler/deployment_handler.go`（編集）
    - **何を**: ApplyDeploymentハンドラーの追加。URLパラメータからdeploymentIDを取得してApplyServiceを呼び出し、作成されたApplyHistoryをレスポンスとして返す。
    - **なぜ**: Applyオペレーションのエントリーポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: POST /api/v1/deployments/:id/applyエンドポイントの登録。
    - **なぜ**: Applyエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/deployments/:id/applyでApplyHistoryが返ること
- [ ] apply中のDeploymentに再applyすると409が返ること
- [ ] 存在しないdeploymentIDで404が返ること
- [ ] 他ユーザーのDeploymentにPOST /applyすると403が返ること
- [ ] k8s apply失敗時に500が返ること
