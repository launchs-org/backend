# PaaS 設計書 — 概要・方針

## システム概要

Kubernetes 上に構築する PaaS。既存イメージ / Dockerfile / railpack によるビルドからのデプロイ、Service による外部公開、Traefik IngressRoute によるインターネット公開を提供する。

---

## 技術スタック前提

| 項目 | 採用 |
|------|------|
| コンテナオーケストレーション | Kubernetes |
| Ingress コントローラー | Traefik (IngressRoute CRD) |
| ORM | GORM |
| テナント | シングルテナント |
| ドメイン形式 | `{name}-{uuid8}.launchs.org` |
| コンテナレジストリ | 自前構築（Harbor 等） |
| ビルド実行環境 | k8s Job |

---

## 設計方針

### API リクエストのフィールド名規則

- クライアントは常に `pending_` なしのフィールド名でリクエストを送る
- サーバー側で `pending_***` カラムに書き込む
- レスポンスは `pending_***` カラムをそのまま返す（フロントは `pending_***` が非空かどうかで未 apply の変更を検知）
- 例: `PUT /deployments/:id` に `{ "image_url": "nginx:1.25" }` → DB の `pending_image_url` に書き込む

### `pending_***` カラム方式

- 設定変更 → `pending_***` カラムに書き込み（現在稼働中の値はそのまま残る）
- `POST /deployments/:id/apply` を叩いた瞬間に `pending_***` を k8s に適用し、空文字にする
- 初回作成時は全フィールドを `pending_***` に入れ、`status = pending` とする
- apply が押されて初めて初期デプロイが実行される

### apply フロー概要

```
POST /apply
  → DB ロック取得 (SELECT FOR UPDATE)
  → env_var 実効キー重複チェック
  → pending_github_commit_sha が HEAD の場合、GitHub API で最新 SHA を取得して上書き
  → ビルドが必要かどうか判定（type が dockerfile/railpack かつ GitHub 情報が更新されている場合）
  → 必要であれば既存ビルドをキャンセルし、k8s Job を作成してビルドを非同期実行
  → k8s manifest を生成（Deployment / Service / IngressRoute / ConfigMap / Secret / PVC）
  → apply_history に manifest スナップショットを INSERT
  → k8s server-side apply 実行
  → 成功: pending_*** を空に、current 値に昇格、app_status = deploying
  → 失敗: apply_history.status = failed、error_message を記録、pending_*** はそのまま
  → ロック解放
```

### ビルド判定ロジック

apply 時に以下のいずれかが現在値と異なる場合にビルドを実行する。

- `pending_github_commit_sha`（HEAD の場合は最新 SHA を取得して比較）
- `pending_github_branch`
- `pending_github_repo_url`
- `pending_github_repo_directory`（build_directory）
- `pending_dockerfile_path`（type=dockerfile のみ）

ビルド中に再度 apply が来た場合は、既存の building 状態のビルド（k8s Job）をキャンセルして新しいビルドを開始する。

`current_build_id` が空 = ビルド中。完了時に新しい build_id をセット。

### apply 完了検知

Watcher プロセスが k8s Deployment の rollout 完了を Watch し、`deployments.k8s_status` を更新する。Pod が Ready になったタイミングで `app_status = running` に更新する。

### commit_sha の HEAD 解決

`pending_github_commit_sha = "HEAD"` の場合、apply 実行時に GitHub API から最新コミット SHA を取得し、`pending_github_commit_sha` を実際の SHA で上書きしてからビルドを実行する。

### GitHub 連携

- パブリックリポジトリのみ対応（認証不要）
- Webhook エンドポイントを deployment ごとに発行: `POST /webhooks/{deployment_id}/github`
- push イベントを受け取り、push された branch が `deployment.github_branch` と一致する場合のみ自動 apply をトリガー
- Webhook の登録・削除は専用 API で管理（deployment に `webhook_secret` を持たせ HMAC 検証）

### ビルド基盤

- k8s Job としてビルドを実行
- ビルドログは `deployment_builds.build_log` TEXT カラムに保存
- ビルド完了後、built image を自前レジストリに push

### k8s_status の扱い

- 各リソーステーブルに `k8s_status JSONB` カラムを持つ
- Watcher がリアルタイムで更新
- 未同期時は `null` で返す

### 削除フロー

**単体リソース削除:**
```
status → deleting → k8s リソース削除確認 → DB レコード削除
```

**Project 削除:**
```
project.status → deleting
→ 配下の全リソースを deleting に一斉更新
→ Watcher が各リソースの k8s 削除完了を検知 → DB レコード個別削除
→ 全リソース削除完了 → k8s namespace 削除（PVC ReclaimPolicy=Delete のため PV も同時削除）
→ namespace 削除完了 → project レコード削除
```

### 環境変数の更新

- `env_vars.value` は即時更新（pending なし）
- k8s への反映は apply されるまで行われない
- `env_var_mounts.status` で適用済みかどうかを管理
  - `pending`: mount 済みだが未 apply
  - `applied`: apply 済み

---

## エンティティ関係

```

projects (user_id)
  ├── deployments (project_id FK)
  │     ├── deployment_builds (deployment_id FK)
  │     ├── apply_history (deployment_id FK)
  │     ├── deployment_webhooks (deployment_id FK)
  │     ├── services (deployment_id FK, 1:1)
  │     │     └── ingress_routes (service_id FK, 1:1)
  │     ├── env_var_mounts (deployment_id FK) ←→ env_vars
  │     └── volume_mounts (deployment_id FK)  ←→ volumes
  ├── env_vars (project_id FK)
  └── volumes (project_id FK)

instance_sizes (グローバルマスター)
user_quotas (user_id, 1:1)
```
