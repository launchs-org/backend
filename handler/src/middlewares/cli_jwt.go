package middlewares

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"handler/logger"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// cliTokenKid は CLI 用 JWT のヘッダに埋め込む鍵識別子
const cliTokenKid = "cli"

var (
	cliTokenPrivKey ed25519.PrivateKey // CLI トークン署名用秘密鍵
	cliTokenPubKey  ed25519.PublicKey  // CLI トークン検証用公開鍵
)

// CliTokenClaim は CLI 用 JWT のクレーム
type CliTokenClaim struct {
	UserID string `json:"userID"` // ユーザーID
	jwt.RegisteredClaims
}

// InitCliToken は CLI トークン用の鍵ペアを環境変数からロードする
func InitCliToken() {
	// 秘密鍵をロードする
	privBlock, _ := pem.Decode([]byte(os.Getenv("CLI_TOKEN_PRIVATE_KEY")))
	if privBlock == nil {
		logger.PrintErr("CLIトークン用秘密鍵のPEMデータの解析に失敗しました")
		return
	}
	privKey, err := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
	if err != nil {
		logger.PrintErr("CLIトークン用秘密鍵のパースに失敗しました: %v", err)
		return
	}
	edPrivKey, ok := privKey.(ed25519.PrivateKey)
	if !ok {
		logger.PrintErr("CLIトークン用の鍵はEd25519秘密鍵ではありません")
		return
	}

	// 公開鍵をロードする
	pubBlock, _ := pem.Decode([]byte(os.Getenv("CLI_TOKEN_PUBLIC_KEY")))
	if pubBlock == nil {
		logger.PrintErr("CLIトークン用公開鍵のPEMデータの解析に失敗しました")
		return
	}
	pubKeyRaw, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		logger.PrintErr("CLIトークン用公開鍵のパースに失敗しました: %v", err)
		return
	}
	edPubKey, ok := pubKeyRaw.(ed25519.PublicKey)
	if !ok {
		logger.PrintErr("CLIトークン用の鍵はEd25519公開鍵ではありません")
		return
	}

	cliTokenPrivKey = edPrivKey
	cliTokenPubKey = edPubKey
	logger.Println("CLIトークン用Ed25519鍵の読み込みに成功しました")
}

// SetCliTokenKeysForTest はテストコードから CLI トークン用の鍵ペアを直接設定するためのヘルパー
func SetCliTokenKeysForTest(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) {
	cliTokenPrivKey = privKey
	cliTokenPubKey = pubKey
}

// IssueCliToken は jti・userID・有効期限を指定してCLI用JWTを発行する
func IssueCliToken(jti string, userID string, expiresAt *time.Time) (string, error) {
	claim := CliTokenClaim{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:       jti, // jtiクレームにトークンIDを設定する
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	if expiresAt != nil {
		claim.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(*expiresAt) // 有効期限を設定する（nilなら無期限）
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claim)
	token.Header["kid"] = cliTokenKid // CLIトークンであることをkidで明示する

	return token.SignedString(cliTokenPrivKey)
}

// ValidateCliToken はCLI用JWTを検証し、クレームを返す
func ValidateCliToken(tokenString string) (CliTokenClaim, error) {
	logger.Println("CLIトークンを検証します")

	claim := CliTokenClaim{}
	token, err := jwt.ParseWithClaims(tokenString, &claim, func(token *jwt.Token) (interface{}, error) {
		return cliTokenPubKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))

	if err != nil {
		return CliTokenClaim{}, err
	}
	if !token.Valid {
		return CliTokenClaim{}, errors.New("CLIトークンが無効です")
	}

	return claim, nil
}

// IsCliTokenHeader はJWTのkidヘッダがCLIトークンを示しているか判定する
func IsCliTokenHeader(tokenString string) bool {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return false
	}
	kid, ok := token.Header["kid"].(string)
	return ok && kid == cliTokenKid
}
