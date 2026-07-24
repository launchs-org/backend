# launchs-org-backend

Go + React + Nginx + PostgreSQL の Docker Compose 構成テンプレート。JWT 認証サービス込みで即起動できる開発環境。

## 技術スタック

| レイヤー | 技術 |
|---------|------|
| バックエンド | Go, Echo v4 |
| フロントエンド | React 19, TypeScript, Vite, Tailwind CSS |
| 認証 | authbase v0.1.6 (JWT / Ed25519) |
| リバースプロキシ | Nginx (HTTPS, ポート 8955) |
| データベース | PostgreSQL 18 |
| DB 管理 | pgAdmin 4 |
| オーケストレーション | Docker Compose |
| タスクランナー | Taskfile |

## アーキテクチャ

```
クライアント
    │
    └── https://localhost:8955
            │
           Nginx
            ├── /app/    → Go バックエンド (8080)
            ├── /auth/   → authbase 認証サービス (8080)
            ├── /ui/     → React フロントエンド (3000)
            └── /statics/ → Nginx 直接配信

http://localhost:8080 → pgAdmin
```

**データベース構成**

| DB 名 | 用途 |
|-------|------|
| `maindb` | アプリケーションデータ |
| `authdb` | 認証情報 |
| `temporal` / `temporal_visibility` | ワークフロー管理 (Temporal 向け) |

## 起動方法

**前提条件:** Docker, Docker Compose, Taskfile, Python 3, OpenSSL

```bash
task setup
```

SSL 証明書と JWT キーの生成・Docker イメージのビルド・全サービスの起動をまとめて実行します。

## アクセス先

| サービス | URL |
|---------|-----|
| フロントエンド | https://localhost:8955/ui/ |
| バックエンド API | https://localhost:8955/app/ |
| 認証 | https://localhost:8955/auth/ |
| 静的ファイル | https://localhost:8955/statics/ |
| pgAdmin | http://localhost:8080 |

自己署名証明書を使用しているため、ブラウザの警告は無視して進んでください。

## API ドキュメント

OpenAPI 仕様書は [https://launchs-org.github.io/backend/](https://launchs-org.github.io/backend/) で公開されています（`develop` ブランチの `docs/openapi.yaml` 更新時に自動反映）。

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `task setup` | キー生成 → ビルド → 起動（初回はこれだけ） |
| `task config` | 環境変数ファイルを生成 |
| `task genkey` | SSL 証明書と JWT キーを生成 |
| `task reset` | コンテナを完全削除して再セットアップ |
| `task release` | 本番向けイメージをビルド |
| `task logs` | 全サービスのログをリアルタイム表示 |
| `task logs:frontend` | フロントエンドのログのみ表示 |
| `task logs:backend` | バックエンドのログのみ表示 |

停止するには `docker compose down`。
