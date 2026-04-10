import secrets
import os
import sys

def generate_random_key(length=64):
    """
    暗号学的に安全なランダムキーを、指定された長さ（デフォルト64文字）で生成します。
    """
    return secrets.token_urlsafe(length)

def confirm_overwrite_all(files_to_check):
    """
    主要な設定ファイルが存在するかを確認し、上書きするかを尋ねます。
    上書きが許可されない場合はFalseを返します。
    """
    existing_files = [f for f in files_to_check if os.path.exists(f)]

    if existing_files:
        print("\n--- ファイルの上書き確認 ---")
        print(f"以下のファイルが既に存在します: {', '.join(existing_files)}")
        response = input("これらのファイルをすべて上書きしますか？ (y/n): ")
        if response.lower() != 'y':
            print("ファイルの生成を中止しました。")
            return False
    return True

def create_env_file(file_path, content):
    """
    指定されたファイルパスに、指定された内容で設定ファイルを生成します。
    """
    with open(file_path, "w", encoding="utf-8") as file:
        file.write(content.strip())
    print(f"✅ ファイル '{file_path}' を生成しました。")

def generate_db_config(db_password):
    return f"""
DB_HOST=db
DB_USER=acp
DB_PASSWORD={db_password}
DB_NAME=acp
DB_PORT=5432
"""

def main():
    """
    メイン処理：コンテナごとに個別の .env 設定ファイルを生成します。
    """
    # 作業ディレクトリを./dataに移動し、存在しなければ作成
    data_dir = "./data"
    os.makedirs(data_dir, exist_ok=True)
    os.chdir(data_dir)

    print("--- 各コンテナ用設定ファイルの生成を開始 ---")

    # 生成するファイルのリスト
    env_files = ["db.env", "auth.env", "build.env", "deploy.env"]
    
    # ファイルの上書き確認
    if not confirm_overwrite_all(env_files):
        return

    # 共通のシークレット生成
    db_password = generate_random_key(32)
    jwt_secret = generate_random_key(64)
    db_config = generate_db_config(db_password)

    # 1. db.env (PostgreSQL コンテナ用)
    db_env_content = f"""
POSTGRES_DB=acp
POSTGRES_USER=acp
POSTGRES_PASSWORD={db_password}
"""
    create_env_file("db.env", db_env_content)

    # 2. auth.env (Auth サービス用)
    auth_env_content = db_config + f"JWT_SECRET={jwt_secret}\n"
    create_env_file("auth.env", auth_env_content)

    # 3. build.env (Build サービス用)
    build_env_content = db_config
    create_env_file("build.env", build_env_content)

    # 4. deploy.env (Deploy サービス用)
    deploy_env_content = db_config
    create_env_file("deploy.env", deploy_env_content)

    print(f"\n--- 設定完了！ ---")
    print(f"設定ファイルがすべて '{os.path.abspath('.')}' ディレクトリに個別に生成されました。")

if __name__ == "__main__":
    main()
