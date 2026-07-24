# ISSUE-019 Traefik IngressRoute操作

## 親 Issue
ISSUE-015

## 概要
Traefik CRD（IngressRoute）のCRUD操作をdynamic clientで実装する。

## 変更ファイル一覧

- `app/src/k8s/ingress_route.go`（新規作成）
    - **何を**: ApplyIngressRoute（作成または更新）・DeleteIngressRoute関数の実装。dynamic.Interfaceを使ってTraefik IngressRoute CRDを操作する。ホスト名・パスルールのrouterRule文字列生成。TLS設定の付与。
    - **なぜ**: Traefik IngressRouteはk8s標準リソースではなくCRDのため、dynamic clientが必要なため

## テスト確認項目

- [ ] Traefik IngressRouteが正常に作成されること
- [ ] 既存IngressRouteが更新されること（冪等性）
- [ ] TLS設定が正しくManifestに反映されること
- [ ] IngressRouteが正常に削除されること
