package services

import (
	"errors"
	"os"
	repo "auth/models"
	s_models "github.com/launchs-org/backend/shared/models"

	"golang.org/x/crypto/bcrypt"
)

// AuthService 認証とトークン発行に関するビジネスロジックを定義するインターフェース
type AuthService interface {
	// 認証関連
	Login(username string, password string) (string, error)
	SignUp(username string, password string, email string) error
	ValidateToken(token string) (bool, error)
	IssueInternalToken(serviceName string) (string, error)

	// ユーザー管理 (CRUD)
	CreateUser(user *s_models.User) error
	GetUser(userID string) (*s_models.User, error)
	DeleteUser(userID string) error
}
// ...

// SignUp 新規ユーザーを登録します。環境変数で許可されている場合のみ実行可能です。
func (service *authService) SignUp(username string, password string, email string) error {
	// サインアップが許可されているか確認
	if os.Getenv("ALLOW_SIGNUP") != "true" {
		return errors.New("サインアップは現在無効です")
	}

	// パスワードをハッシュ化
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// ユーザーを作成
	user := &s_models.User{
		Username: username,
		Password: string(hashedPassword),
		Email:    email,
	}

	return service.CreateUser(user)
}

// authService AuthService の実装
type authService struct {
	db *s_models.Database
}

// NewAuthService AuthService の新しいインスタンスを作成する
func NewAuthService(db *s_models.Database) AuthService {
	return &authService{
		db: db,
	}
}

// CreateUser 新規ユーザーをデータベースに保存します。
func (service *authService) CreateUser(user *s_models.User) error {
	// GORMを使用してユーザー情報を保存
	return service.db.Conn.Create(user).Error
}

// GetUser ユーザーIDを指定してユーザー情報を取得します。
func (service *authService) GetUser(userID string) (*s_models.User, error) {
	var user s_models.User
	// ユーザーIDに基づいてデータベースから1件取得
	if err := service.db.Conn.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// DeleteUser ユーザーIDを指定してユーザー情報を削除します。
func (service *authService) DeleteUser(userID string) error {
	// 指定されたIDのユーザーを物理削除（または論理削除）
	return service.db.Conn.Delete(&s_models.User{}, "id = ?", userID).Error
}

// Login ユーザー名とパスワードを検証し、JWT トークンを発行します。
func (service *authService) Login(username string, password string) (string, error) {
	// ユーザーを検索
	user, err := repo.GetUserByUsername(username)
	if err != nil {
		return "", errors.New("無効なユーザー名またはパスワードです")
	}

	// パスワードを比較 (bcryptを使用)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("無効なユーザー名またはパスワードです")
	}

	// 認証成功時、トークンを発行
	return "mock-jwt-token", nil
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
