# ISSUE-022 環境変数CRUD

## 親 Issue
ISSUE-021

## 概要
環境変数のCRUDエンドポイントを実装する。環境変数はプロジェクトスコープで管理し、デプロイメントへのマウントは別途行う。

## 変更ファイル一覧

- `app/src/models/env_var.go`（編集）
    - **何を**: EnvVarモデルの定義。ProjectIDへの外部キー、key・value・is_secretフィールドを持つ。is_secretがtrueの場合はk8s Secretに格納される。
    - **なぜ**: 環境変数エンティティのDB表現を定義するため

- `app/src/repository/env_var_repository.go`（新規作成）
    - **何を**: EnvVarRepositoryインターフェースと実装。Create・FindByID・FindAllByProjectID・Update・Deleteメソッドを持つ。
    - **なぜ**: 環境変数のDB操作を抽象化するため

- `app/src/service/env_var_service.go`（新規作成）
    - **何を**: EnvVarServiceインターフェースと実装。ListEnvVars・CreateEnvVar・UpdateEnvVar・DeleteEnvVarのCRUD。すべての操作でProjectのUserIDとリクエストユーザーIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: 環境変数管理のビジネスロジックをハンドラーから分離するため。また、他ユーザーのプロジェクトリソースへの不正アクセスを防ぐため

- `app/src/handler/env_var_handler.go`（新規作成）
    - **何を**: ListEnvVars・CreateEnvVar・UpdateEnvVar・DeleteEnvVarハンドラーの実装。
    - **なぜ**: 環境変数CRUDのHTTPエントリーポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/POST /api/v1/projects/:id/env-vars、GET/PUT/DELETE /api/v1/env-vars/:idエンドポイントの登録。
    - **なぜ**: 環境変数CRUDエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/projects/:id/env-varsで環境変数が作成できること
- [ ] GET /api/v1/projects/:id/env-varsで環境変数一覧が取得できること
- [ ] PUT /api/v1/env-vars/:idで環境変数が更新できること
- [ ] DELETE /api/v1/env-vars/:idで環境変数が削除できること
- [ ] is_secret=trueの環境変数の値がレスポンスでマスクされること
- [ ] 他ユーザーのProjectにPOST /env-varsすると403が返ること
- [ ] 他ユーザーのProjectのGET /env-varsすると403が返ること
- [ ] 他ユーザーのProjectの環境変数をPUTすると403が返ること
- [ ] 他ユーザーのProjectの環境変数をDELETEすると403が返ること

### repository 層テスト

- [ ] EnvVarRepository.Createで環境変数が作成できること
- [ ] EnvVarRepository.FindAllByProjectIDでプロジェクトの環境変数一覧が取得できること
