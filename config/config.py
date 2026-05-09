import base64
import json
import os
import secrets
from urllib.parse import urlparse


def generate_random_key(length=64):
    return secrets.token_urlsafe(length)


def _input_with_default(prompt, default):
    value = input(f"{prompt} (デフォルト: {default}): ").strip()
    return value if value else default


def _ask_yes_no(prompt):
    return input(prompt).strip().lower() == 'y'


def _append_optional_env(template, harbor_url, harbor_project,
                         harbor_username=None, harbor_password=None,
                         k8s_namespace=None):
    if harbor_url:
        template += f'\nHARBOR_URL = "{harbor_url}"\nHARBOR_PROJECT = "{harbor_project}"\n'
        if harbor_username is not None:
            template += f'HARBOR_USERNAME = "{harbor_username}"\nHARBOR_PASSWORD = "{harbor_password}"\n'
    if k8s_namespace:
        template += f'\nK8S_NAMESPACE = "{k8s_namespace}"\n'
    return template


def get_oauth_credentials(provider_name):
    print(f"\n--- {provider_name} OAuth 設定 ---")
    if _ask_yes_no(f"{provider_name} の設定をしますか？ (y/n): "):
        client_id = input(f"{provider_name} のクライアントIDを入力してください: ")
        client_secret = input(f"{provider_name} のクライアントシークレットを入力してください: ")
        return client_id, client_secret
    return "", ""


def get_admin_credentials():
    print("\n--- 管理者アカウントの設定 ---")
    admin_email = input("管理者のメールアドレスを入力してください: ")
    admin_password = generate_random_key(32)
    return admin_email, admin_password


def confirm_overwrite_all(files_to_check):
    existing_files = [f for f in files_to_check if os.path.exists(f)]

    if existing_files:
        print("\n--- ファイルの上書き確認 ---")
        print(f"以下のファイルが既に存在します: {', '.join(existing_files)}")
        if not _ask_yes_no("これらのファイルをすべて上書きしますか？ (y/n): "):
            print("ファイルの生成を中止しました。")
            return False
    return True


def create_env_file(file_path, content):
    with open(file_path, "w", encoding="utf-8") as file:
        file.write(content.strip())
    print(f"✅ ファイル '{file_path}' を生成しました。")


def get_db_config():
    """
    PostgreSQL の接続情報を収集して返します。
    返り値: (auth_dsn, app_dsn, task_dsn)
    """
    print("\n--- データベース設定 ---")
    host = _input_with_default("DB ホスト", "db")
    port = _input_with_default("DB ポート", "5432")

    print("\n  [認証 DB (authdb)]")
    auth_user = _input_with_default("  ユーザー名", "main")
    auth_pass = _input_with_default("  パスワード", "main")
    auth_db   = _input_with_default("  データベース名", "authdb")

    print("\n  [メイン DB (maindb)]")
    app_user = _input_with_default("  ユーザー名", "main")
    app_pass = _input_with_default("  パスワード", "main")
    app_db   = _input_with_default("  データベース名", "maindb")

    print("\n  [タスクキュー DB (taskdb) — River ジョブキュー用]")
    task_user = _input_with_default("  ユーザー名", "task_user")
    task_pass = _input_with_default("  パスワード", "task_pass")
    task_db   = _input_with_default("  データベース名", "taskdb")

    base = f"host={host} port={port} sslmode=disable TimeZone=Asia/Tokyo"
    auth_dsn = f"{base} user={auth_user} password={auth_pass} dbname={auth_db}"
    app_dsn  = f"{base} user={app_user} password={app_pass} dbname={app_db}"
    task_dsn = f"host={host} port={port} sslmode=disable user={task_user} password={task_pass} dbname={task_db}"

    return auth_dsn, app_dsn, task_dsn


def create_auth_env(auth_dsn):
    discord_client_id, discord_client_secret = get_oauth_credentials("Discord")
    google_client_id, google_client_secret = get_oauth_credentials("Google")
    github_client_id, github_client_secret = get_oauth_credentials("Github")
    microsoft_client_id, microsoft_client_secret = get_oauth_credentials("Microsoft")

    admin_email, admin_password = get_admin_credentials()

    token_secret_key = generate_random_key()
    admin_session_key = generate_random_key()

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

DB_TYPE = "postgres"
DB_DSN = "{auth_dsn}"

TOKEN_SECRET = {token_secret_key}
ADMIN_SESSION_KEY = {admin_session_key}

GRPC_ADDR = ":9000"
CUSTOM_SCHEME = "authbase"
"""
    create_env_file("auth.env", auth_env_template)


def get_service_ports():
    print("\n--- マイクロサービスのポート設定 ---")
    app_port     = _input_with_default("app サービスのポート", "8090")
    builder_port = _input_with_default("builder サービスのポート", "8091")
    watcher_port = _input_with_default("watcher サービスのポート", "8092")
    return app_port, builder_port, watcher_port


def get_harbor_config():
    print("\n--- Harbor レジストリ設定 ---")
    if not _ask_yes_no("Harbor を設定しますか？ (y/n, デフォルト: n): "):
        return "", "", "", ""

    harbor_url      = input("Harbor URL (例: https://harbor.example.com): ").strip()
    harbor_project  = _input_with_default("Harbor プロジェクト名", "library")
    harbor_username = input("Harbor ユーザー名: ").strip()
    harbor_password = input("Harbor パスワード: ").strip()
    return harbor_url, harbor_project, harbor_username, harbor_password


def get_k8s_config():
    print("\n--- Kubernetes 設定 ---")
    if not _ask_yes_no("Kubernetes を設定しますか？ (y/n, デフォルト: n): "):
        return ""
    return _input_with_default("Kubernetes namespace", "default")


def create_app_env(app_dsn, task_dsn, app_port, builder_port, watcher_port, harbor_url, harbor_project, k8s_namespace):
    session_secret_key = generate_random_key()
    template = f"""SessionSecret = "{session_secret_key}"
GRPC_SERVER = auth:9000
DATABASE_TYPE = "postgres"
DATABASE_DSN = "{app_dsn}"
TASK_DATABASE_DSN = "{task_dsn}"
APP_PORT = {app_port}
BUILDER_PORT = {builder_port}
WATCHER_PORT = {watcher_port}
"""
    template = _append_optional_env(template, harbor_url, harbor_project, k8s_namespace=k8s_namespace)
    create_env_file("app.env", template)


def create_builder_env(app_dsn, task_dsn, builder_port, harbor_url, harbor_project, harbor_username, harbor_password):
    template = f"""DATABASE_TYPE = "postgres"
DATABASE_DSN = "{app_dsn}"
TASK_DATABASE_DSN = "{task_dsn}"
BUILDER_PORT = {builder_port}
"""
    template = _append_optional_env(template, harbor_url, harbor_project,
                                    harbor_username=harbor_username, harbor_password=harbor_password)
    create_env_file("builder.env", template)


def create_watcher_env(app_dsn, watcher_port, harbor_url, harbor_project, k8s_namespace):
    template = f"""DATABASE_TYPE = "postgres"
DATABASE_DSN = "{app_dsn}"
WATCHER_PORT = {watcher_port}
"""
    template = _append_optional_env(template, harbor_url, harbor_project, k8s_namespace=k8s_namespace)
    create_env_file("watcher.env", template)


def create_controller_env(app_dsn, task_dsn, harbor_url, harbor_project, k8s_namespace):
    template = f"""DATABASE_TYPE = "postgres"
DATABASE_DSN = "{app_dsn}"
TASK_DATABASE_DSN = "{task_dsn}"
"""
    template = _append_optional_env(template, harbor_url, harbor_project, k8s_namespace=k8s_namespace)
    create_env_file("controller.env", template)


def _create_docker_config(harbor_url, harbor_username, harbor_password):
    docker_config_dir = os.path.expanduser("~/.docker")
    os.makedirs(docker_config_dir, exist_ok=True)

    harbor_host = urlparse(harbor_url).netloc
    auth_str = base64.b64encode(f"{harbor_username}:{harbor_password}".encode()).decode()

    docker_config = {
        "auths": {
            harbor_host: {
                "auth": auth_str
            }
        }
    }

    config_path = os.path.join(docker_config_dir, "config.json")
    with open(config_path, "w", encoding="utf-8") as f:
        json.dump(docker_config, f, indent=2)
    print(f"✅ Docker config '{docker_config_dir}/config.json' を生成しました。")


def main():
    data_dir = "./data"
    os.makedirs(data_dir, exist_ok=True)
    os.chdir(data_dir)

    print("--- OAuth およびアプリケーション設定の開始 ---")

    files_to_check = ["auth.env", "app.env", "builder.env", "watcher.env", "controller.env"]
    if not confirm_overwrite_all(files_to_check):
        return

    auth_dsn, app_dsn, task_dsn = get_db_config()
    app_port, builder_port, watcher_port = get_service_ports()
    harbor_url, harbor_project, harbor_username, harbor_password = get_harbor_config()
    k8s_namespace = get_k8s_config()

    create_auth_env(auth_dsn)
    create_app_env(app_dsn, task_dsn, app_port, builder_port, watcher_port, harbor_url, harbor_project, k8s_namespace)
    create_builder_env(app_dsn, task_dsn, builder_port, harbor_url, harbor_project, harbor_username, harbor_password)
    create_watcher_env(app_dsn, watcher_port, harbor_url, harbor_project, k8s_namespace)
    create_controller_env(app_dsn, task_dsn, harbor_url, harbor_project, k8s_namespace)

    if harbor_url and harbor_username and harbor_password:
        _create_docker_config(harbor_url, harbor_username, harbor_password)

    print("\n--- 設定完了！ ---")
    print("設定ファイルがすべて './data' ディレクトリに生成されました。")


if __name__ == "__main__":
    main()
