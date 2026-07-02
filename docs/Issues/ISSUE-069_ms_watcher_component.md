# ISSUE-069: watcher コンポーネント作成

## 親 Issue

ISSUE-067: バックエンドをマイクロサービスに分割する

## 概要

K8s リソースの状態を監視し、変化を直接 PostgreSQL に反映する独立プロセス `watcher/` を作成する。
現行の `handler/src/` にある Watch ループ群とリーダーエレクションをそのまま移植し、`handler/src/main.go` から Watcher 起動コードを削除する。

## 実装手順

### 1. watcher/ ディレクトリ構造

```
watcher/
└── src/
    ├── go.mod          # module watcher; replace app/shared => ../../shared
    ├── main.go         # エントリーポイント（Watch ループ起動）
    ├── leader/         # 現行 handler/src/leader/ をそのままコピー
    └── k8s/            # Watch 関連ファイルを handler/src/k8s/ からコピー
        ├── client.go
        ├── deployment.go       # WatchDeployments
        ├── service.go          # WatchServices
        ├── ingress_route.go    # WatchIngressRoutes
        ├── pvc.go              # WatchPVCs
        ├── namespace.go        # WatchNamespaces
        ├── build.go            # WatchBuildJobs（ビルド起動ロジックは削除）
        ├── watch_metrics.go    # PollMetrics
        └── metrics.go
```

### 2. watcher/src/main.go

現行 `handler/src/main.go` のリーダーエレクション + Watcher 起動部分を移植する。

```go
func main() {
    // DB 接続初期化
    // K8s クライアント初期化
    // リーダーエレクション起動（PostgreSQL Advisory Lock）
    // leader.RunAsLeader(ctx, db, func(ctx context.Context) {
    //   go k8s.WatchDeployments(...)
    //   go k8s.WatchServices(...)
    //   ...
    // })
}
```

### 3. handler/src/main.go から削除するもの

- `leader.RunAsLeader(...)` の呼び出しと Watcher 起動コード
- Watcher に必要だった K8s クライアント・Repository の初期化（handler で不要になるもの）

## 移植元ファイル（handler/src/）

- `k8s/deployment.go` → `WatchDeployments`, `pollDeployments`
- `k8s/service.go` → `WatchServices`
- `k8s/ingress_route.go` → `WatchIngressRoutes`
- `k8s/pvc.go` → `WatchPVCs`
- `k8s/namespace.go` → `WatchNamespaces`
- `k8s/build.go` → `WatchBuildJobs`（ビルド起動ロジック `k8s.CreateBuildJob` は builder に移管するため削除）
- `k8s/watch_metrics.go` → `PollMetrics`
- `leader/election.go` → そのままコピー

## 変更予定ファイル

- `watcher/src/`（新規作成）
  - 何を: Watch ループ群を独立プロセスとして動かすコンポーネント
  - なぜ: handler から状態監視責務を分離するため

- `handler/src/main.go`（編集）
  - 何を: `leader.RunAsLeader` と Watcher 起動コードを削除
  - なぜ: Watcher は独立プロセスになるため handler では不要

- `docker-compose.yml`（編集）
  - 何を: `watcher` サービスを追加
  - なぜ: 独立コンテナとして起動するため

## テスト確認項目

- [ ] `docker compose exec watcher go build ./...` でビルドが通る
- [ ] `docker compose up` で watcher コンテナが起動する
- [ ] watcher を起動すると K8s Deployment の状態変化が DB に反映される
- [ ] watcher を再起動するとリーダーエレクション後に監視が再開される
- [ ] handler コンテナ起動時に Watcher 関連のコードが実行されなくなっている
