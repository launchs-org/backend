package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestEncryptArchiveDecryptArchiveRoundTrip(t *testing.T) {
	// テストケースを定義する
	testCases := []struct {
		name      string
		plainData []byte
	}{
		{
			name:      "通常のバイト列を暗号化・復号できる",
			plainData: []byte("hello world, this is a test archive content"),
		},
		{
			name:      "ブロックサイズと同じ長さのバイト列を暗号化・復号できる",
			plainData: make([]byte, 16),
		},
		{
			name:      "空データを暗号化・復号できる",
			plainData: []byte{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, encKeyHex, sha256Hex, err := EncryptArchive(testCase.plainData) // 暗号化する
			if err != nil {
				t.Fatalf("EncryptArchiveが失敗しました: %v", err)
			}

			expectedHash := sha256.Sum256(encoded) // 暗号文自体のハッシュを計算する
			if hex.EncodeToString(expectedHash[:]) != sha256Hex {
				t.Fatalf("SHA256ハッシュが暗号文と一致しません")
			}

			decoded, err := DecryptArchive(encoded, encKeyHex) // 復号する
			if err != nil {
				t.Fatalf("DecryptArchiveが失敗しました: %v", err)
			}

			if string(decoded) != string(testCase.plainData) {
				t.Fatalf("復号結果が元データと一致しません: got=%q want=%q", decoded, testCase.plainData)
			}
		})
	}
}

func TestDecryptArchiveWithWrongKey(t *testing.T) {
	plainData := []byte("secret archive content")

	encoded, _, _, err := EncryptArchive(plainData) // 正しい鍵で暗号化する
	if err != nil {
		t.Fatalf("EncryptArchiveが失敗しました: %v", err)
	}

	wrongKeyHex := "00000000000000000000000000000000000000000000000000000000000000" // 誤った鍵（全ゼロ）

	decoded, decryptErr := DecryptArchive(encoded, wrongKeyHex) // 誤った鍵で復号を試みる
	if decryptErr == nil && string(decoded) == string(plainData) {
		t.Fatalf("誤った鍵で復号したにもかかわらず元データと一致してしまいました")
	}
}
