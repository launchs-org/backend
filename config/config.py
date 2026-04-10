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

def create_db_env(db_password):
    """
    db.env (PostgreSQL用) を生成する
    """
    db_env_content = f"""
POSTGRES_DB=acp
POSTGRES_USER=acp
POSTGRES_PASSWORD={db_password}
"""
    create_env_file("db.env", db_env_content)

def create_services_env(db_password, jwt_secret):
    """
    各マイクロサービス共通の services.env を生成する
    """
    services_env_content = f"""
DB_HOST=db
DB_USER=acp
DB_PASSWORD={db_password}
DB_NAME=acp
DB_PORT=5432
JWT_SECRET={jwt_secret}
"""
    create_env_file("services.env", services_env_content)

def main():
    """
    メイン処理：複数の設定ファイル生成関数を呼び出します。
    """
    # 背景: PostgreSQL を使用するマイクロサービス構成に合わせて env ファイルを生成するように変更

    # 作業ディレクトリを./dataに移動し、存在しなければ作成
    data_dir = "./data"
    os.makedirs(data_dir, exist_ok=True)
    os.chdir(data_dir)

    print("--- サービスの環境設定の生成を開始 ---")

    # ファイルの上書き確認を行い、許可されない場合は終了
    files_to_check = ["db.env", "services.env"]
    if not confirm_overwrite_all(files_to_check):
        return

    # ランダムなパスワードとシークレットを生成
    db_password = generate_random_key(32)
    jwt_secret = generate_random_key(64)

    # db.env ファイルを生成
    create_db_env(db_password)

    # services.env ファイルを生成
    create_services_env(db_password, jwt_secret)

    print(f"\n--- 設定完了！ ---")
    print(f"設定ファイルがすべて '{os.path.abspath('.')}' ディレクトリに生成されました。")
    print(f"Docker Compose 等でこれらのファイルを env_file として読み込んでください。")

if __name__ == "__main__":
    main()
