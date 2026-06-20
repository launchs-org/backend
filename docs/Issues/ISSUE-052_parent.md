# ISSUE-052 IngressRoute リファクタリング（Project 紐づけ・PathRule 分離）

## 概要

現状の IngressRoute は Deployment に 1:1 で紐づき、パス・ポートを直接フィールドに持つ設計になっている。
これをプロジェクト単位でドメインを払い出し、パスルールを別テーブル（PathRule）で管理する設計に変更する。

## 背景

- 現状は「1 Deployment = 1 IngressRoute = 1 パスルール」という制約がある
- 要件として「1 プロジェクトに 1 ドメインを払い出し、パスごとに対象サービスを選択する」形にしたい
- PathRule を独立したエンティティにすることで、複数パスルールの管理・pending 状態の追跡が可能になる

## 子 Issue

- ISSUE-053: IngressRoute モデル・リポジトリ変更（Project 紐づけ）
- ISSUE-054: PathRule モデル・リポジトリ作成
- ISSUE-055: IngressRoute・PathRule サービス・ハンドラー・ルーター変更
- ISSUE-056: k8s IngressRoute apply ロジック変更（PathRule 集約）
