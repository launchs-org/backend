# ISSUE-027 ボリュームCRUD

## 親 Issue
ISSUE-026

## 概要
PersistentVolumeClaimの設定を管理するボリュームのCRUDエンドポイントを実装する。ボリュームはプロジェクトスコープで管理する。

## 変更ファイル一覧

- `app/src/models/volume.go`（変更なし）
    - **何を**: 変更不要。docs/02_tables.md のテーブル定義に storage_class フィールドは存在しないため、現行モデル（ID・ProjectID・Name・SizeMB・Status・K8sStatus）をそのまま使用する。
    - **なぜ**: storage_class はテーブル定義に含まれておらず、追加は不要なため

- `app/src/repository/volume_repository.go`（新規作成）
    - **何を**: VolumeRepositoryインターフェースと実装。Create・FindByID・FindAllByProjectID・Deleteメソッドを持つ。
    - **なぜ**: ボリュームのDB操作を抽象化するため

- `app/src/service/volume_service.go`（新規作成）
    - **何を**: VolumeServiceインターフェースと実装。ListVolumes・CreateVolume・DeleteVolumeのCRUD。すべての操作でProjectのUserIDとリクエストユーザーIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: ボリューム管理のビジネスロジックをハンドラーから分離するため。また、他ユーザーのプロジェクトリソースへの不正アクセスを防ぐため

- `app/src/handler/volume_handler.go`（新規作成）
    - **何を**: ListVolumes・CreateVolume・DeleteVolumeハンドラーの実装。
    - **なぜ**: ボリュームCRUDのHTTPエントリーポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/POST /api/v1/projects/:id/volumes、DELETE /api/v1/volumes/:idエンドポイントの登録。
    - **なぜ**: ボリュームCRUDエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/projects/:id/volumesでボリュームが作成できること
- [ ] GET /api/v1/projects/:id/volumesでボリューム一覧が取得できること
- [ ] DELETE /api/v1/volumes/:idでボリュームが削除できること
- [ ] 他ユーザーのProjectにPOST /volumesすると403が返ること
- [ ] 他ユーザーのProjectのGET /volumesすると403が返ること
- [ ] 他ユーザーのProjectのボリュームをDELETEすると403が返ること

### repository 層テスト

- [ ] VolumeRepository.Createでボリュームが作成できること
- [ ] VolumeRepository.FindAllByProjectIDでプロジェクトのボリューム一覧が取得できること
