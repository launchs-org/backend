# ISSUE-068: 基盤整備・shared モジュール作成

## 親 Issue

ISSUE-067: バックエンドをマイクロサービスに分割する

## 概要

マイクロサービス分割の基盤となる以下を整備する。

1. `shared/` Go モジュールの作成（全コンポーネント共通コード）
2. `app/` → `handler/` へのリネーム
3. `docker-compose.yml` に Temporal Server を追加
4. 各コンポーネントディレクトリの `go.mod` 雛形作成

## 実装手順

### 1. shared/ モジュール作成

```
shared/
├── go.mod          # module app/shared
├── models/         # 現行 app/src/models/ をそのままコピー
├── repository/     # 現行 app/src/repository/ をそのままコピー
├── dto/            # 新規作成（コンポーネント間で共有する構造体）
│   ├── apply.go    # ApplyRequest / ApplyWorkflowInput
│   ├── build.go    # BuildRequest / BuildWorkflowInput
│   └── project.go  # CreateProjectWorkflowInput 等
├── logger/         # 現行 app/src/logger/ をそのままコピー
└── config/         # 現行 app/src/config/ をそのままコピー
```

### 2. app/ → handler/ リネーム

- ディレクトリ名変更: `app/` → `handler/`
- `handler/src/go.mod` のモジュール名を `app` から `handler` に更新
- `docker-compose.yml` のビルドパス・サービス名を `app` → `handler` に更新

### 3. docker-compose.yml 更新

Temporal Server を追加する。

```yaml
services:
  temporal:
    image: temporalio/auto-setup:latest
    ports:
      - "7233:7233"   # Temporal gRPC
      - "8088:8088"   # Temporal Web UI
    environment:
      - DB=postgres
      - DB_PORT=5432
      - POSTGRES_USER=${DB_USER}
      - POSTGRES_PWD=${DB_PASSWORD}
      - POSTGRES_SEEDS=${DB_HOST}
    depends_on:
      - db
```

### 4. 各コンポーネントの go.mod 雛形作成

`controller/`, `watcher/`, `builder/` それぞれに以下の `go.mod` を作成する。

```
module controller  # watcher / builder それぞれ

go 1.25

require (
    app/shared v0.0.0
    go.temporal.io/sdk v1.x.x
    # ... その他依存
)

replace app/shared => ../../shared
```

## 変更予定ファイル

- `shared/`（新規作成）
  - 何を: 全コンポーネント共通コードを集約する Go モジュール
  - なぜ: models/repository/dto の重複を避けるため

- `app/` → `handler/`（リネーム）
  - 何を: ディレクトリ名・`go.mod` モジュール名・docker-compose のパスを変更
  - なぜ: コンポーネント名を明確にするため

- `docker-compose.yml`（編集）
  - 何を: Temporal Server サービスを追加
  - なぜ: 開発環境で Temporal を動かすため

- `controller/src/go.mod`, `watcher/src/go.mod`, `builder/src/go.mod`（新規作成）
  - 何を: 各コンポーネントの Go モジュール定義（shared への replace ディレクティブ含む）
  - なぜ: 次の Issue での実装のための土台を作るため

## テスト確認項目

- [ ] `docker compose build` が全サービスでエラーなく完了する
- [ ] `docker compose up` で handler + PostgreSQL + Temporal が起動する
- [ ] Temporal Web UI（http://localhost:8088）にアクセスできる
- [ ] `docker compose exec handler go build ./...` でビルドが通る
- [ ] `shared/` の `go build ./...` が通る
