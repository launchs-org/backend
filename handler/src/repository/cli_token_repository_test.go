package repository

import (
	"context"
	"errors"
	"handler/models"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TestCliTokenRepository_Create_正常に作成される は Create でCLIトークンが作成できることを確認する
func TestCliTokenRepository_Create_正常に作成される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	repo := NewCliTokenRepository(db)

	jti := uuid.NewString() // テスト用 jti を生成する
	cliTokenData := &models.CliToken{
		ID:     jti,               // jti を設定する
		UserID: "test-user-id",    // ユーザーIDを設定する
		Name:   "test-token-name", // 用途ラベルを設定する
	}
	t.Cleanup(func() { db.Unscoped().Delete(cliTokenData) }) // テスト終了後にレコードを削除する

	if err := repo.Create(context.Background(), cliTokenData); err != nil {
		t.Fatalf("Create がエラーを返しました: %v", err)
	}

	fetchedToken, err := repo.FindByID(context.Background(), jti)
	if err != nil {
		t.Fatalf("作成したCLIトークンの取得に失敗しました: %v", err)
	}
	if fetchedToken.UserID != "test-user-id" { // ユーザーIDが一致することを確認する
		t.Errorf("期待する UserID: test-user-id, 実際の UserID: %s", fetchedToken.UserID)
	}
}

// TestCliTokenRepository_FindByID_正常に取得される は FindByID でjti指定の取得ができることを確認する
func TestCliTokenRepository_FindByID_正常に取得される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	repo := NewCliTokenRepository(db)

	jti := uuid.NewString() // テスト用 jti を生成する
	cliTokenData := &models.CliToken{
		ID:     jti,
		UserID: "test-user-id",
		Name:   "test-token-name",
	}
	if err := db.Create(cliTokenData).Error; err != nil {
		t.Fatalf("テスト用 CliToken の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(cliTokenData) })

	fetchedToken, err := repo.FindByID(context.Background(), jti)
	if err != nil {
		t.Fatalf("FindByID がエラーを返しました: %v", err)
	}
	if fetchedToken.ID != jti { // ID が一致することを確認する
		t.Errorf("期待する ID: %s, 実際の ID: %s", jti, fetchedToken.ID)
	}
}

// TestCliTokenRepository_Delete_レコードが削除される は Delete でレコードが物理削除されることを確認する
func TestCliTokenRepository_Delete_レコードが削除される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	repo := NewCliTokenRepository(db)

	jti := uuid.NewString() // テスト用 jti を生成する
	cliTokenData := &models.CliToken{
		ID:     jti,
		UserID: "test-user-id",
		Name:   "test-token-name",
	}
	if err := db.Create(cliTokenData).Error; err != nil {
		t.Fatalf("テスト用 CliToken の作成に失敗しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(cliTokenData) })

	if err := repo.Delete(context.Background(), jti); err != nil {
		t.Fatalf("Delete がエラーを返しました: %v", err)
	}

	_, err := repo.FindByID(context.Background(), jti)
	if !errors.Is(err, gorm.ErrRecordNotFound) { // 削除後はレコードが見つからないことを確認する
		t.Errorf("期待するエラー: gorm.ErrRecordNotFound, 実際のエラー: %v", err)
	}
}

// TestCliTokenRepository_FindAllByUserID_ユーザー単位で取得される は FindAllByUserID でユーザーに紐づく一覧が取得できることを確認する
func TestCliTokenRepository_FindAllByUserID_ユーザー単位で取得される(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	repo := NewCliTokenRepository(db)

	tokenA := &models.CliToken{ID: uuid.NewString(), UserID: "test-user-list", Name: "token-a"}
	tokenB := &models.CliToken{ID: uuid.NewString(), UserID: "test-user-list", Name: "token-b"}
	otherUserToken := &models.CliToken{ID: uuid.NewString(), UserID: "other-user", Name: "token-other"}
	for _, tokenData := range []*models.CliToken{tokenA, tokenB, otherUserToken} {
		if err := db.Create(tokenData).Error; err != nil {
			t.Fatalf("テスト用 CliToken の作成に失敗しました: %v", err)
		}
		t.Cleanup(func(target *models.CliToken) func() {
			return func() { db.Unscoped().Delete(target) }
		}(tokenData))
	}

	tokenList, err := repo.FindAllByUserID(context.Background(), "test-user-list")
	if err != nil {
		t.Fatalf("FindAllByUserID がエラーを返しました: %v", err)
	}
	if len(tokenList) != 2 { // test-user-list に紐づく2件のみ取得されることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(tokenList))
	}
}
