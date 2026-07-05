# ISSUE-058 デプロイメント削除時に terminating 中にレコードが消える問題の修正

## 親 Issue
なし（バグ修正）

## 概要
Deployment を削除すると、k8s Deployment が Terminating 状態にある段階で DB の Deployment レコードが削除されてしまう問題を修正する。

### k8s Watch の terminating / 完全削除の判定方法

k8s の Watch API では以下の挙動となる：

- `watch.Modified` + `DeletionTimestamp != nil`: Terminating フェーズ（Pod の Graceful Termination が進行中）
- `watch.Deleted`: etcd からオブジェクトが完全削除された（完全削除済み）

`watch.Deleted` の時点では `k8sDeployment.DeletionTimestamp` が nil であることを確認することで Terminating との区別が可能。

### 原因
現在の `WatchDeployments`（`app/src/k8s/deployment.go:249`）は `watch.Deleted` ハンドラーで `deployment.status == deleting` のみチェックして連鎖削除を実行しており、`watch.Modified` + `DeletionTimestamp != nil`（Terminating）の段階では削除処理は走らない。

ただし以下の競合タイミング問題がある：

1. `DeleteDeployment` サービスが `status = deleting` を DB に書いた直後（同期的）に k8s リソース削除 API を呼び出す
2. k8s が即座に `watch.Deleted` イベントを発火させると、Watcher がほぼ同時に `status == deleting` を確認して連鎖削除を実行する
3. この場合 k8s Deployment が実際には Terminating 中（Pod が生きている）にもかかわらず DB レコードが消える

**修正方針**: `watch.Deleted` ハンドラーで `k8sDeployment.DeletionTimestamp != nil` の場合はまだ Terminating 中とみなしてスキップし、`DeletionTimestamp == nil`（完全削除済み）の場合のみ連鎖削除を実行する。

## 変更ファイル一覧

- `app/src/k8s/deployment.go`（編集）
    - 何を: `watch.Deleted` ハンドラーの冒頭で `k8sDeployment.DeletionTimestamp != nil` の場合に `return`（スキップ）するガード節を追加する。`watch.Modified` ハンドラーでは `DeletionTimestamp != nil` の場合に `app_status` を `terminating`（または既存の適切なステータス）に更新するロジックを追加する
    - なぜ: `DeletionTimestamp != nil` = Terminating 中（Pod が残っている）、`watch.Deleted` + `DeletionTimestamp == nil` = 完全削除済みという k8s の判定ルールに則って処理を分岐させるため

## テスト確認項目

- [ ] Deployment を DELETE した後、k8s Deployment が Terminating 中はフロントエンド上にデプロイメントが表示され続けること
- [ ] k8s Deployment が完全に削除された後（Watcher の watch.Deleted かつ DeletionTimestamp が nil）、DB の Deployment レコードと関連レコードが削除されること
- [ ] `status != deleting` の Deployment に対して k8s 側の Deleted イベントが来ても DB レコードが削除されないこと
