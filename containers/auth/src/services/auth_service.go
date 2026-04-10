package services

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
func NewAuthService() AuthService {
	return &authService{}
}

func (service *authService) Login(username string, password string) (string, error) {
	// 実装はユーザーが行う
	return "", nil
}

func (service *authService) ValidateToken(token string) (bool, error) {
	// 実装はユーザーが行う
	return true, nil
}

func (service *authService) IssueInternalToken(serviceName string) (string, error) {
	// 実装はユーザーが行う
	return "", nil
}
