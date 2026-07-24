# ISSUE-051 k8s Job（dockerfile ビルダー）実装

## 親 Issue
ISSUE-035

## 概要
dockerfile を使ったコンテナビルドを k8s Job として実行する操作関数を実装する。
ISSUE-037 にて `CreateBuildJob` は未実装エラーを返す仮実装となっているため、本 Issue で設計・実装を行う。

## 背景
ISSUE-037 では railpack ビルドタイプを `railpack` パッケージ経由で実装した。
dockerfile ビルドタイプは builder イメージの設計・選定が必要なため、別 Issue として切り出した。

## 実装手順

### 検討事項
- dockerfile ビルダーイメージの選定（kaniko / buildkit / 独自イメージ等）
- Harbor への push 認証方式
- initContainer 構成（git clone → docker build → push）
- リソース制限・タイムアウト設定
- railpack パッケージと同様のクライアント設計にするか検討する

### 変更ファイル（予定）

- `app/src/k8s/build.go`（編集）
    - 何を: `CreateBuildJob` に dockerfile ビルダーの Job 作成ロジックを実装する
    - なぜ: 現在は未実装エラーを返す仮実装のため

- `app/src/service/build_service.go`（編集）
    - 何を: `BuildTypeDockerfile` ケースで実装済みの `CreateBuildJob` を呼び出す
    - なぜ: 現在は未実装エラーが返るため dockerfile タイプのビルドが起動できない

## テスト確認項目

- [ ] dockerfile 指定の Job が正常に作成されること
- [ ] Harbor 認証情報が Job の env 変数に正しく設定されること
- [ ] k8s Job が正常に削除されること
- [ ] ビルド成功時にイメージが Harbor へ push されること
