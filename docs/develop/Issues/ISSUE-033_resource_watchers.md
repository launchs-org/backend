# ISSUE-033 リソースWatcher群

## 親 Issue
ISSUE-031

## 概要
k8s Service・IngressRoute・PVC・ConfigMap・SecretのWatcherを実装する。各リソースの状態変化に応じてDBのstatusを更新する。

## 変更ファイル一覧

- `app/src/k8s/service.go`（編集）
    - **何を**: WatchServices関数の追加。k8s Service watch APIでイベントを監視してDBのService.statusを更新する。
    - **なぜ**: k8s Serviceの状態をDBに反映するため
- `app/src/k8s/ingress_route.go`（編集）
    - **何を**: WatchIngressRoutes関数の追加。Traefik IngressRoute CRDのイベントを監視してDBのIngressRoute.statusを更新する。
    - **なぜ**: Traefik IngressRouteの状態をDBに反映するため
- `app/src/k8s/pvc.go`（編集）
    - **何を**: WatchPVCs関数の追加。PVCのBound状態を監視してDBのVolume.statusを更新する。
    - **なぜ**: PVCのバインド状態をDBに反映するため
- `app/src/main.go`（編集）
    - **何を**: 各Watcher関数をgoroutineで起動する処理の追加。
    - **なぜ**: 全Watcherをバックグラウンドで常時起動させるため

## テスト確認項目

- [ ] k8s Serviceのstatusが変化するとDBが更新されること
- [ ] IngressRouteのstatusが変化するとDBが更新されること
- [ ] PVCがBoundになるとDBのVolume.statusがboundに更新されること
