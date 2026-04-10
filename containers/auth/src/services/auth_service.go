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

// Login ユーザー名とパスワードを検証し、JWT トークンを発行します。
func (service *authService) Login(username string, password string) (string, error) {
	// 実装はユーザーが行う
	return "", nil
}

// ValidateToken 送信されたトークンの妥当性を検証します。
func (service *authService) ValidateToken(token string) (bool, error) {
	// 実装はユーザーが行う
	return true, nil
}

// IssueInternalToken マイクロサービス間で利用する内部認証用トークンを発行します。
func (service *authService) IssueInternalToken(serviceName string) (string, error) {
	// 実装はユーザーが行う
	return "", nil
}
