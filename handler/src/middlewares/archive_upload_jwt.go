package middlewares

import (
	"errors"
	"handler/logger"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// archiveUploadTokenKid はアーカイブアップロードトークンのヘッダに埋め込む鍵識別子
const archiveUploadTokenKid = "archive-upload"

// archiveUploadTokenExpiry はアップロードトークンの有効期限（発行時刻からの相対時間）
const archiveUploadTokenExpiry = 15 * time.Minute

var archiveUploadTokenSecret []byte // アーカイブアップロードトークン署名用の共有シークレット

// ArchiveUploadTokenClaim はアーカイブアップロードトークンのクレーム。
// zip/tar.gz アップロード完了時に発行し、ビルド開始APIに渡すことで
// file.io のダウンロードリンクと復号鍵を安全に橋渡しする（DBには保存しない）。
type ArchiveUploadTokenClaim struct {
	DeploymentID string `json:"deployment_id"` // 対象デプロイメントID（ビルド開始時にパスパラメータと一致確認する）
	ArchiveURL   string `json:"archive_url"`   // file.io のダウンロードリンク
	EncKeyHex    string `json:"enc_key_hex"`   // AES復号鍵（16進）
	SHA256Hex    string `json:"sha256_hex"`    // 暗号文全体のSHA256ハッシュ（16進、破損・改竄検知用）
	FileName     string `json:"file_name"`     // 元ファイル名（DeploymentBuild記録用）
	SizeBytes    int64  `json:"size_bytes"`    // 元サイズ（DeploymentBuild記録用）
	jwt.RegisteredClaims
}

// InitArchiveUploadToken はアーカイブアップロードトークン用の共有シークレットを環境変数からロードする
func InitArchiveUploadToken() {
	secret := os.Getenv("ARCHIVE_UPLOAD_TOKEN_SECRET")
	if secret == "" {
		logger.PrintErr("ARCHIVE_UPLOAD_TOKEN_SECRETが設定されていません")
		return
	}
	archiveUploadTokenSecret = []byte(secret)
	logger.Println("アーカイブアップロードトークン用シークレットの読み込みに成功しました")
}

// SetArchiveUploadTokenSecretForTest はテストコードから共有シークレットを直接設定するためのヘルパー
func SetArchiveUploadTokenSecretForTest(secret []byte) {
	archiveUploadTokenSecret = secret
}

// IssueArchiveUploadToken はアーカイブアップロードトークンを発行する
func IssueArchiveUploadToken(claim ArchiveUploadTokenClaim) (string, error) {
	now := time.Now()
	claim.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),                              // 発行時刻を設定する
		ExpiresAt: jwt.NewNumericDate(now.Add(archiveUploadTokenExpiry)), // 有効期限を設定する（短命）
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	token.Header["kid"] = archiveUploadTokenKid // アーカイブアップロードトークンであることをkidで明示する

	return token.SignedString(archiveUploadTokenSecret)
}

// ValidateArchiveUploadToken はアーカイブアップロードトークンを検証し、クレームを返す
func ValidateArchiveUploadToken(tokenString string) (*ArchiveUploadTokenClaim, error) {
	claim := &ArchiveUploadTokenClaim{}
	token, err := jwt.ParseWithClaims(tokenString, claim, func(token *jwt.Token) (interface{}, error) {
		return archiveUploadTokenSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("アーカイブアップロードトークンが無効です")
	}

	return claim, nil
}
