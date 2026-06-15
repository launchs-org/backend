package service

import (
	"app/models"
	"context"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// mockEnvVarMountRepository は EnvVarMountRepository のテスト用モック実装
type mockEnvVarMountRepository struct {
	createFunc                        func(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error
	findByIDFunc                      func(ctx context.Context, mountID string) (*models.EnvVarMount, error)
	findAllByDeploymentIDFunc         func(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error)
	findByDeploymentIDAndEnvVarIDFunc func(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error)
	deleteFunc                        func(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error
}

func (mock *mockEnvVarMountRepository) Create(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	if mock.createFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createFunc(ctx, tx, mount)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockEnvVarMountRepository) FindByID(ctx context.Context, mountID string) (*models.EnvVarMount, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, mountID)
	}
	return &models.EnvVarMount{ID: mountID, DeploymentID: "deployment-id-1"}, nil // デフォルトはマウント設定を返す
}

func (mock *mockEnvVarMountRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error) {
	if mock.findAllByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllByDeploymentIDFunc(ctx, deploymentID)
	}
	return []*models.EnvVarMount{}, nil // デフォルトは空一覧を返す
}

func (mock *mockEnvVarMountRepository) FindByDeploymentIDAndEnvVarID(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) {
	if mock.findByDeploymentIDAndEnvVarIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDAndEnvVarIDFunc(ctx, deploymentID, envVarID)
	}
	return nil, gorm.ErrRecordNotFound // デフォルトは存在しないとして返す
}

func (mock *mockEnvVarMountRepository) Delete(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
	if mock.deleteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteFunc(ctx, tx, mount)
	}
	return nil // デフォルトは nil を返す
}

// mockDeploymentRepositoryForMount は DeploymentRepository のテスト用モック実装（mount service テスト専用）
type mockDeploymentRepositoryForMount struct {
	findByIDFunc func(ctx context.Context, deploymentID string) (*models.Deployment, error)
}

func (mock *mockDeploymentRepositoryForMount) Create(ctx context.Context, deployment *models.Deployment) error {
	return nil
}

func (mock *mockDeploymentRepositoryForMount) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, deploymentID)
	}
	return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // デフォルトは deployment を返す
}

func (mock *mockDeploymentRepositoryForMount) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil
}

func (mock *mockDeploymentRepositoryForMount) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return []models.Deployment{}, nil
}

func (mock *mockDeploymentRepositoryForMount) Save(ctx context.Context, deployment *models.Deployment) error {
	return nil
}

func (mock *mockDeploymentRepositoryForMount) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	return nil
}

func (mock *mockDeploymentRepositoryForMount) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepositoryForMount) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepositoryForMount) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepositoryForMount) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepositoryForMount) Delete(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

// TestListEnvVarMounts_正常に一覧が取得される は ListEnvVarMounts がマウント設定一覧を返すことを確認する
func TestListEnvVarMounts_正常に一覧が取得される(t *testing.T) {
	expectedList := []*models.EnvVarMount{
		{ID: "mount-id-1", DeploymentID: "deployment-id-1", EnvVarID: "env-var-id-1"}, // マウント設定1
		{ID: "mount-id-2", DeploymentID: "deployment-id-1", EnvVarID: "env-var-id-2"}, // マウント設定2
	}

	mountRepo := &mockEnvVarMountRepository{
		findAllByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]*models.EnvVarMount, error) {
			return expectedList, nil // 一覧を返す
		},
	}
	deploymentRepo := &mockDeploymentRepositoryForMount{}  // デフォルト実装を使用する
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewEnvVarMountService(nil, mountRepo, deploymentRepo, projectRepo) // サービスを生成する（db は未使用）

	result, err := svc.ListEnvVarMounts(context.Background(), "test-user-id", "deployment-id-1") // 一覧を取得する
	if err != nil {
		t.Fatalf("ListEnvVarMounts がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
}

// TestListEnvVarMounts_他ユーザーのDeploymentはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestListEnvVarMounts_他ユーザーのDeploymentはErrForbiddenを返す(t *testing.T) {
	mountRepo := &mockEnvVarMountRepository{}                 // 使用しないモックを設定する
	deploymentRepo := &mockDeploymentRepositoryForMount{}     // デフォルト実装を使用する
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "other-user-id"}, nil // 別ユーザーとして返す
		},
	}

	svc := NewEnvVarMountService(nil, mountRepo, deploymentRepo, projectRepo) // サービスを生成する

	_, err := svc.ListEnvVarMounts(context.Background(), "test-user-id", "deployment-id-1") // 一覧を取得する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("ErrForbidden が返ることを期待しましたが、エラーが返りませんでした")
	}
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestCreateEnvVarMount_正常にマウント設定が作成される は CreateEnvVarMount がマウント設定を作成することを確認する
func TestCreateEnvVarMount_正常にマウント設定が作成される(t *testing.T) {
	mountRepo := &mockEnvVarMountRepository{} // デフォルト実装（重複なし・作成成功）を使用する
	deploymentRepo := &mockDeploymentRepositoryForMount{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t) // テスト用 DB を準備する（トランザクション用）

	svc := NewEnvVarMountService(db, mountRepo, deploymentRepo, projectRepo) // サービスを生成する

	req := CreateEnvVarMountRequest{
		EnvVarID:    "env-var-id-1", // env_var ID を設定する
		OverrideKey: "MY_KEY",        // オーバーライドキーを設定する
	}
	result, err := svc.CreateEnvVarMount(context.Background(), "test-user-id", "deployment-id-1", req) // マウント設定を作成する
	if err != nil {
		t.Fatalf("CreateEnvVarMount がエラーを返しました: %v", err)
	}
	if result.EnvVarID != "env-var-id-1" { // env_var ID が一致することを確認する
		t.Errorf("期待する env_var_id: env-var-id-1, 実際の env_var_id: %s", result.EnvVarID)
	}
	if result.OverrideKey != "MY_KEY" { // override_key が一致することを確認する
		t.Errorf("期待する override_key: MY_KEY, 実際の override_key: %s", result.OverrideKey)
	}
}

// TestCreateEnvVarMount_重複マウントはErrDuplicateMountを返す は同一 deployment・同一 env_var のマウントが ErrDuplicateMount を返すことを確認する
func TestCreateEnvVarMount_重複マウントはErrDuplicateMountを返す(t *testing.T) {
	mountRepo := &mockEnvVarMountRepository{
		findByDeploymentIDAndEnvVarIDFunc: func(ctx context.Context, deploymentID string, envVarID string) (*models.EnvVarMount, error) {
			return &models.EnvVarMount{ID: "existing-mount-id"}, nil // 既存マウントを返して重複を示す
		},
	}
	deploymentRepo := &mockDeploymentRepositoryForMount{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewEnvVarMountService(nil, mountRepo, deploymentRepo, projectRepo) // サービスを生成する

	req := CreateEnvVarMountRequest{EnvVarID: "env-var-id-1"} // リクエストを設定する
	_, err := svc.CreateEnvVarMount(context.Background(), "test-user-id", "deployment-id-1", req) // マウント設定を作成する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("ErrDuplicateMount が返ることを期待しましたが、エラーが返りませんでした")
	}
	if err != ErrDuplicateMount { // ErrDuplicateMount であることを確認する
		t.Errorf("期待するエラー: ErrDuplicateMount, 実際のエラー: %v", err)
	}
}

// TestCreateEnvVarMount_他ユーザーのDeploymentはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestCreateEnvVarMount_他ユーザーのDeploymentはErrForbiddenを返す(t *testing.T) {
	mountRepo := &mockEnvVarMountRepository{}
	deploymentRepo := &mockDeploymentRepositoryForMount{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "other-user-id"}, nil // 別ユーザーとして返す
		},
	}

	svc := NewEnvVarMountService(nil, mountRepo, deploymentRepo, projectRepo) // サービスを生成する

	req := CreateEnvVarMountRequest{EnvVarID: "env-var-id-1"} // リクエストを設定する
	_, err := svc.CreateEnvVarMount(context.Background(), "test-user-id", "deployment-id-1", req) // マウント設定を作成する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestDeleteEnvVarMount_正常にマウント設定が削除される は DeleteEnvVarMount がマウント設定を削除することを確認する
func TestDeleteEnvVarMount_正常にマウント設定が削除される(t *testing.T) {
	deleteCalled := false // 削除が呼ばれたことを追跡する変数
	mountRepo := &mockEnvVarMountRepository{
		deleteFunc: func(ctx context.Context, tx *gorm.DB, mount *models.EnvVarMount) error {
			deleteCalled = true // 削除が呼ばれたことを記録する
			return nil          // 削除成功を返す
		},
	}
	deploymentRepo := &mockDeploymentRepositoryForMount{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t) // テスト用 DB を準備する

	svc := NewEnvVarMountService(db, mountRepo, deploymentRepo, projectRepo) // サービスを生成する

	err := svc.DeleteEnvVarMount(context.Background(), "test-user-id", "mount-id-1") // マウント設定を削除する
	if err != nil {
		t.Fatalf("DeleteEnvVarMount がエラーを返しました: %v", err)
	}
	if !deleteCalled { // 削除が呼ばれていることを確認する
		t.Error("Delete が呼び出されませんでした")
	}
}

// TestDeleteEnvVarMount_他ユーザーのDeploymentはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestDeleteEnvVarMount_他ユーザーのDeploymentはErrForbiddenを返す(t *testing.T) {
	mountRepo := &mockEnvVarMountRepository{}
	deploymentRepo := &mockDeploymentRepositoryForMount{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "other-user-id"}, nil // 別ユーザーとして返す
		},
	}

	svc := NewEnvVarMountService(nil, mountRepo, deploymentRepo, projectRepo) // サービスを生成する

	err := svc.DeleteEnvVarMount(context.Background(), "test-user-id", "mount-id-1") // マウント設定を削除する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}
