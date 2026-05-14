# プロジェクト概要 ✨

このプロジェクトは、Docker Compose を利用したマルチサービス Web アプリケーションです。

---

## 🛠️ 使用技術

*   **バックエンド:** Go, Echo Web Framework
*   **フロントエンド:** React, TypeScript, Vite, @react-router, npm
*   **インフラ:** Docker, Docker Compose, Nginx, OpenSSL
*   **タスクランナー:** Taskfile

---

## 🌐 アプリケーション構成

*   **Go バックエンド (`app`) 🖥️:**
    Echo フレームワーク製のシンプルな API サーバー。


*   **React フロントエンド (`frontend`) ⚛️:**
    React と `@react-router` を使用したウェブアプリ。Vite で開発。


*   **Nginx (`nginx`) 🕸️:**
    リバースプロキシとして機能し、トラフィックを振り分けます。
    自己署名SSL証明書 (HTTPS) を使用し、ポート `8443` でリクエストを受け付けます。


*   **OpenSSL (`openssl`) 🔑:**
    Nginx 用の SSL 証明書と JWT 認証用の Ed25519 キーを生成します。

---

## 🚦 Nginx リバースプロキシ設定

Nginx はポート `8443` で HTTPS リクエストを受け付け、以下のルーティングを行います。

*   `/app/` へのリクエスト: Go バックエンド (`app` サービス) のポート `8080` へプロキシ。
*   `/ui/` へのリクエスト: React フロントエンド (`frontend` サービス) のポート `3000` へプロキシ。
*   `/statics/` へのリクエスト: Nginx コンテナ内の `/var/www/nginx/` ディレクトリから静的ファイルを直接配信。
    **フロントエンドのコードは `/statics/ からは提供されません。**

---

# 環境設定 ⚙️

プロジェクトの設定と実行には、[Taskfile](https://taskfile.dev) を使用します。

Taskfile の詳細はこちら: `https://taskfile.dev`

1.  **初期セットアップ:**

    必要なキーの生成と全サービスの起動を行うには、次のコマンドを実行します。

    ```bash
    task setup
    ```

    このコマンドは以下の処理を実行します。

    *   SSL 証明書 (`server.crt`, `server.key`) と Ed25519 キーペアを生成し、`nginx/keys` および `openssl/jwtKeys` に保存。
    *   Go バックエンド、React フロントエンド、Nginx の Docker イメージをビルド。
    *   `docker compose up -d` を使用して、全サービスをデタッチモードで起動。


2.  **アプリケーションへのアクセス:**

    *   **フロントエンド UI 💡:**
        `https://localhost:8443/ui/` にアクセス。

    *   **バックエンド API 🔗:**
        `https://localhost:8443/app/` を介して API エンドポイントにアクセス。
        
        例: `https://localhost:8443/app/` で Go バックエンドのルートエンドポイントにヒット。

    *   **Nginx 静的ファイル 📂:**
        `https://localhost:8443/statics/` から Nginx が直接配信する静的コンテンツにアクセス。

---

# コマンド一覧 📋

## 一般的なコマンド (Taskfile と Docker Compose)

*   **`task setup`** ▶️:
    (推奨) キーを生成し、全サービスをデタッチモードで起動。

*   **`task genkey`** 🔑:
    SSL 証明書と JWT キーを生成します (`task setup` が自動で呼び出し)。

*   **`docker compose up -d`** ⬆️:
    `docker-compose.yaml` で定義された全サービスをデタッチモードで起動。

*   **`docker compose up`** 🚀:
    全サービスをフォアグラウンドで起動 (ログの直接確認に便利)。

*   **`docker compose down`** ⬇️:
    `docker compose up` で作成された全サービスを停止・削除。

*   **`docker compose build`** 🏗️:
    サービスイメージをビルドまたはリビルド。クリーンなリビルドは `docker compose build --no-cache`。

*   **`docker compose logs -f`** 📝:
    全サービスのログを追跡。

*   **`task logs`** 📜:
    全サービスのログをリアルタイムで表示します。

*   **`task logs:frontend`** 🌐📝:
    フロントエンドサービスのログをリアルタイムで表示します。

*   **`task logs:backend`** 🖥️📝:
    バックエンドサービスのログをリアルタイムで表示します。

---

# 開発とテスト 🧪

## 1. データベース接続テスト

共有モデル (`shared/models`) のデータベース接続とマイグレーションのテストを実行できます。

### 前提条件
- データベースコンテナが起動していること (`task setup` 済み)
- ホストから DB にアクセスできること (`docker-compose.yaml` でポート `5432` が公開されていること)

### 実行方法
以下のコマンドを実行すると、自動的に `.env.test` が作成（存在しない場合）され、テストが実行されます。

```bash
task test:db
```

### 環境変数のカスタマイズ
初回実行時に作成される `.env.test` を編集することで、接続先や認証情報を変更できます。

```bash
# .env.test の例
DB_HOST=localhost
DB_PORT=5432
DB_USER=acp
DB_PASSWORD=example
DB_NAME=acp
```

## 2. テスト実行

### 実行方法
```bash
docker compose exec -T app go test ./controller/... ./service/... ./model/...
```

