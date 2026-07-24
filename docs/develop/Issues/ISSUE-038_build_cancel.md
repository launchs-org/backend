# ISSUE-038 ビルドキャンセル

## 親 Issue
ISSUE-035

## 概要
実行中のビルドをキャンセルするエンドポイントを実装する。k8s Jobを削除してDeploymentBuildのstatusをcanceledに更新する。

## 変更ファイル一覧

- `app/src/service/build_service.go`（編集）
    - **何を**: CancelBuildメソッドの追加。k8s Jobを削除してDeploymentBuild.statusをcanceledに更新する。already completed/failedのビルドに対してはエラーを返す。CancelBuildではDeploymentBuildからDeploymentを取得しProjectIDからProjectを解決してUserIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: ビルドキャンセルのビジネスロジックをハンドラーから分離するため。また、他ユーザーのビルドへの不正アクセスを防ぐため
- `app/src/handler/build_handler.go`（編集）
    - **何を**: CancelBuildハンドラーの追加。
    - **なぜ**: ビルドキャンセルのHTTPエントリーポイントが必要なため
- `app/src/router/router.go`（編集）
    - **何を**: DELETE /api/v1/builds/:idエンドポイントの登録。
    - **なぜ**: ビルドキャンセルエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] DELETE /api/v1/builds/:idでビルドがキャンセルされること
- [ ] キャンセル後にk8s Jobが削除されること
- [ ] キャンセル後にDeploymentBuild.statusがcanceledになること
- [ ] 完了済みビルドのキャンセルで409が返ること
- [ ] 他ユーザーのビルドをDELETEすると403が返ること
