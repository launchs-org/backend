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

-- tasks テーブル: 非同期タスクキュー
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(255) PRIMARY KEY,
    task_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL,
    timeout_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    error_message TEXT,
    CONSTRAINT tasks_status_check CHECK (status IN ('pending', 'running', 'done', 'failed', 'cancelled'))
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_task_type ON tasks(task_type);
CREATE INDEX idx_tasks_timeout_at ON tasks(timeout_at);

-- pg_notify 用のリスナー関数
CREATE OR REPLACE FUNCTION notify_task_created()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('task_created', json_build_object(
        'id', NEW.id,
        'task_type', NEW.task_type,
        'created_at', NEW.created_at
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- tasks テーブル INSERT トリガー
DROP TRIGGER IF EXISTS trigger_notify_task_created ON tasks;
CREATE TRIGGER trigger_notify_task_created
    AFTER INSERT ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION notify_task_created();

-- main ユーザーに tasks テーブルへの権限を付与
GRANT ALL PRIVILEGES ON TABLE tasks TO main;
GRANT ALL PRIVILEGES ON SEQUENCE tasks_id_seq TO main;
GRANT EXECUTE ON FUNCTION notify_task_created() TO main;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO main;
