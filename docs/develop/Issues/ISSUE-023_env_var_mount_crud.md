# ISSUE-023 環境変数マウントCRUD

## 親 Issue
ISSUE-021

## 概要
環境変数をデプロイメントにマウントする設定のCRUDエンドポイントを実装する。マウント設定はapply時にk8sのcontainer.envまたはenvFromとして反映される。

## 変更ファイル一覧

- `app/src/models/env_var_mount.go`（編集）
    - **何を**: EnvVarMountモデルの定義。DeploymentIDとEnvVarIDへの外部キー、mount_keyフィールド（k8s側の環境変数名）を持つ。
    - **なぜ**: 環境変数とデプロイメントの多対多関係をDBで管理するため

- `app/src/repository/env_var_mount_repository.go`（新規作成）
    - **何を**: EnvVarMountRepositoryインターフェースと実装。Create・FindAllByDeploymentID・Deleteメソッドを持つ。
    - **なぜ**: マウント設定のDB操作を抽象化するため

- `app/src/service/env_var_service.go`（編集）
    - **何を**: ListEnvVarMounts・CreateEnvVarMount・DeleteEnvVarMountメソッドの追加。同一DeploymentIDで同一EnvVarIDの重複マウントを拒否する。すべての操作でDeploymentのProjectIDからProjectを取得してUserIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: 環境変数マウント管理のビジネスロジックをハンドラーから分離するため。また、他ユーザーのデプロイメントへの不正アクセスを防ぐため

- `app/src/handler/env_var_handler.go`（編集）
    - **何を**: ListEnvVarMounts・CreateEnvVarMount・DeleteEnvVarMountハンドラーの追加。
    - **なぜ**: マウント設定管理のHTTPエントリーポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/POST /api/v1/deployments/:id/env-var-mounts、DELETE /api/v1/env-var-mounts/:idエンドポイントの登録。
    - **なぜ**: マウント設定エンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/deployments/:id/env-var-mountsでマウント設定が作成できること
- [ ] GET /api/v1/deployments/:id/env-var-mountsでマウント設定一覧が取得できること
- [ ] DELETE /api/v1/env-var-mounts/:idでマウント設定が削除できること
- [ ] 同一DeploymentIDで同一EnvVarIDの重複マウントが拒否されること
- [ ] 他ユーザーのDeploymentにPOST /env-var-mountsすると403が返ること
- [ ] 他ユーザーのDeploymentのGET /env-var-mountsすると403が返ること
- [ ] 他ユーザーのDeploymentのマウント設定をDELETEすると403が返ること

### repository 層テスト

- [ ] EnvVarMountRepository.FindAllByDeploymentIDでマウント設定一覧が取得できること
