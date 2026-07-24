# ISSUE-032 Deployment Watcher

## 親 Issue
ISSUE-031

## 概要
k8s Deploymentリソースの状態変化を監視して、DBのDeploymentのapp_statusとk8s_statusを自動更新するWatcherを実装する。
複数Podが起動した際の重複監視を防ぐため、PostgreSQL Advisory Lock（pg_try_advisory_lock）を使ったリーダーエレクションを実装し、常に1つのPodのみがWatcherを実行する。リーダーPodが落ちた場合は別のPodが自動的に昇格する。

## 変更ファイル一覧

- `app/src/k8s/deployment.go`（編集）
    - **何を**: WatchDeployments関数の追加。k8s watch APIを使ってDeploymentの追加・変更・削除イベントを監視する。launchs.org/deployment-idラベルでDeploymentを特定し、ReadyReplicasの状態からapp_statusを計算してDBを更新する。appsv1.DeploymentStatusをJSONシリアライズしてk8s_statusに保存する。Deleted イベントでDBレコードを削除する。
    - **なぜ**: k8s側の実際のデプロイ状態をDBに反映するため
- `app/src/repository/deployment_repository.go`（編集）
    - **何を**: DeploymentRepositoryインターフェースにUpdateAppStatusとUpdateK8sStatusとDeleteメソッドを追加・実装する。
    - **なぜ**: WatcherからDB更新するためのrepositoryメソッドが必要なため
- `app/src/leader/election.go`（新規作成）
    - **何を**: RunAsLeader関数の実装。pg_try_advisory_lockで排他ロックを取得し、ロック取得に成功したPodのみがcallbackを実行する。DB接続が切れるとロックは自動解放され、別のPodが次のポーリングで昇格できる。ロック取得失敗時は一定間隔でリトライし続ける。
    - **なぜ**: 複数Pod起動時に1つのPodのみWatcherを実行するリーダーエレクションが必要なため
- `app/src/main.go`（編集）
    - **何を**: アプリ起動時にgoroutineでRunAsLeader経由でWatchDeployments()を起動する処理の追加。
    - **なぜ**: リーダーエレクションを通じてWatcherをバックグラウンドで常時起動させるため

## テスト確認項目

- [ ] k8s DeploymentがRunningになるとDBのapp_statusがrunningに更新されること
- [ ] k8s DeploymentがPendingのときDBのapp_statusがdeployingに更新されること
- [ ] k8s DeploymentがDeletedになるとDBレコードが削除されること
- [ ] ADDED/MODIFIEDイベントでk8s_statusにappsv1.DeploymentStatusがJSONで保存されること
- [ ] リーダーロックを取得したPodのみWatcherが実行されること
- [ ] リーダーがロックを保持している間、別のPodはロック取得待ちになること

### repository 層テスト

- [ ] DeploymentRepository.UpdateAppStatusでapp_statusが更新できること
- [ ] DeploymentRepository.UpdateK8sStatusでk8s_statusが更新できること
