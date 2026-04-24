-- root権限で実行することを想定しています

-- データベースが存在しない場合のみ作成する
CREATE DATABASE IF NOT EXISTS authdb;
CREATE DATABASE IF NOT EXISTS maindb;

-- main ユーザーが存在しない場合は作成し、パスワードを main に設定
CREATE USER IF NOT EXISTS 'main'@'%' IDENTIFIED BY 'main';

-- authdb の全権限を main ユーザーに付与
GRANT ALL PRIVILEGES ON authdb.* TO 'main'@'%';

-- maindb の全権限を main ユーザーに付与
GRANT ALL PRIVILEGES ON maindb.* TO 'main'@'%';

-- 権限の変更を反映
FLUSH PRIVILEGES;