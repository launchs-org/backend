# ISSUE-039 ビルドWatcher

## 親 Issue
ISSUE-035

## 概要
k8s Jobの完了・失敗イベントを監視してDeploymentBuildのstatusを更新するWatcherを実装する。ビルド成功時はDeploymentのpending_image_urlを更新する。

## 変更ファイル一覧

- `app/src/k8s/build.go`（編集）
    - **何を**: WatchBuildJobs関数の追加。k8s Job watch APIでComplete・Failedイベントを監視する。launchs.org/build-idラベルでDeploymentBuildを特定する。成功時はDeploymentBuild.statusをsucceededに更新し、Deploymentのpending_image_urlにビルドイメージURLをセットする。失敗時はDeploymentBuild.statusをfailedに更新する。
    - **なぜ**: k8s Jobの完了状態をDBに反映するため
- `app/src/main.go`（編集）
    - **何を**: goroutineでWatchBuildJobs()を起動する処理の追加。
    - **なぜ**: ビルドWatcherをバックグラウンドで常時起動させるため

## テスト確認項目

- [ ] k8s JobがCompleteになるとDeploymentBuild.statusがsucceededになること
- [ ] ビルド成功時にDeploymentのpending_image_urlが更新されること
- [ ] k8s JobがFailedになるとDeploymentBuild.statusがfailedになること
