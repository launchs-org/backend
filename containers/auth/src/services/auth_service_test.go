package services

import (
	"os"
	"testing"
	"golang.org/x/crypto/bcrypt"
)

func TestLogin(t *testing.T) {
	password := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(password)); err != nil {
		t.Errorf("パスワードの検証に失敗しました: %v", err)
	}
}

// TestSignUp サインアップ機能のテスト
func TestSignUp(t *testing.T) {
	// 1. 環境変数が設定されていない場合（許可されていない状態）
	os.Setenv("ALLOW_SIGNUP", "false")
	service := &authService{} // モックDBは必要だが、ALLOW_SIGNUPチェックが先なので簡易的にテスト
	err := service.SignUp("testuser", "pass", "test@example.com")
	if err == nil || err.Error() != "サインアップは現在無効です" {
		t.Errorf("許可されていないはずのサインアップが成功しました: %v", err)
	}
}
