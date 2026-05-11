package harbor

import (
	"crypto/rand"
	"math/big"
)

const (
	lowerChars   = "abcdefghijklmnopqrstuvwxyz"
	upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars   = "0123456789"
	specialChars = "!@#$%^&*"
	allChars     = lowerChars + upperChars + digitChars + specialChars
	passwordLen  = 24
)

// generatePassword は Harbor のパスワードポリシー
// (大文字・小文字・数字・記号を各 1 文字以上含む) を満たす
// ランダムパスワードを生成します。
func generatePassword() string {
	mustOne := []string{lowerChars, upperChars, digitChars, specialChars}
	buf := make([]byte, passwordLen)

	// 各文字種から最低 1 文字ずつ確保
	for i, charset := range mustOne {
		buf[i] = randomChar(charset)
	}
	// 残りをランダムに埋める
	for i := len(mustOne); i < passwordLen; i++ {
		buf[i] = randomChar(allChars)
	}
	// Fisher–Yates シャッフル
	for i := passwordLen - 1; i > 0; i-- {
		j := cryptoRandInt(i + 1)
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func randomChar(charset string) byte {
	return charset[cryptoRandInt(len(charset))]
}

func cryptoRandInt(n int) int {
	max := big.NewInt(int64(n))
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic("crypto/rand 失敗: " + err.Error())
	}
	return int(v.Int64())
}
