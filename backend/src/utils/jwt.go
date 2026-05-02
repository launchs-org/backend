package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JobTokenClaim struct {
	JobID string	//実行されるJOBID
	ImageName	string	//イメージ名
	ImageTag 	string	//イメージのタグ
}

// トークンを生成する
func GenerateJobToken(claim JobTokenClaim) (string,error) {
	// トークンの情報を生成
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"JobID" : claim.JobID,
		"ImageTag" : claim.ImageTag,
		"ImageName" : claim.ImageName,
		"exp":       time.Now().Add(time.Minute * 10).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(os.Getenv("SessionSecret")))

	return tokenString,err
}

func VerifyJobToken(tokenString string) (JobTokenClaim, error) {
	var claim JobTokenClaim
	// 署名鍵の取得（生成時と同じもの）
	secretKey := []byte(os.Getenv("SessionSecret"))

	// 1. パースと署名アルゴリズムの検証
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// アルゴリズムが HS512 かどうかを確認
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secretKey, nil
	})

	if err != nil {
		return claim, err // 有効期限切れ (exp) もここで err として返ります
	}

	// 2. クレームの抽出
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// 各フィールドを文字列として取り出す（型アサーション）
		// 万が一 key が存在しない場合を考慮し、ok イディオムを使うのが安全です
		if v, ok := claims["JobID"].(string); ok {
			claim.JobID = v
		}
		if v, ok := claims["ImageName"].(string); ok {
			claim.ImageName = v
		}
		if v, ok := claims["ImageTag"].(string); ok {
			claim.ImageTag = v
		}

		return claim, nil
	}

	return claim, jwt.ErrSignatureInvalid
}