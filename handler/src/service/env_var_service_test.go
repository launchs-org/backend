package service

import (
	"handler/models"
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// mockEnvVarRepository は EnvVarRepository のテスト用モック実装
type mockEnvVarRepository struct {
	createFunc                  func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error
	findByIDFunc                func(ctx context.Context, envVarID string) (*models.EnvVar, error)
	findAllByProjectIDFunc      func(ctx context.Context, projectID string) ([]*models.EnvVar, error)
	updateFunc                  func(ctx context.Context, envVar *models.EnvVar) error // tx は省略したシグネチャで定義する
	deleteFunc                  func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error
	existsByProjectIDAndKeyFunc func(ctx context.Context, projectID string, key string, excludeID string) (bool, error)
}

func (mock *mockEnvVarRepository) Create(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	return mock.createFunc(ctx, tx, envVar) // モック関数を呼び出す
}

func (mock *mockEnvVarRepository) FindByID(ctx context.Context, envVarID string) (*models.EnvVar, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, envVarID)
	}
	return &models.EnvVar{ID: envVarID, ProjectID: "project-id-1"}, nil // デフォルトは env_var を返す
}

func (mock *mockEnvVarRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
	if mock.findAllByProjectIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllByProjectIDFunc(ctx, projectID)
	}
	return []*models.EnvVar{}, nil // デフォルトは空一覧を返す
}

func (mock *mockEnvVarRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, envVarID string) (*models.EnvVar, error) {
	if mock.findByIDFunc != nil { // findByID と同じモック関数を流用する
		return mock.findByIDFunc(ctx, envVarID)
	}
	return &models.EnvVar{ID: envVarID, ProjectID: "project-id-1"}, nil // デフォルトは env_var を返す
}

func (mock *mockEnvVarRepository) Update(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	if mock.updateFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateFunc(ctx, envVar)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockEnvVarRepository) Delete(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
	if mock.deleteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteFunc(ctx, tx, envVar)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockEnvVarRepository) ExistsByProjectIDAndKey(ctx context.Context, projectID string, key string, excludeID string) (bool, error) {
	if mock.existsByProjectIDAndKeyFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.existsByProjectIDAndKeyFunc(ctx, projectID, key, excludeID)
	}
	return false, nil // デフォルトは重複なしを返す
}

// TestListEnvVars_正常に一覧が取得される は ListEnvVars が env_var 一覧を返すことを確認する
func TestListEnvVars_正常に一覧が取得される_service(t *testing.T) {
	expectedList := []*models.EnvVar{
		{ID: "env-var-id-1", ProjectID: "project-id-1", Key: "KEY1", Value: "val1", IsSecret: false}, // 通常 env_var
		{ID: "env-var-id-2", ProjectID: "project-id-1", Key: "SECRET", Value: "secret-val", IsSecret: true}, // シークレット env_var
	}

	envVarRepo := &mockEnvVarRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
			return expectedList, nil // 一覧を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewEnvVarService(nil, envVarRepo, projectRepo) // サービスを生成する（db は未使用）

	result, err := svc.ListEnvVars(context.Background(), "test-user-id", "project-id-1") // 一覧を取得する
	if err != nil {
		t.Fatalf("ListEnvVars がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
}

// TestListEnvVars_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestListEnvVars_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	envVarRepo := &mockEnvVarRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewEnvVarService(nil, envVarRepo, projectRepo) // サービスを生成する

	_, err := svc.ListEnvVars(context.Background(), "other-user-id", "project-id-1") // 他ユーザーとして一覧を取得する
	if err == nil {
		t.Fatal("ErrForbidden が返ることを期待しましたが、エラーが返りませんでした")
	}
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestCreateEnvVar_正常にenv_varが作成される は CreateEnvVar が env_var を作成することを確認する
func TestCreateEnvVar_正常にenv_varが作成される_service(t *testing.T) {
	var capturedEnvVar *models.EnvVar // キャプチャした env_var を格納する変数を定義する

	envVarRepo := &mockEnvVarRepository{
		createFunc: func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
			capturedEnvVar = envVar      // env_var をキャプチャする
			envVar.ID = "new-env-var-id" // ID を付与する
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t) // テスト用 DB を準備する
	svc := NewEnvVarService(db, envVarRepo, projectRepo) // サービスを生成する

	req := CreateEnvVarRequest{Key: "MY_KEY", Value: "my-value", IsSecret: false} // リクエストを定義する
	result, err := svc.CreateEnvVar(context.Background(), "test-user-id", "project-id-1", req) // env_var を作成する
	if err != nil {
		t.Fatalf("CreateEnvVar がエラーを返しました: %v", err)
	}
	if result.ID != "new-env-var-id" { // ID が付与されていることを確認する
		t.Errorf("期待する ID: new-env-var-id, 実際の ID: %s", result.ID)
	}
	if capturedEnvVar.Key != "MY_KEY" { // キーが正しく設定されていることを確認する
		t.Errorf("期待する Key: MY_KEY, 実際の Key: %s", capturedEnvVar.Key)
	}
	if capturedEnvVar.ProjectID != "project-id-1" { // ProjectID が正しく設定されていることを確認する
		t.Errorf("期待する ProjectID: project-id-1, 実際の ProjectID: %s", capturedEnvVar.ProjectID)
	}
}

// TestCreateEnvVar_キーが重複する場合はランダムサフィックスでリネームされる は重複時にエラーを返さずリネームすることを確認する
func TestCreateEnvVar_キーが重複する場合はランダムサフィックスでリネームされる_service(t *testing.T) {
	var capturedEnvVar *models.EnvVar // キャプチャした env_var を格納する変数を定義する
	existsCallCount := 0              // ExistsByProjectIDAndKey の呼び出し回数を記録する

	envVarRepo := &mockEnvVarRepository{
		createFunc: func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
			capturedEnvVar = envVar      // env_var をキャプチャする
			envVar.ID = "new-env-var-id" // ID を付与する
			return nil
		},
		existsByProjectIDAndKeyFunc: func(ctx context.Context, projectID string, key string, excludeID string) (bool, error) {
			existsCallCount++
			return key == "MY_KEY", nil // 希望キーのみ重複しているとみなす
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t) // テスト用 DB を準備する
	svc := NewEnvVarService(db, envVarRepo, projectRepo) // サービスを生成する

	req := CreateEnvVarRequest{Key: "MY_KEY", Value: "my-value", IsSecret: false} // リクエストを定義する
	result, err := svc.CreateEnvVar(context.Background(), "test-user-id", "project-id-1", req) // env_var を作成する
	if err != nil {
		t.Fatalf("CreateEnvVar がエラーを返しました: %v", err) // エラーではなくリネームされることを期待する
	}
	if capturedEnvVar.Key == "MY_KEY" { // リネームされていることを確認する
		t.Error("キーがリネームされていません")
	}
	if !strings.HasPrefix(capturedEnvVar.Key, "MY_KEY_") { // "MY_KEY_" プレフィックスを持つことを確認する
		t.Errorf("期待するプレフィックス: MY_KEY_, 実際のキー: %s", capturedEnvVar.Key)
	}
	if result.Key != capturedEnvVar.Key { // 返り値にもリネーム後のキーが反映されていることを確認する
		t.Errorf("返り値のキーが一致しません: %s != %s", result.Key, capturedEnvVar.Key)
	}
	if existsCallCount < 2 { // 希望キーと候補キーの両方で確認していることを確認する
		t.Errorf("ExistsByProjectIDAndKey の呼び出し回数が想定より少ないです: %d", existsCallCount)
	}
}

// TestCreateEnvVar_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestCreateEnvVar_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	envVarRepo := &mockEnvVarRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewEnvVarService(nil, envVarRepo, projectRepo) // サービスを生成する

	req := CreateEnvVarRequest{Key: "KEY", Value: "val", IsSecret: false} // リクエストを定義する
	_, err := svc.CreateEnvVar(context.Background(), "other-user-id", "project-id-1", req) // 他ユーザーとして作成する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestUpdateEnvVar_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestUpdateEnvVar_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	envVarRepo := &mockEnvVarRepository{
		findByIDFunc: func(ctx context.Context, envVarID string) (*models.EnvVar, error) {
			return &models.EnvVar{ID: envVarID, ProjectID: "project-id-1"}, nil // env_var を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewEnvVarService(nil, envVarRepo, projectRepo) // サービスを生成する

	req := UpdateEnvVarRequest{} // リクエストを定義する
	_, err := svc.UpdateEnvVar(context.Background(), "other-user-id", "env-var-id-1", req) // 他ユーザーとして更新する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestDeleteEnvVar_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestDeleteEnvVar_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	envVarRepo := &mockEnvVarRepository{
		findByIDFunc: func(ctx context.Context, envVarID string) (*models.EnvVar, error) {
			return &models.EnvVar{ID: envVarID, ProjectID: "project-id-1"}, nil // env_var を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewEnvVarService(nil, envVarRepo, projectRepo) // サービスを生成する

	err := svc.DeleteEnvVar(context.Background(), "other-user-id", "env-var-id-1") // 他ユーザーとして削除する
	if err != ErrForbidden { // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}
