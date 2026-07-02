-- PostgreSQL初期化スクリプト
-- postgres ユーザー（スーパーユーザー）で実行されます。

-- ─────────────────────────────────────────
-- データベース作成
-- ─────────────────────────────────────────
CREATE DATABASE authdb;
CREATE DATABASE maindb;

-- Temporal 用データベース（temporal-admin-tools が自動スキーマ作成するため空で作成）
CREATE DATABASE temporal;
CREATE DATABASE temporal_visibility;

-- ─────────────────────────────────────────
-- ユーザー作成
-- ─────────────────────────────────────────

-- アプリケーション用メインユーザー
CREATE USER main WITH PASSWORD 'main';
GRANT ALL PRIVILEGES ON DATABASE authdb TO main;
GRANT ALL PRIVILEGES ON DATABASE maindb TO main;

-- Temporal 専用ユーザー（auto-setup が DB 作成を試みるため CREATEDB が必要）
CREATE USER temporal WITH PASSWORD 'temporal' CREATEDB;
GRANT ALL PRIVILEGES ON DATABASE temporal TO temporal;
GRANT ALL PRIVILEGES ON DATABASE temporal_visibility TO temporal;

-- ─────────────────────────────────────────
-- スキーマ権限付与（各DBに接続して実行）
-- ─────────────────────────────────────────

\c authdb
GRANT ALL ON SCHEMA public TO main;

\c maindb
GRANT ALL ON SCHEMA public TO main;

\c temporal
GRANT ALL ON SCHEMA public TO temporal;

\c temporal_visibility
GRANT ALL ON SCHEMA public TO temporal;

\c maindb
