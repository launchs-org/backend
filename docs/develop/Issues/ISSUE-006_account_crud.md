# ISSUE-006 アカウントCRUD

## 親 Issue
ISSUE-005

## 概要
ユーザーのクォータ情報を管理するUserQuotaモデルとCRUDエンドポイントを実装する。ユーザー登録時にデフォルトクォータレコードを作成する。

## 変更ファイル一覧

- `app/src/models/user_quota.go`（編集）
    - **何を**: UserQuotaモデルの定義。max_projects・max_deployments・max_replicas_per_deployment・max_volume_mbフィールドを持つ。
    - **なぜ**: ユーザーごとのリソース上限をDBで管理するため
- `app/src/repository/user_quota_repository.go`（編集）
    - **何を**: UserQuotaRepositoryインターフェースと実装。FindByUserID・Create・Updateメソッドを持つ。
    - **なぜ**: クォータ情報のDB操作を抽象化するため
- `app/src/handler/user_quota_handler.go`（編集）
    - **何を**: GetUserQuotaとUpdateUserQuotaハンドラーの実装。JWTクレームからuserIDを取得してクォータを操作する。
    - **なぜ**: ユーザーが自身のクォータを確認・更新できるエンドポイントが必要なため
- `app/src/router/router.go`（編集）
    - **何を**: GET/PUT /api/v1/users/quotaエンドポイントの登録。
    - **なぜ**: クォータ管理エンドポイントをルーターに接続するため

## テスト確認項目

- [ ] GET /api/v1/users/quotaで自身のクォータが取得できること
- [ ] PUT /api/v1/users/quotaでクォータが更新できること
- [ ] 存在しないユーザーのクォータ取得で404が返ること
### repository 層テスト

- [ ] UserQuotaRepository.FindByUserIDでクォータが取得できること
- [ ] UserQuotaRepository.Updateでクォータが更新できること
