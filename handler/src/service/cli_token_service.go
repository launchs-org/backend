package service

import (
	"context"
	"handler/middlewares"
	"handler/models"
	"handler/repository"
	"time"

	"github.com/google/uuid"
)

// CliTokenService は CLIトークンの発行・一覧・失効のビジネスロジックを定義するインターフェース
type CliTokenService interface {
	IssueToken(ctx context.Context, userID string, name string, expiresAt *time.Time) (*models.CliToken, string, error) // CLIトークンを発行する（平文トークンを一度だけ返す）
	ListTokens(ctx context.Context, userID string) ([]*models.CliToken, error)                                          // ユーザーの発行済みCLIトークン一覧を返す
	RevokeToken(ctx context.Context, userID string, cliTokenID string) error                                            // CLIトークンを失効させる
}

// cliTokenServiceImpl は CliTokenService の実装
type cliTokenServiceImpl struct {
	cliTokenRepo repository.CliTokenRepository // CLIトークンリポジトリ
}

// NewCliTokenService は CliTokenService の実装を返す
func NewCliTokenService(cliTokenRepo repository.CliTokenRepository) CliTokenService {
	return &cliTokenServiceImpl{
		cliTokenRepo: cliTokenRepo, // CLIトークンリポジトリを注入する
	}
}

// IssueToken は新規にjtiを発行し、CLI用JWTに署名してDBにメタ情報を保存する
func (svc *cliTokenServiceImpl) IssueToken(ctx context.Context, userID string, name string, expiresAt *time.Time) (*models.CliToken, string, error) {
	jti := uuid.NewString() // トークンIDを生成する

	signedToken, err := middlewares.IssueCliToken(jti, userID, expiresAt) // CLI用JWTに署名する
	if err != nil {
		return nil, "", err // 署名エラーを返す
	}

	cliTokenData := &models.CliToken{
		ID:        jti,       // jtiを主キーとして設定する
		UserID:    userID,    // 発行対象のユーザーIDを設定する
		Name:      name,      // 用途ラベルを設定する
		ExpiresAt: expiresAt, // 有効期限を設定する（nilなら無期限）
	}
	if err := svc.cliTokenRepo.Create(ctx, cliTokenData); err != nil { // リポジトリ経由でメタ情報を保存する
		return nil, "", err // 作成エラーを返す
	}

	return cliTokenData, signedToken, nil // メタ情報と平文トークンを返す
}

// ListTokens はユーザーの発行済みCLIトークン一覧を返す
func (svc *cliTokenServiceImpl) ListTokens(ctx context.Context, userID string) ([]*models.CliToken, error) {
	return svc.cliTokenRepo.FindAllByUserID(ctx, userID) // リポジトリ経由で一覧を取得する
}

// RevokeToken は所有者チェックを行った上でCLIトークンを失効させる
func (svc *cliTokenServiceImpl) RevokeToken(ctx context.Context, userID string, cliTokenID string) error {
	cliTokenData, err := svc.cliTokenRepo.FindByID(ctx, cliTokenID) // CLIトークンを取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if cliTokenData.UserID != userID { // ユーザーIDが一致しない場合はforbiddenを返す
		return ErrForbidden // アクセス拒否エラーを返す
	}
	return svc.cliTokenRepo.Revoke(ctx, cliTokenID) // リポジトリ経由で失効させる
}
