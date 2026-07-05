# ISSUE-020 Apply拡張（Service・Ingress対応）

## 親 Issue
ISSUE-015

## 概要
ApplyサービスにService・IngressRouteのk8s同期処理を追加する。apply時にDeploymentと合わせてServiceとIngressRouteも作成・更新する。

## 変更ファイル一覧

- `app/src/service/apply.go`（編集）
    - **何を**: Applyメソッドの拡張。k8s Deployment同期後にk8s Serviceの適用を追加。IngressRouteレコードが存在する場合はTraefik IngressRouteも適用。apply成功後にService・IngressRouteのpending_*フィールドも昇格。
    - **なぜ**: ネットワーク公開のためにDeploymentと合わせてService/IngressRouteを同期する必要があるため

## テスト確認項目

- [ ] applyでk8s Serviceが作成・更新されること
- [ ] IngressRoute設定がある場合にTraefik IngressRouteが作成・更新されること
- [ ] apply後にService・IngressRouteのpending_*フィールドがクリアされること
- [ ] k8s Service apply失敗時にApplyHistoryがfailedに更新されること
