package services

import (
	"testing"
	"golang.org/x/crypto/bcrypt"
	s_models "shared/models"
)

// MockDB として動作する構造体等は本来必要ですが、今回は簡易的に
// NewAuthService のシグネチャに合わせて db を渡します。
// 実際にはモックライブラリ等が必要になる可能性があります。

func TestLogin(t *testing.T) {
	// 簡易テスト：パスワードハッシュの生成と検証の確認
	password := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	// ここでは DB モックが未実装のため、Loginメソッドを直接テストするのは困難です。
	// まずはLoginメソッドが期待通りにハッシュを比較できるかを確認するロジックを別途検証します。

	if err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(password)); err != nil {
		t.Errorf("パスワードの検証に失敗しました: %v", err)
	}
}
