package services

import (
	repo "auth/models"
	s_models "shared/models"
)

// AuthService 認証とトークン発行に関するビジネスロジックを定義するインターフェース
type AuthService interface {
	Login(username string, password string) (string, error)
	ValidateToken(token string) (bool, error)
	IssueInternalToken(serviceName string) (string, error)
}

// authService AuthService の実装
type authService struct {
}

// NewAuthService AuthService の新しいインスタンスを作成する
func NewAuthService(db *s_models.Database) AuthService {
	return &authService{}
}

// Login ユーザー名とパスワードを検証し、JWT トークンを発行します。
func (service *authService) Login(username string, password string) (string, error) {
	user, err := repo.GetUserByUsername(username)
	if err != nil {
		return "", err
	}
	// パスワード検証ロジックなどは後ほど
	_ = user
	return "jwt-token", nil
}

// ValidateToken 送信されたトークンの妥当性を検証します。
func (service *authService) ValidateToken(token string) (bool, error) {
	// バリデーションロジックは後ほど
	return true, nil
}

// IssueInternalToken マイクロサービス間で利用する内部認証用トークンを発行します。
func (service *authService) IssueInternalToken(serviceName string) (string, error) {
	// 実装はユーザーが行う
	return "", nil
}
