# ISSUE-007 k8s Namespace管理

## 親 Issue
ISSUE-005

## 概要
プロジェクト作成・削除に連動してk8s Namespaceを作成・削除する関数を実装する。

## 変更ファイル一覧

- `app/src/k8s/namespace.go`（編集）
    - **何を**: CreateNamespace・DeleteNamespace関数の実装。名前空間の存在確認と冪等な作成・削除を行う。
    - **なぜ**: プロジェクトごとにk8sの分離環境（Namespace）が必要なため

## テスト確認項目

- [ ] Namespaceが正常に作成されること
- [ ] 既存Namespaceへの作成が冪等に動作すること
- [ ] Namespaceが正常に削除されること
- [ ] 存在しないNamespace削除が冪等に動作すること
