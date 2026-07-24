# PaaS 設計書 — 残課題

## 未決定事項はありません

すべての設計上の決定事項が確定しました。

---

## 確定済み決定事項一覧

| 項目 | 決定内容 |
|------|---------|
| テナント | シングルテナント |
| Ingress | Traefik IngressRoute CRD |
| ORM | GORM |
| pending_*** 方式 | API リクエストは pending_ なしフィールド名、サーバー側で pending_ カラムに書き込む |
| k8s_status 型 | JSONB。未同期時は null |
| ビルド実行環境 | k8s Job |
| ビルドログ | deployment_builds.build_log TEXT カラムに保存 |
| コンテナレジストリ | 自前構築（Harbor 等） |
| GitHub 連携 | パブリックリポジトリのみ（認証不要） |
| Webhook | deployment ごとに URL を発行。push branch が github_branch と一致する場合のみ自動 apply |
| Webhook 認証 | HMAC-SHA256（X-Hub-Signature-256）|
| ビルドキャンセル | apply 時に building 中の k8s Job を削除して新規ビルドを開始 |
| commit_sha HEAD | apply 時に GitHub API から最新 SHA を取得して上書き |
| build_directory | dockerfile/railpack 共通。このディレクトリに CD した状態でビルド開始 |
| IngressRoute 作成 | Service とは独立して任意タイミングで POST。Service と 1:1 |
| IngressRoute 変更 | 作成後変更不可 |
| rollback | 現時点では未実装（将来対応予定） |
| env_var value | 即時更新（pending なし）。k8s 反映は関連 deployment の apply 時 |
| env_var_mounts status | pending（未 apply）/ applied（apply 済み）|
| volume size_mb | 作成後変更不可 |
| PVC ReclaimPolicy | Delete（namespace 削除時に PV も削除） |
| project ステータス | provisioning / active / deleting |
| 削除フロー | 単体: deleting → k8s 削除確認 → DB 削除。Project: 全リソース削除 → namespace 削除 → DB 削除 |
| quota 管理 | accounts テーブルとは独立した account_quotas テーブル |
| k8s_status 同期 | Watcher プロセスが各リソースの k8s_status カラムをリアルタイム更新 |
| apply 完了検知 | Watcher が Pod Ready を検知して app_status = running に更新 |
| Secret 保管 | DB 平文保存（初期実装） |
