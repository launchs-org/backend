# ISSUE-017 k8s Service操作

## 親 Issue
ISSUE-015

## 概要
k8s ServiceリソースのCRUD操作関数を実装する。

## 変更ファイル一覧

- `app/src/k8s/service.go`（新規作成）
    - **何を**: ApplyService（作成または更新）・DeleteService関数の実装。ServiceのTypeに応じてClusterIP/NodePort/LoadBalancerを生成する。Deploymentとのセレクター一致を保証する。
    - **なぜ**: k8s Service操作の実装を集約するため

## テスト確認項目

- [ ] k8s Serviceが正常に作成されること
- [ ] 既存Serviceが更新されること（冪等性）
- [ ] k8s Serviceが正常に削除されること
