-- PostgreSQL初期化スクリプト
-- このスクリプトは postgres ユーザー（スーパーユーザー）で実行されます。

-- データベースの作成
CREATE DATABASE authdb;
CREATE DATABASE maindb;
CREATE DATABASE taskdb;

-- メインユーザーの作成
CREATE USER main WITH PASSWORD 'main';
GRANT ALL PRIVILEGES ON DATABASE authdb TO main;
GRANT ALL PRIVILEGES ON DATABASE maindb TO main;

-- taskdb 専用ユーザーの作成
CREATE USER task_user WITH PASSWORD 'task_pass';
GRANT ALL PRIVILEGES ON DATABASE taskdb TO task_user;

-- 各データベースに対して権限を付与するための設定（PostgreSQLでは接続後にGRANTが必要な場合があるため）
-- 以下の操作は各データベースに接続して実行する必要がありますが、
-- docker-point-initdb.d では単一のスクリプトとして実行されるため、
-- 必要に応じて接続切り替えを行います。

\c authdb
GRANT ALL ON SCHEMA public TO main;

\c maindb
GRANT ALL ON SCHEMA public TO main;

\c taskdb
GRANT ALL ON SCHEMA public TO task_user;

\c maindb
