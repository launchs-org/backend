package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidPadding はPKCS#7パディングが不正な場合のエラー
var ErrInvalidPadding = errors.New("パディングが不正です（鍵が間違っている可能性があります）")

// EncryptArchive は plainData を AES-256-CBC で暗号化し、
// iv(16byte) || ciphertext の形式で返す。
// 併せて暗号化鍵の16進文字列と、暗号文全体(iv||ciphertext)のSHA256ハッシュの16進文字列を返す。
// SHA256ハッシュは file.io からのダウンロード結果が破損・改竄されていないかを
// builder 側で確認するためのものであり、秘密鍵によるMAC(HMAC)ではなく無鍵ハッシュで十分と判断している。
func EncryptArchive(plainData []byte) (encoded []byte, encKeyHex string, sha256Hex string, err error) {
	encKey := make([]byte, 32) // AES-256
	if _, err = io.ReadFull(rand.Reader, encKey); err != nil {
		return nil, "", "", fmt.Errorf("暗号化鍵の生成に失敗しました: %w", err)
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("AESブロック暗号の生成に失敗しました: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, "", "", fmt.Errorf("IVの生成に失敗しました: %w", err)
	}

	paddedData := pkcs7Pad(plainData, aes.BlockSize) // CBCモードはブロックサイズ境界のパディングが必須

	ciphertext := make([]byte, len(paddedData))
	cbcEncrypter := cipher.NewCBCEncrypter(block, iv)
	cbcEncrypter.CryptBlocks(ciphertext, paddedData)

	encoded = append(append([]byte{}, iv...), ciphertext...) // iv || ciphertext

	hash := sha256.Sum256(encoded) // 改竄・破損検知用の無鍵ハッシュ

	return encoded, hex.EncodeToString(encKey), hex.EncodeToString(hash[:]), nil
}

// DecryptArchive は EncryptArchive で暗号化されたバイト列を復号する。
// Go側の往復テスト用途であり、実運用でのbuilder側復号はopenssl CLIで行う。
func DecryptArchive(encoded []byte, encKeyHex string) ([]byte, error) {
	encKey, err := hex.DecodeString(encKeyHex)
	if err != nil {
		return nil, fmt.Errorf("暗号化鍵のデコードに失敗しました: %w", err)
	}

	if len(encoded) < aes.BlockSize {
		return nil, errors.New("暗号文が短すぎます")
	}
	iv := encoded[:aes.BlockSize]
	ciphertext := encoded[aes.BlockSize:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("暗号文の長さがブロックサイズの倍数ではありません")
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("AESブロック暗号の生成に失敗しました: %w", err)
	}

	paddedData := make([]byte, len(ciphertext))
	cbcDecrypter := cipher.NewCBCDecrypter(block, iv)
	cbcDecrypter.CryptBlocks(paddedData, ciphertext)

	return pkcs7Unpad(paddedData)
}

// pkcs7Pad はPKCS#7方式でデータをブロックサイズの倍数までパディングする
func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padding...)
}

// pkcs7Unpad はPKCS#7パディングを除去する
func pkcs7Unpad(data []byte) ([]byte, error) {
	dataLen := len(data)
	if dataLen == 0 {
		return nil, ErrInvalidPadding
	}
	padLen := int(data[dataLen-1])
	if padLen == 0 || padLen > dataLen || padLen > aes.BlockSize {
		return nil, ErrInvalidPadding
	}
	for _, paddingByte := range data[dataLen-padLen:] {
		if int(paddingByte) != padLen {
			return nil, ErrInvalidPadding
		}
	}
	return data[:dataLen-padLen], nil
}
