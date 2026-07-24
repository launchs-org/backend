import secrets
import os
import sys

def generate_random_key(length=64):
    """
    暗号学的に安全なランダムキーを、指定された長さ（デフォルト64文字）で生成します。
    """
    return secrets.token_urlsafe(length)

def get_oauth_credentials(provider_name):
    """
    指定されたプロバイダーのOAuthクライアントIDとシークレットをユーザーに入力させます。
    設定をスキップするオプションも提供します。
    """
    print(f"\n--- {provider_name} OAuth 設定 ---")
    response = input(f"{provider_name} の設定をしますか？ (y/n): ")
    if response.lower() == 'y':
        client_id = input(f"{provider_name} のクライアントIDを入力してください: ")
        client_secret = input(f"{provider_name} のクライアントシークレットを入力してください: ")
        return client_id, client_secret
    else:
        # 'n'が入力された場合は空の文字列を返す
        return "", ""

def get_admin_credentials():
    """
    管理者のメールアドレスを取得し、ランダムなパスワードを生成します。
    """
    print(f"\n--- 管理者アカウントの設定 ---")
    admin_email = input(f"管理者のメールアドレスを入力してください: ")
    admin_password = generate_random_key(32)  # 管理者パスワードは32文字で生成
    return admin_email, admin_password


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

def create_auth_env(db_type, db_dsn):
    """
    auth.env ファイルを生成するための設定情報を対話形式で取得し、ファイルに書き出します。
    """
    # 各OAuthプロバイダーの認証情報を対話形式で取得
    discord_client_id, discord_client_secret = get_oauth_credentials("Discord")
    google_client_id, google_client_secret = get_oauth_credentials("Google")
    github_client_id, github_client_secret = get_oauth_credentials("Github")
    microsoft_client_id, microsoft_client_secret = get_oauth_credentials("Microsoft")

    # 管理者アカウント情報を取得
    admin_email, admin_password = get_admin_credentials()

    # 認証とセッション用のランダムキーを自動生成（長さ64文字）
    token_secret_key = generate_random_key()
    admin_session_key = generate_random_key()
    bridge_token_secret_key = generate_random_key()

    # auth.env のテンプレート
    auth_env_template = f"""
DiscordClientID = {discord_client_id}
DiscordClientSecret = {discord_client_secret}
DiscordCallback = https://localhost:8947/auth/oauth/discord/callback

GoogleClientID = {google_client_id}
GoogleClientSecret = {google_client_secret}
GoogleCallback = https://localhost:8947/auth/oauth/google/callback

GithubClientID = {github_client_id}
GithubClientSecret = {github_client_secret}
GithubCallback = https://localhost:8947/auth/oauth/github/callback

MicrosoftClientID = {microsoft_client_id}
MicrosoftClientSecret = {microsoft_client_secret}
MicrosoftCallback = https://localhost:8947/auth/oauth/microsoftonline/callback

AdminEmail = "{admin_email}"
AdminPassword = "{admin_password}"

DB_TYPE = "{db_type}"
DB_DSN = "{db_dsn}"

TOKEN_SECRET = {token_secret_key}
ADMIN_SESSION_KEY = {admin_session_key}
BRIDGE_TOKEN_SECRET = {bridge_token_secret_key}

GRPC_ADDR = ":9000"
CUSTOM_SCHEME = "launchs-org"
LOGIN_REDIRECT_URL = /ui/
APP_NAME = "launchs-org"
"""
    create_env_file("auth.env", auth_env_template)

def create_env_file(file_path, content):
    """
    指定されたファイルパスに、指定された内容で設定ファイルを生成します。
    """
    with open(file_path, "w", encoding="utf-8") as file:
        file.write(content.strip())
    print(f"✅ ファイル '{file_path}' を生成しました。")

def get_harbor_credentials():
    """
    Harbor の接続情報をユーザーに入力させます。
    """
    print("\n--- Harbor 設定 ---")
    harbor_endpoint = input("Harbor のエンドポイントを入力してください (例: https://172.33.0.1): ")
    harbor_robot_name = input("Harbor 管理用 robot アカウント名（base64エンコード済み）を入力してください: ")
    harbor_robot_secret = input("Harbor 管理用 robot アカウントのシークレットを入力してください: ")
    return harbor_endpoint, harbor_robot_name, harbor_robot_secret


def main():
    """
    メイン処理：複数の設定ファイル生成関数を呼び出します。
    """
    # 作業ディレクトリを./dataに移動し、存在しなければ作成
    data_dir = "./data"
    os.makedirs(data_dir, exist_ok=True)
    os.chdir(data_dir)

    print("--- OAuth およびアプリケーション設定の開始 ---")

    # ファイルの上書き確認を行い、許可されない場合は終了
    files_to_check = ["auth.env", "app.env"]
    if not confirm_overwrite_all(files_to_check):
        return

    # Harbor の接続情報を取得
    harbor_endpoint, harbor_robot_name, harbor_robot_secret = get_harbor_credentials()

    # app.env のテンプレート
    app_env_template = f"""
DB_HOST = db
DB_USER = main
DB_PASSWORD = main
DB_NAME = maindb
DB_PORT = 5432

HARBOR_ENDPOINT = {harbor_endpoint}
HARBOR_ROBOT_NAME = {harbor_robot_name}
HARBOR_ROBOT_SECRET = {harbor_robot_secret}
"""

    # app.env ファイルを生成
    create_env_file("app.env", app_env_template)

    # auth.env ファイルを生成
    create_auth_env("postgres", "host=db user=main password=main dbname=authdb port=5432 sslmode=disable TimeZone=Asia/Tokyo")

    print(f"\n--- 設定完了！ ---")
    print(f"設定ファイルがすべて './data' ディレクトリに生成されました。")

if __name__ == "__main__":
    main()
