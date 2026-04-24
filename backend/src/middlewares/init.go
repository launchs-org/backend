package middlewares

import (
	"backend/logger"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
)

var (
	pubKey ed25519.PublicKey
)

func Init() {
	// PEMブロックの解析
	block, _ := pem.Decode([]byte(os.Getenv("JWT_PUBLIC_KEY")))
	if block == nil {
		logger.PrintErr("PEMデータの解析に失敗しました")
		return
	}

	// 公開鍵のパース
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logger.PrintErr("公開鍵のパースに失敗しました: %v", err)
		return
	}

	// Ed25519公開鍵の型アサーション
	edPublicKey, ok := publicKey.(ed25519.PublicKey)
	if !ok {
		logger.PrintErr("キーはEd25519公開鍵ではありません")
	}

	// 公開鍵の情報を表示
	logger.Println("Ed25519公開鍵の読み込みに成功しました\n")
	logger.Println("公開鍵の長さ: %d バイト\n", len(edPublicKey))
	logger.Println("公開鍵の値: %v\n", edPublicKey)

	pubKey = edPublicKey
}
