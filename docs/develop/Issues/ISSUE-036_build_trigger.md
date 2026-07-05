# ISSUE-036 ビルドトリガーエンドポイント

## 親 Issue
ISSUE-035

## 概要
デプロイメントのビルドを開始するエンドポイントを実装する。DeploymentBuildレコードを作成してk8s Jobをトリガーする。

## 変更ファイル一覧

- `app/src/models/deployment_build.go`（編集）
    - **何を**: DeploymentBuildモデルの定義。DeploymentIDへの外部キー、build_type（dockerfile/railpack）、status（pending/building/succeeded/failed/canceled）、github_repo_url・github_branch・commit_shaフィールドを持つ。
    - **なぜ**: ビルドの実行状態と結果をDBで管理するため
- `app/src/repository/deployment_build_repository.go`（新規作成）
    - **何を**: DeploymentBuildRepositoryインターフェースと実装。Create・FindByID・FindAllByDeploymentID・UpdateStatusメソッドを持つ。
    - **なぜ**: ビルドレコードのDB操作を抽象化するため
- `app/src/service/build_service.go`（新規作成）
    - **何を**: BuildServiceインターフェースと実装。TriggerBuildメソッドの実装。DeploymentBuildレコードをpendingで作成後にk8s Jobを起動する。ビルドタイプに応じてdockerfile/railpackのJobを生成する。TriggerBuildではDeploymentのProjectIDからProjectを取得してUserIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: ビルドトリガーのビジネスロジックをハンドラーから分離するため。また、他ユーザーのデプロイメントへの不正アクセスを防ぐため
- `app/src/handler/build_handler.go`（新規作成）
    - **何を**: TriggerBuildハンドラーの実装。
    - **なぜ**: ビルドトリガーのHTTPエントリーポイントが必要なため
- `app/src/router/router.go`（編集）
    - **何を**: POST /api/v1/deployments/:id/buildエンドポイントの登録。
    - **なぜ**: ビルドトリガーエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/deployments/:id/buildでビルドがトリガーされること
- [ ] DeploymentBuildレコードがpendingで作成されること
- [ ] k8s Jobが作成されること
- [ ] ビルド中のDeploymentに再ビルドをトリガーすると409が返ること
- [ ] 他ユーザーのDeploymentにPOST /buildすると403が返ること
### repository 層テスト

- [ ] DeploymentBuildRepository.Createでビルドレコードが作成できること
- [ ] DeploymentBuildRepository.UpdateStatusでstatusが更新できること
