# ISSUE-074 CLIトークン発行・管理機能

## 親 Issue
なし

## 概要

このAPI向けにCLIツールを提供したいが、現状の認証はブラウザ向けJWT（外部authサービスが発行、handlerはEd25519公開鍵で検証のみ）しかなく、CLIが使える認証手段が存在しない。

grillingセッションでの検討の結果、以下が判明・決定した:

- handler（backend-mini）は現状「JWT検証専門」であり、ブラウザ用JWTの署名鍵（`JWT_PRIVATE_KEY`）はauth側サービスにのみ配布されている。handler側で流用できない。
- CLIツールの利用シーンは「手元での対話利用のみ」（CI/CD・自動化は今回スコープ外）。
- 将来的にはブラウザ認可コードフロー（PKCE付きOAuth的なもの）も欲しいが、それは別途authサービス側の対応が必要な将来課題であり、今回の設計はそれを妨げない形にする。
- 上記制約から、CLIトークンは**handler自身が保有する専用の鍵ペア**で署名・検証する、ブラウザ用JWTとは独立したトークン系統として実装する。
- CLIトークンは長期利用を前提としつつ、漏洩時に個別失効できる必要があるため、JWTに`jti`を持たせてDBで有効性を管理する（ステートレスなブラウザJWTとは異なり、CLIトークンの検証だけDB照合を伴う）。
- 発行・一覧・失効はWeb管理画面（ブラウザ用JWTで保護された既存の`RequireAuth`エンドポイント）からのみ行う。CLIトークン自身でCLIトークンを管理させない（鶏卵問題の回避）。
- CLIツール本体（Goバイナリ等）の実装は今回のスコープ外。トークンはユーザーがWeb画面上でコピーし、CLI側の設定ファイルに手動で貼り付ける前提。

実装内容:

1. CLIトークン専用のEd25519鍵ペアを生成し、handlerに`kid`付きで配布・ロードする
2. `cli_tokens`テーブルを新設し、発行済みトークンのメタ情報（jti, user_id, name, expires_at, revoked_at）を管理する
3. CLIトークン発行・一覧・失効のCRUD API（ブラウザJWTで保護）を実装する
4. `RequireAuth`ミドルウェアを拡張し、JWTヘッダの`kid`を見てCLI用トークンの場合はCLI用公開鍵で検証+DBでjtiの有効性を照合する

## 変更ファイル一覧

- `openssl/cliTokenKeys/genkey.sh`（新規作成）
    - **何を**: 既存の`openssl/genkey.sh`に倣い、CLIトークン専用のEd25519鍵ペアを生成し`CLI_TOKEN_PRIVATE_KEY`/`CLI_TOKEN_PUBLIC_KEY`としてenvファイルに出力するスクリプト
    - **なぜ**: ブラウザ用JWTの鍵とは独立した、handler自身が署名・検証を完結できる鍵ペアが必要なため

- `config/config.py`（編集）
    - **何を**: `main()`の初回セットアップ処理に、CLIトークン専用のEd25519鍵ペア（`CLI_TOKEN_PRIVATE_KEY`/`CLI_TOKEN_PUBLIC_KEY`）を生成し`app.env`テンプレートに書き出す処理を追加する
    - **なぜ**: 初回セットアップ時に他の設定（DB・Harbor等）と同様、CLIトークン鍵もまとめて対話式セットアップの一部として生成できるようにするため

- `docker-compose.yaml`（編集）
    - **何を**: handlerサービスの`env_file`にCLIトークン用の鍵envを追加する
    - **なぜ**: handlerがCLIトークンの署名・検証の両方を自己完結して行うため

- `handler/src/models/cli_token.go`（新規作成）
    - **何を**: `CliToken`モデル定義。`ID`（jti, UUID主キー）, `UserID`, `Name`（用途ラベル）, `ExpiresAt`（nullable, nullなら無期限）, `RevokedAt`（nullable）, `CreatedAt`。`TableName()`で`cli_tokens`を返す
    - **なぜ**: 発行済みCLIトークンの有効性をDBで管理し、個別失効を可能にするため

- `handler/src/repository/cli_token_repository.go`（新規作成）
    - **何を**: `CliTokenRepository`インターフェースと実装。`Create`・`FindByID`（jtiで取得）・`FindAllByUserID`・`Revoke`（revoked_atを設定）メソッドを持つ
    - **なぜ**: CLIトークンのDB操作を既存パターン（webhook_repository.go等）に倣い抽象化するため

- `handler/src/service/cli_token_service.go`（新規作成）
    - **何を**: `CliTokenService`インターフェースと実装。`IssueToken`（jti生成・DBレコード作成・JWT署名して平文トークンを一度だけ返す）・`ListTokens`（ユーザーの発行済みトークン一覧、平文トークンは含まない）・`RevokeToken`（所有者チェック後にrevoked_atを設定）を実装する
    - **なぜ**: トークン発行・署名・DB管理のビジネスロジックをハンドラーから分離するため

- `handler/src/handler/cli_token_handler.go`（新規作成）
    - **何を**: `CreateCliToken`（POST、有効期限を指定可能・無期限も許可）・`ListCliTokens`（GET）・`RevokeCliToken`（DELETE）ハンドラーの実装
    - **なぜ**: HTTPリクエストの受け取りとレスポンス返却を担う層が必要なため

- `handler/src/middlewares/cli_jwt.go`（新規作成）
    - **何を**: CLIトークン専用公開鍵のロード、CLI用JWTの署名関数（`IssueCliToken`）、検証関数（`ValidateCliToken`）。claimには`jti`・`userID`を含む
    - **なぜ**: ブラウザ用JWT検証ロジック（jwt.go）と鍵・署名主体が異なるため分離する

- `handler/src/middlewares/auth.go`（編集）
    - **何を**: `RequireAuth`内でJWTヘッダの`kid`を見て、CLI用トークンであれば`ValidateCliToken`で検証しCliTokenRepositoryでjtiの有効性（revoked_at IS NULL かつ 期限内）を照合する分岐を追加する。ブラウザ用JWTの検証パス（既存の`ValidateToken`）には手を加えない
    - **なぜ**: 同一の認証必須エンドポイント群に対して、ブラウザJWT・CLIトークンの両方を受け付けられるようにするため

- `handler/src/middlewares/init.go`（編集）
    - **何を**: `Init()`内でCLIトークン用公開鍵（`CLI_TOKEN_PUBLIC_KEY`）も併せてロードする
    - **なぜ**: サーバー起動時に検証用の鍵をすべて準備しておく必要があるため

- `handler/src/main.go`（編集）
    - **何を**: `CliTokenRepository`・`CliTokenService`・`CliTokenHandler`のDI組み立てを追加する
    - **なぜ**: 新規ハンドラーを既存の手動DIパターンに接続するため

- `handler/src/router/router.go`（編集）
    - **何を**: `POST /api/v1/cli-tokens`・`GET /api/v1/cli-tokens`・`DELETE /api/v1/cli-tokens/:id`を、既存の`RequireAuth`のみのグループ（ブラウザJWT前提）に登録する
    - **なぜ**: CLIトークンの発行・管理はブラウザでログイン済みのユーザーのみが行えるようにするため

## テスト確認項目

- [ ] `POST /api/v1/cli-tokens`でCLIトークンが発行され、レスポンスに平文トークンが一度だけ含まれること
- [ ] 発行時に有効期限（無期限 or 期限指定）を選択できること
- [ ] `GET /api/v1/cli-tokens`で自分の発行済みトークン一覧が取得できること（平文トークンは含まれない）
- [ ] `DELETE /api/v1/cli-tokens/:id`でトークンが失効し、`revoked_at`が設定されること
- [ ] 他ユーザーのCLIトークンを`DELETE`すると403が返ること
- [ ] 発行されたCLIトークンをAuthorizationヘッダに付けて既存の認証必須エンドポイントにアクセスでき、`RequireAuth`を通過すること
- [ ] 失効済み（revoked_at設定済み）のCLIトークンでアクセスすると401が返ること
- [ ] 期限切れのCLIトークンでアクセスすると401が返ること
- [ ] ブラウザ用JWT（kidなし、または異なるkid）は従来通り`ValidateToken`で検証され、CLIトークン用の分岐に影響されないこと
- [ ] CLIトークンを使って`POST /api/v1/cli-tokens`（CLIトークンの発行）を試みると拒否されること（鶏卵問題の回避確認）

### repository 層テスト

- [ ] `CliTokenRepository.Create`でレコードが作成できること
- [ ] `CliTokenRepository.FindByID`でjti指定の取得ができること
- [ ] `CliTokenRepository.Revoke`で`revoked_at`が更新されること
