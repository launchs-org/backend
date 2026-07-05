#!/bin/sh

# ディレクトリとファイルパスの定義
cliKeysDir="/clitokenkeys"
privatePath="$cliKeysDir/private.key"
publicPath="$cliKeysDir/public.key"
privateEnvPath="$cliKeysDir/private.env"
publicEnvPath="$cliKeysDir/public.env"

# ディレクトリが存在しない場合は作成
mkdir -p "$cliKeysDir"

echo "CLIトークン鍵は $cliKeysDir に保存します"

# Ed25519鍵の処理
privateKeyCreated=0

if [ ! -f "$privatePath" ]; then
    echo "CLIトークン用Ed25519秘密鍵が存在しないため、新しい鍵を生成します..."
    openssl genpkey -algorithm ED25519 -out "$privatePath"
    privateKeyCreated=1
    echo "CLIトークン用Ed25519秘密鍵の生成が完了しました"
else
    echo "既存のCLIトークン用Ed25519秘密鍵を使用します"
fi

# 秘密鍵が新しく作られたか、公開鍵が存在しない場合に公開鍵を生成
if [ $privateKeyCreated -eq 1 ] || [ ! -f "$publicPath" ]; then
    echo "CLIトークン用Ed25519公開鍵を生成します..."
    openssl pkey -in "$privatePath" -pubout -out "$publicPath"
    echo "CLIトークン用Ed25519公開鍵の生成が完了しました"
fi

# Ed25519鍵の内容を.envファイルとして出力（末尾のバックスラッシュを削除）
key_content=$(cat "$privatePath" | tr '\n' '\\' | sed 's/\\$//' | sed 's/\\/\\n/g')
echo "CLI_TOKEN_PRIVATE_KEY=\"$key_content\"" > "$privateEnvPath"

key_content=$(cat "$publicPath" | tr '\n' '\\' | sed 's/\\$//' | sed 's/\\/\\n/g')
echo "CLI_TOKEN_PUBLIC_KEY=\"$key_content\"" > "$publicEnvPath"

echo "鍵情報を $privateEnvPath と $publicEnvPath に .env 形式で出力しました"

echo "\n===== CLIトークン用Ed25519鍵の情報 ====="
openssl pkey -in "$privatePath" -text -noout
