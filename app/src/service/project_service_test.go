package service

import (
	"app/models"
	"context"
	"testing"

	"gorm.io/gorm"
)

// mockProjectRepositoryForProjectService は ProjectRepository のテスト用モック（project_service テスト専用）
type mockProjectRepositoryForProjectService struct {
	findByIDFunc func(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error)
}

func (mock *mockProjectRepositoryForProjectService) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, tx, projectID)
	}
	return nil, nil // デフォルトは nil を返す
}

func (mock *mockProjectRepositoryForProjectService) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	return nil, nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) Save(ctx context.Context, project *models.Project) error {
	return nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	return nil, nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // テストでは使用しない
}

func (mock *mockProjectRepositoryForProjectService) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return nil // テストでは使用しない
}

// mockHarborCredentialRepositoryForProjectService は HarborCredentialRepository のテスト用モック（project_service テスト専用）
type mockHarborCredentialRepositoryForProjectService struct {
	findByProjectIDFunc func(ctx context.Context, tx *gorm.DB, projectID string) (*models.HarborCredential, error)
}

func (mock *mockHarborCredentialRepositoryForProjectService) Create(ctx context.Context, tx *gorm.DB, credential *models.HarborCredential) error {
	return nil // テストでは使用しない
}

func (mock *mockHarborCredentialRepositoryForProjectService) FindByProjectID(ctx context.Context, tx *gorm.DB, projectID string) (*models.HarborCredential, error) {
	if mock.findByProjectIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByProjectIDFunc(ctx, tx, projectID)
	}
	return nil, nil // デフォルトは nil を返す
}

func (mock *mockHarborCredentialRepositoryForProjectService) DeleteByProjectID(ctx context.Context, tx *gorm.DB, projectID string) error {
	return nil // テストでは使用しない
}

func (mock *mockHarborCredentialRepositoryForProjectService) FindByProjectIDNoTx(ctx context.Context, projectID string) (*models.HarborCredential, error) {
	return nil, nil // テストでは使用しない
}

// TestGetProjectQuota_403_他ユーザーはForbiddenを返す は他ユーザーのプロジェクトへのクォータ取得で ErrForbidden を返すことを確認する
func TestGetProjectQuota_403_他ユーザーはForbiddenを返す(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	projectRepo := &mockProjectRepositoryForProjectService{
		findByIDFunc: func(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
			return &models.Project{
				ID:     projectID,    // プロジェクト ID を設定する
				UserID: "other-user", // 別ユーザーの所有プロジェクトを設定する
			}, nil
		},
	}

	svc := &projectServiceImpl{
		projectRepo: projectRepo, // テスト用リポジトリを注入する
	}

	_, err := svc.GetProjectQuota(ctx, "user-1", "project-1") // 他ユーザーのクォータを取得しようとする
	if err != ErrForbidden {                                  // ErrForbidden が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", ErrForbidden, err)
	}
}

// TestGetProjectQuota_404_プロジェクトが存在しない は存在しないプロジェクトのクォータ取得で gorm.ErrRecordNotFound を返すことを確認する
func TestGetProjectQuota_404_プロジェクトが存在しない(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	projectRepo := &mockProjectRepositoryForProjectService{
		findByIDFunc: func(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
			return nil, gorm.ErrRecordNotFound // レコードなしエラーを返す
		},
	}

	svc := &projectServiceImpl{
		projectRepo: projectRepo, // テスト用リポジトリを注入する
	}

	_, err := svc.GetProjectQuota(ctx, "user-1", "nonexistent") // 存在しないプロジェクトのクォータを取得しようとする
	if err != gorm.ErrRecordNotFound {                           // gorm.ErrRecordNotFound が返ることを確認する
		t.Errorf("期待するエラー %v、実際のエラー %v", gorm.ErrRecordNotFound, err)
	}
}
