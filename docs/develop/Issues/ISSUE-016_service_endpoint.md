# ISSUE-016 ServiceエンドポイントCRUD

## 親 Issue
ISSUE-015

## 概要
k8s Serviceのポート設定を管理するエンドポイントを実装する。Serviceレコードはデプロイメント作成時に生成済みのため、更新・取得のみを担当する。

## 変更ファイル一覧

- `app/src/models/service.go`（編集）
    - **何を**: Serviceモデルの定義。port・target_port・type（ClusterIP/NodePort/LoadBalancer）フィールドを持つ。pending_*パターンを適用し、pending_port・pending_target_portを保持する。
    - **なぜ**: k8s Serviceの設定をDBで管理し、applyまでの変更をステージングするため

- `app/src/repository/deployment_repository.go`（編集）
    - **何を**: ServiceRepositoryインターフェースにFindByDeploymentID・Updateメソッドを追加。
    - **なぜ**: ServiceのDB操作を抽象化するため

- `app/src/service/deployment_service.go`（編集）
    - **何を**: GetService・UpdateServiceメソッドをDeploymentServiceに追加。更新はpending_*フィールドへの書き込みのみ。GetService・UpdateServiceではDeploymentのProjectIDからProjectを取得してUserIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: Service設定のビジネスロジックをハンドラーから分離するため。また、他ユーザーのデプロイメントへの不正アクセスを防ぐため

- `app/src/handler/deployment_handler.go`（編集）
    - **何を**: GetServiceとUpdateServiceハンドラーの追加。
    - **なぜ**: Service設定の取得・更新エンドポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/PUT /api/v1/deployments/:id/serviceエンドポイントの登録。
    - **なぜ**: Serviceエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] GET /api/v1/deployments/:id/serviceでService設定が取得できること
- [ ] PUT /api/v1/deployments/:id/serviceでpending_*フィールドが更新されること
- [ ] apply後にpending値が実際の値に昇格されること
- [ ] 他ユーザーのDeploymentのServiceをGETすると403が返ること
- [ ] 他ユーザーのDeploymentのServiceをPUTすると403が返ること

### repository 層テスト

- [ ] ServiceRepository.FindByDeploymentIDでService設定が取得できること
- [ ] ServiceRepository.UpdateでService設定が更新できること
