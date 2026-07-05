# ISSUE-034 Namespace Watcher

## 親 Issue
ISSUE-031

## 概要
k8s Namespaceの削除イベントを監視して、Project削除フローの完了処理（DBレコード削除）を行うWatcherを実装する。

## 変更ファイル一覧

- `app/src/k8s/namespace.go`（編集）
    - **何を**: WatchNamespaces関数の追加。k8s Namespace watch APIでDeletedイベントを監視する。launchs.org/project-idラベルでProjectを特定し、DBのProjectレコードを削除する。
    - **なぜ**: Namespace削除完了をトリガーにProjectのDB削除を行うため
- `app/src/main.go`（編集）
    - **何を**: goroutineでWatchNamespaces()を起動する処理の追加。
    - **なぜ**: Namespace WatcherをバックグラウンドでDB削除をトリガーするため

## テスト確認項目

- [ ] k8s NamespaceのDeletedイベントでDBのProjectレコードが削除されること
- [ ] 他プロジェクトのNamespace削除イベントで別プロジェクトに影響しないこと
### repository 層テスト

- [ ] ProjectRepository.DeleteでProjectレコードが削除できること
