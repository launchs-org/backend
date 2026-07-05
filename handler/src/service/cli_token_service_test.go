package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"handler/middlewares"
	"handler/models"
	"testing"
	"time"

	"gorm.io/gorm"
)

// mockCliTokenRepository は CliTokenRepository のテスト用モック実装
type mockCliTokenRepository struct {
	createFunc          func(ctx context.Context, cliTokenData *models.CliToken) error
	findByIDFunc        func(ctx context.Context, cliTokenID string) (*models.CliToken, error)
	findAllByUserIDFunc func(ctx context.Context, userID string) ([]*models.CliToken, error)
	revokeFunc          func(ctx context.Context, cliTokenID string) error
}

func (mock *mockCliTokenRepository) Create(ctx context.Context, cliTokenData *models.CliToken) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, cliTokenData)
	}
	return nil
}

func (mock *mockCliTokenRepository) FindByID(ctx context.Context, cliTokenID string) (*models.CliToken, error) {
	if mock.findByIDFunc != nil {
		return mock.findByIDFunc(ctx, cliTokenID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (mock *mockCliTokenRepository) FindAllByUserID(ctx context.Context, userID string) ([]*models.CliToken, error) {
	if mock.findAllByUserIDFunc != nil {
		return mock.findAllByUserIDFunc(ctx, userID)
	}
	return []*models.CliToken{}, nil
}

func (mock *mockCliTokenRepository) Revoke(ctx context.Context, cliTokenID string) error {
	if mock.revokeFunc != nil {
		return mock.revokeFunc(ctx, cliTokenID)
	}
	return nil
}

// setupCliTokenTestKeys はテスト用にCLIトークン署名鍵をセットする
func setupCliTokenTestKeys(t *testing.T) {
	t.Helper()
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("テスト用鍵ペアの生成に失敗しました: %v", err)
	}
	middlewares.SetCliTokenKeysForTest(privKey, pubKey)
}

// TestCliTokenService_IssueToken_正常に発行される は IssueToken でCLIトークンが発行されることを確認する
func TestCliTokenService_IssueToken_正常に発行される(t *testing.T) {
	setupCliTokenTestKeys(t)

	var createdToken *models.CliToken
	mockRepo := &mockCliTokenRepository{
		createFunc: func(ctx context.Context, cliTokenData *models.CliToken) error {
			createdToken = cliTokenData
			return nil
		},
	}
	svc := NewCliTokenService(mockRepo)

	expiry := time.Now().Add(24 * time.Hour)
	tokenData, plainToken, err := svc.IssueToken(context.Background(), "user-1", "my-cli", &expiry)
	if err != nil {
		t.Fatalf("IssueToken がエラーを返しました: %v", err)
	}
	if plainToken == "" { // 平文トークンが返却されることを確認する
		t.Errorf("平文トークンが空です")
	}
	if tokenData.UserID != "user-1" { // ユーザーIDが設定されることを確認する
		t.Errorf("期待する UserID: user-1, 実際の UserID: %s", tokenData.UserID)
	}
	if createdToken == nil || createdToken.ID == "" { // jtiが発行されDBに保存されることを確認する
		t.Errorf("CliTokenRepository.Create が呼ばれていないか、jtiが空です")
	}
}

// TestCliTokenService_IssueToken_無期限指定 は expiresAt が nil の場合に無期限のトークンが発行されることを確認する
func TestCliTokenService_IssueToken_無期限指定(t *testing.T) {
	setupCliTokenTestKeys(t)

	mockRepo := &mockCliTokenRepository{}
	svc := NewCliTokenService(mockRepo)

	tokenData, _, err := svc.IssueToken(context.Background(), "user-1", "my-cli", nil)
	if err != nil {
		t.Fatalf("IssueToken がエラーを返しました: %v", err)
	}
	if tokenData.ExpiresAt != nil { // ExpiresAt が nil のままであることを確認する
		t.Errorf("期待する ExpiresAt: nil, 実際の ExpiresAt: %v", tokenData.ExpiresAt)
	}
}

// TestCliTokenService_RevokeToken_所有者本人は失効できる は RevokeToken が所有者本人からの失効を許可することを確認する
func TestCliTokenService_RevokeToken_所有者本人は失効できる(t *testing.T) {
	revoked := false
	mockRepo := &mockCliTokenRepository{
		findByIDFunc: func(ctx context.Context, cliTokenID string) (*models.CliToken, error) {
			return &models.CliToken{ID: cliTokenID, UserID: "user-1"}, nil
		},
		revokeFunc: func(ctx context.Context, cliTokenID string) error {
			revoked = true
			return nil
		},
	}
	svc := NewCliTokenService(mockRepo)

	if err := svc.RevokeToken(context.Background(), "user-1", "token-1"); err != nil {
		t.Fatalf("RevokeToken がエラーを返しました: %v", err)
	}
	if !revoked { // Revoke が呼ばれていることを確認する
		t.Errorf("CliTokenRepository.Revoke が呼ばれていません")
	}
}

// TestCliTokenService_RevokeToken_他ユーザーは失効できない は RevokeToken が所有者以外からの失効を拒否することを確認する
func TestCliTokenService_RevokeToken_他ユーザーは失効できない(t *testing.T) {
	mockRepo := &mockCliTokenRepository{
		findByIDFunc: func(ctx context.Context, cliTokenID string) (*models.CliToken, error) {
			return &models.CliToken{ID: cliTokenID, UserID: "owner-user"}, nil
		},
	}
	svc := NewCliTokenService(mockRepo)

	err := svc.RevokeToken(context.Background(), "other-user", "token-1")
	if err != ErrForbidden { // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestCliTokenService_ListTokens_ユーザーの一覧を返す は ListTokens がリポジトリの結果をそのまま返すことを確認する
func TestCliTokenService_ListTokens_ユーザーの一覧を返す(t *testing.T) {
	expected := []*models.CliToken{
		{ID: "token-1", UserID: "user-1"},
		{ID: "token-2", UserID: "user-1"},
	}
	mockRepo := &mockCliTokenRepository{
		findAllByUserIDFunc: func(ctx context.Context, userID string) ([]*models.CliToken, error) {
			return expected, nil
		},
	}
	svc := NewCliTokenService(mockRepo)

	tokenList, err := svc.ListTokens(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListTokens がエラーを返しました: %v", err)
	}
	if len(tokenList) != 2 { // 件数が一致することを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(tokenList))
	}
}
