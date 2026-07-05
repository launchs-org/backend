# ISSUE-050 Quotaチェック実装

## 親 Issue
ISSUE-049

## 概要
deployment作成・更新・apply時のQuotaチェックをservice/quota_service.goに実装して各エンドポイントに組み込む。

## 変更ファイル一覧

- `app/src/service/quota_service.go`（編集）
    - **何を**: CheckProjectQuota・CheckDeploymentQuota・CheckReplicasQuota・CheckVolumeQuota関数の実装。UserQuotaRepositoryで上限値と現在の使用量を取得して比較する。超過時はsentinel error（ErrProjectQuotaExceeded等）を返す。
    - **なぜ**: Quotaチェックロジックを集約してDRYに保つため
- `app/src/repository/user_quota_repository.go`（編集）
    - **何を**: CountProjects・CountDeployments・SumVolumeMB・CountReplicasメソッドの追加。ユーザーIDで絞り込んで現在の使用量をカウントする。
    - **なぜ**: QuotaチェックのDB集計クエリをリポジトリ層に集約するため
- `app/src/service/project_service.go`（編集）
    - **何を**: CreateProjectメソッドの先頭にCheckProjectQuota呼び出しを追加。
    - **なぜ**: プロジェクト作成前にQuotaを検証するため
- `app/src/service/deployment_service.go`（編集）
    - **何を**: CreateDeploymentメソッドの先頭にCheckDeploymentQuota呼び出しを追加。UpdateDeploymentとApplyDeploymentにCheckReplicasQuota呼び出しを追加。
    - **なぜ**: デプロイメント作成・更新・apply前にQuotaを検証するため
- `app/src/service/volume_service.go`（編集）
    - **何を**: CreateVolumeメソッドの先頭にCheckVolumeQuota呼び出しを追加。
    - **なぜ**: ボリューム作成前にQuotaを検証するため
- `app/src/handler/project_handler.go`（編集）
    - **何を**: CreateProjectハンドラーでErrProjectQuotaExceededをキャッチして400レスポンスを返す処理の追加。
    - **なぜ**: Quotaエラーを適切なHTTPステータスに変換するため
- `app/src/handler/deployment_handler.go`（編集）
    - **何を**: CreateDeployment・UpdateDeployment・ApplyDeploymentハンドラーでQuota系エラーをキャッチして400レスポンスを返す処理の追加。
    - **なぜ**: Quotaエラーを適切なHTTPステータスに変換するため

## テスト確認項目

- [ ] max_projectsを超えるプロジェクト作成で400が返ること
- [ ] max_deploymentsを超えるデプロイメント作成で400が返ること
- [ ] max_replicas_per_deploymentを超えるreplicas設定で400が返ること
- [ ] max_volume_mbを超えるボリューム作成で400が返ること
- [ ] Quota更新後に新しい制限が即時反映されること
### repository 層テスト

- [ ] UserQuotaRepository.CountProjectsでプロジェクト数が正しくカウントされること
- [ ] UserQuotaRepository.CountDeploymentsでデプロイメント数が正しくカウントされること
- [ ] UserQuotaRepository.SumVolumeMBでボリューム合計が正しく集計されること
