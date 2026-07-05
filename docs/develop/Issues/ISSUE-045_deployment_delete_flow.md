# ISSUE-045 Deployment削除フロー

## 親 Issue
ISSUE-044

## 概要
Deployment削除時にk8s Deployment・Service・IngressRoute・ConfigMap・Secretを一括削除するフローを実装する。k8sリソース削除後、WatcherがDeletedイベントを検知してDBレコードを削除する。

## 変更ファイル一覧

- `app/src/service/deployment_service.go`（編集）
    - **何を**: DeleteDeploymentメソッドの拡張。statusをdeletingに更新してから関連するk8s全リソース（Deployment・Service・IngressRoute・ConfigMap・Secret）を削除する。EnvVarMountとVolumeMountのstatusをdeletingに更新する。
    - **なぜ**: Deployment削除に連動して関連k8sリソースを全て削除するため
- `app/src/k8s/deployment.go`（編集）
    - **何を**: WatchDeploymentsのDeletedイベント処理を拡張。関連する全DBレコード（EnvVarMount・VolumeMount・ApplyHistory・DeploymentBuild）を削除後にDeploymentレコードを削除する。
    - **なぜ**: k8s Deployment削除完了をトリガーにDBレコードを連鎖削除するため

## テスト確認項目

- [ ] DELETE /api/v1/deployments/:id後にk8s Deploymentが削除されること
- [ ] k8s Service・IngressRouteも削除されること
- [ ] k8s削除完了後にWatcherがDBレコードを削除すること
- [ ] 削除中のDeploymentにapplyすると409が返ること
### repository 層テスト

- [ ] DeploymentRepository.UpdateStatusでstatusをdeletingに更新できること
- [ ] DeploymentRepository.DeleteでDBレコードが削除できること
