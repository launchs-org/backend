# ISSUE-012 Applyサービス

## 親 Issue
ISSUE-009

## 概要
デプロイメントのpending_*フィールドをk8sに同期するApplyサービスを実装する。SELECT FOR UPDATEでロックを取得し、k8s apply成功後にpending値を昇格させてApplyHistoryを記録する。

## 変更ファイル一覧

- `app/src/service/apply.go`（編集）
    - **何を**: ApplyServiceインターフェースとApplyメソッドの実装。処理フロー：①SELECT FOR UPDATEでDeploymentをロック、②Projectを取得してnamespaceを解決、③pending_*値の有効値を解決、④InstanceSizeを取得、⑤Manifestを生成、⑥ApplyHistoryレコードを作成、⑦k8s Deploymentを作成または更新、⑧成功時はpending_*を昇格してstatusをrunningに更新、⑨失敗時はApplyHistoryをfailedに更新。
    - **なぜ**: k8s同期のビジネスロジックを集約し、データ整合性を保証するため

- `app/src/models/apply_history.go`（編集）
    - **何を**: ApplyHistoryモデルの定義。DeploymentIDへの外部キー、適用したManifestのJSONスナップショット、status（applied/failed）、エラーメッセージフィールドを持つ。
    - **なぜ**: Apply操作の監査ログをDBに記録するため

- `app/src/repository/apply_history_repository.go`（編集）
    - **何を**: ApplyHistoryRepositoryインターフェースと実装。Create・UpdateStatusメソッドを持つ。
    - **なぜ**: Apply履歴のDB操作を抽象化するため

## テスト確認項目

- [ ] pending_*フィールドがk8sに同期されること
- [ ] apply成功後にpending_*フィールドがクリアされること
- [ ] apply成功後にApplyHistoryレコードがappliedで作成されること
- [ ] k8s apply失敗時にApplyHistoryがfailedに更新されること
- [ ] apply中に同一Deploymentへの並行applyがブロックされること

### repository 層テスト

- [ ] ApplyHistoryRepository.Createで履歴レコードが作成できること
- [ ] ApplyHistoryRepository.UpdateStatusでstatusが更新できること
