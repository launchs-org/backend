-- テスト用 PostgreSQL 初期化スクリプト
-- postgres ユーザー（スーパーユーザー）で実行されます。

-- テスト用データベース作成
CREATE DATABASE testdb;

-- テスト用ユーザー作成
CREATE USER testuser WITH PASSWORD 'testpass';
GRANT ALL PRIVILEGES ON DATABASE testdb TO testuser;

\c testdb
GRANT ALL ON SCHEMA public TO testuser;
