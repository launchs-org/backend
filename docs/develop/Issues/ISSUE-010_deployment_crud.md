# ISSUE-010 デプロイメントCRUD

## 親 Issue
ISSUE-009

## 概要
デプロイメントのCRUDエンドポイントを実装する。更新はpending_*フィールドへの書き込みのみで、実際のk8s反映はapply時に行う。作成時にServiceレコードも同時生成する。

## 変更ファイル一覧

- `app/src/models/deployment.go`（編集）
    - **何を**: Deploymentモデルの定義。Status定数（pending/running/failed/deleting）、AppStatus定数（pending/building/deploying/running/error）、デプロイタイプ定数（image_url/dockerfile/railpack）。現在値フィールドとpending_*フィールドの両方を持つ。
    - **なぜ**: applyまでステージングされた変更を保持するpending_*パターンを実現するため

- `app/src/models/service.go`（編集）
    - **何を**: Serviceモデルの定義。deploymentと1対1で紐づく。port・target_portフィールドを持つ。
    - **なぜ**: k8s Serviceの設定をDBで管理するため

- `app/src/repository/deployment_repository.go`（編集）
    - **何を**: DeploymentRepositoryとServiceRepositoryのインターフェースと実装。DeploymentRepositoryはCreate・FindByID・FindByIDForUpdate（SELECT FOR UPDATE）・FindAllByProjectID・Save・Updatesメソッドを持つ。
    - **なぜ**: デプロイメントのDB操作を抽象化し、apply時のロック取得を可能にするため

- `app/src/service/deployment_service.go`（編集）
    - **何を**: DeploymentServiceインターフェースと実装。ListDeployments・CreateDeployment（Serviceレコードも同時生成）・GetDeployment・UpdateDeployment（pending_*フィールドのみ更新）・DeleteDeployment（statusをdeletingに変更）。
    - **なぜ**: デプロイメントのビジネスロジックをハンドラーから分離するため

- `app/src/handler/deployment_handler.go`（編集）
    - **何を**: ListDeployments・CreateDeployment・GetDeployment・UpdateDeployment・DeleteDeploymentハンドラーの実装。
    - **なぜ**: HTTPリクエストの受け取りとレスポンス返却を担う層が必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/POST /api/v1/projects/:id/deployments、GET/PUT/DELETE /api/v1/deployments/:idエンドポイントの登録。
    - **なぜ**: デプロイメントCRUDエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/projects/:id/deploymentsでデプロイメントが作成されること
- [ ] 作成時にServiceレコードも同時生成されること
- [ ] GET /api/v1/deployments/:idでデプロイメントが取得できること
- [ ] PUT /api/v1/deployments/:idでpending_*フィールドが更新されること
- [ ] DELETE /api/v1/deployments/:idでstatusがdeletingになること
- [ ] 他ユーザーのDeploymentをGETすると403が返ること
- [ ] 他ユーザーのDeploymentをPUTすると403が返ること
- [ ] 他ユーザーのDeploymentをDELETEすると403が返ること

### repository 層テスト

- [ ] DeploymentRepository.Createでデプロイメントが作成できること
- [ ] DeploymentRepository.FindByIDでデプロイメントが取得できること
- [ ] DeploymentRepository.FindByIDForUpdateでSELECT FOR UPDATEが発行されること
- [ ] DeploymentRepository.Updatesでpendingフィールドのみが更新されること
