# ISSUE-046 Project削除フロー

## 親 Issue
ISSUE-044

## 概要
Project削除時に配下の全Deploymentを削除してからk8s Namespaceを削除するフローを実装する。全リソース削除完了後にNamespace WatcherがDBのProjectレコードを削除する。

## 変更ファイル一覧

- `app/src/service/project_service.go`（編集）
    - **何を**: DeleteProjectメソッドの拡張。Project.statusをdeletingに更新。配下の全Deploymentに対してDeleteDeploymentを実行。EnvVarとVolumeのstatusをdeletingに更新。全Deployment削除完了を監視するgoroutineを起動してk8s Namespaceを削除する。
    - **なぜ**: Project削除に連動して全リソースを順次削除するため
- `app/src/repository/deployment_repository.go`（編集）
    - **何を**: FindAllByProjectIDメソッドとUpdateAllStatusByProjectIDメソッドの追加または確認。
    - **なぜ**: 削除対象の全Deploymentを取得・一括更新するため

## テスト確認項目

- [ ] DELETE /api/v1/projects/:id後に全Deploymentがdeletingになること
- [ ] 全リソース削除後にk8s Namespaceが削除されること
- [ ] Namespace削除後にWatcherがProjectレコードを削除すること
- [ ] 削除中のProjectに新規Deployment作成で409が返ること
### repository 層テスト

- [ ] ProjectRepository.DeleteでProjectレコードが削除できること
- [ ] DeploymentRepository.FindAllByProjectIDで全Deploymentが取得できること
