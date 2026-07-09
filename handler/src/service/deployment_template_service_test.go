package service

import (
	"handler/models"
	"context"
	"strings"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// mockDeploymentTemplateRepository は DeploymentTemplateRepository のテスト用モック実装
type mockDeploymentTemplateRepository struct {
	findByIDFunc func(ctx context.Context, templateID string) (*models.DeploymentTemplate, error)
}

func (mock *mockDeploymentTemplateRepository) FindAll(ctx context.Context) ([]*models.DeploymentTemplate, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentTemplateRepository) FindByID(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, templateID)
	}
	return nil, nil
}
func (mock *mockDeploymentTemplateRepository) Create(ctx context.Context, template *models.DeploymentTemplate) error {
	return nil // 使用しない
}
func (mock *mockDeploymentTemplateRepository) Update(ctx context.Context, template *models.DeploymentTemplate) error {
	return nil // 使用しない
}
func (mock *mockDeploymentTemplateRepository) Delete(ctx context.Context, templateID string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentTemplateRepository) Upsert(ctx context.Context, template *models.DeploymentTemplate) error {
	return nil // 使用しない
}

// mockDeploymentRepositoryForTemplate は DeploymentRepository のテスト用モック実装（deployment_template_service テスト専用）
type mockDeploymentRepositoryForTemplate struct {
	createWithTxFunc func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error
}

func (mock *mockDeploymentRepositoryForTemplate) Create(ctx context.Context, deployment *models.Deployment) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) CreateWithTx(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
	if mock.createWithTxFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createWithTxFunc(ctx, tx, deployment)
	}
	return nil // デフォルトは nil を返す
}
func (mock *mockDeploymentRepositoryForTemplate) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) FindAllRunning(ctx context.Context) ([]models.Deployment, error) {
	return nil, nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) Save(ctx context.Context, deployment *models.Deployment) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdatePendingImageID(ctx context.Context, deploymentID string, imageID string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdatePendingGithubBuildFields(ctx context.Context, deploymentID string, repoURL string, branch string, commitSHA string, directory string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdateDeleteProgress(ctx context.Context, deploymentID string, progress string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) Delete(ctx context.Context, deploymentID string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdateDeploymentStatus(ctx context.Context, deploymentID string, status models.DeploymentStatus) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) UpdateCurrentBuildID(ctx context.Context, deploymentID string, buildID string) error {
	return nil // 使用しない
}
func (mock *mockDeploymentRepositoryForTemplate) ClearCurrentBuildID(ctx context.Context, deploymentID string) error {
	return nil // 使用しない
}

// newTestDeploymentTemplateService はテスト用の DeploymentTemplateService を生成するヘルパー関数
func newTestDeploymentTemplateService(
	t *testing.T,
	templateRepo *mockDeploymentTemplateRepository,
	deploymentRepo *mockDeploymentRepositoryForTemplate,
	envVarRepo *mockEnvVarRepository,
	projectRepo *mockProjectRepository,
) DeploymentTemplateService {
	db := setupApplyTestDB(t) // テスト用 DB を準備する（トランザクション開始のみに使用する）
	return NewDeploymentTemplateService(
		db,
		templateRepo,
		deploymentRepo,
		&mockServiceRepository{},
		envVarRepo,
		&mockEnvVarMountRepository{},
		&mockVolumeRepository{},
		&mockVolumeMountRepository{},
		projectRepo,
		&mockImageRepository{},
		&noopUserQuotaRepository{},
	)
}

// TestCreateDeploymentFromTemplate_TemplateIDが設定される はテンプレート由来のenv_varにTemplateIDが記録されることを確認する
func TestCreateDeploymentFromTemplate_TemplateIDが設定される_service(t *testing.T) {
	templateData := &models.DeploymentTemplate{
		ID:           "template-id-1",
		ImageURL:     "image:latest",
		InstanceSize: "small",
		Replicas:     1,
	}
	envVarsJSON, err := marshalTemplateEnvVars([]models.TemplateEnvVar{{Key: "KEY1", Value: "val1"}}) // テンプレートのenv_var定義を構築する
	if err != nil {
		t.Fatalf("marshalTemplateEnvVars がエラーを返しました: %v", err)
	}
	templateData.EnvVars = envVarsJSON

	var capturedEnvVars []*models.EnvVar // 作成された env_var を記録する

	templateRepo := &mockDeploymentTemplateRepository{
		findByIDFunc: func(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) {
			return templateData, nil // テンプレートを返す
		},
	}
	deploymentRepo := &mockDeploymentRepositoryForTemplate{
		createWithTxFunc: func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
			deployment.ID = "deployment-id-1" // ID を付与する
			return nil
		},
	}
	envVarRepo := &mockEnvVarRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
			return []*models.EnvVar{}, nil // 既存の env_var はなし
		},
		createFunc: func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
			envVar.ID = "env-var-id-1"                     // ID を付与する
			capturedEnvVars = append(capturedEnvVars, envVar) // 作成された env_var を記録する
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := newTestDeploymentTemplateService(t, templateRepo, deploymentRepo, envVarRepo, projectRepo)

	req := CreateDeploymentFromTemplateRequest{
		ProjectID:  "project-id-1",
		TemplateID: "template-id-1",
		Name:       "test-deploy",
	}
	_, err = svc.CreateDeploymentFromTemplate(context.Background(), "test-user-id", req)
	if err != nil {
		t.Fatalf("CreateDeploymentFromTemplate がエラーを返しました: %v", err)
	}
	if len(capturedEnvVars) != 1 { // env_var が1件作成されたことを確認する
		t.Fatalf("期待する作成件数: 1, 実際: %d", len(capturedEnvVars))
	}
	if capturedEnvVars[0].TemplateID == nil || *capturedEnvVars[0].TemplateID != "template-id-1" { // TemplateID が設定されていることを確認する
		t.Errorf("TemplateID が設定されていません: %v", capturedEnvVars[0].TemplateID)
	}
}

// TestCreateDeploymentFromTemplate_テンプレート内の同名キーはランダムサフィックスでリネームされる はテンプレート内重複キーの解決を確認する
func TestCreateDeploymentFromTemplate_テンプレート内の同名キーはランダムサフィックスでリネームされる_service(t *testing.T) {
	templateData := &models.DeploymentTemplate{
		ID:           "template-id-1",
		ImageURL:     "image:latest",
		InstanceSize: "small",
		Replicas:     1,
	}
	envVarsJSON, err := marshalTemplateEnvVars([]models.TemplateEnvVar{ // テンプレート内に同名キーを2つ定義する
		{Key: "DUPLICATE_KEY", Value: "val1"},
		{Key: "DUPLICATE_KEY", Value: "val2"},
	})
	if err != nil {
		t.Fatalf("marshalTemplateEnvVars がエラーを返しました: %v", err)
	}
	templateData.EnvVars = envVarsJSON

	var capturedEnvVars []*models.EnvVar // 作成された env_var を記録する

	templateRepo := &mockDeploymentTemplateRepository{
		findByIDFunc: func(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) {
			return templateData, nil
		},
	}
	deploymentRepo := &mockDeploymentRepositoryForTemplate{
		createWithTxFunc: func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
			deployment.ID = "deployment-id-1"
			return nil
		},
	}
	envVarRepo := &mockEnvVarRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
			return []*models.EnvVar{}, nil // 既存の env_var はなし
		},
		createFunc: func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
			envVar.ID = "env-var-id-" + envVar.Key
			capturedEnvVars = append(capturedEnvVars, envVar)
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil
		},
	}

	svc := newTestDeploymentTemplateService(t, templateRepo, deploymentRepo, envVarRepo, projectRepo)

	req := CreateDeploymentFromTemplateRequest{
		ProjectID:  "project-id-1",
		TemplateID: "template-id-1",
		Name:       "test-deploy",
	}
	_, err = svc.CreateDeploymentFromTemplate(context.Background(), "test-user-id", req)
	if err != nil {
		t.Fatalf("CreateDeploymentFromTemplate がエラーを返しました: %v", err) // エラーにならず両方作成されることを期待する
	}
	if len(capturedEnvVars) != 2 { // 両方作成されたことを確認する
		t.Fatalf("期待する作成件数: 2, 実際: %d", len(capturedEnvVars))
	}
	if capturedEnvVars[0].Key != "DUPLICATE_KEY" { // 1件目はそのままのキーであることを確認する
		t.Errorf("期待するキー: DUPLICATE_KEY, 実際: %s", capturedEnvVars[0].Key)
	}
	if !strings.HasPrefix(capturedEnvVars[1].Key, "DUPLICATE_KEY_") { // 2件目はリネームされていることを確認する
		t.Errorf("期待するプレフィックス: DUPLICATE_KEY_, 実際: %s", capturedEnvVars[1].Key)
	}
}

// TestCreateDeploymentFromTemplate_ExtraEnvVarsはTemplateIDが設定されない はExtraEnvVarsが手動追加扱いになることを確認する
func TestCreateDeploymentFromTemplate_ExtraEnvVarsはTemplateIDが設定されない_service(t *testing.T) {
	templateData := &models.DeploymentTemplate{
		ID:           "template-id-1",
		ImageURL:     "image:latest",
		InstanceSize: "small",
		Replicas:     1,
	}

	var capturedEnvVars []*models.EnvVar // 作成された env_var を記録する

	templateRepo := &mockDeploymentTemplateRepository{
		findByIDFunc: func(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) {
			return templateData, nil
		},
	}
	deploymentRepo := &mockDeploymentRepositoryForTemplate{
		createWithTxFunc: func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment) error {
			deployment.ID = "deployment-id-1"
			return nil
		},
	}
	envVarRepo := &mockEnvVarRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]*models.EnvVar, error) {
			return []*models.EnvVar{}, nil
		},
		createFunc: func(ctx context.Context, tx *gorm.DB, envVar *models.EnvVar) error {
			envVar.ID = "env-var-id-extra"
			capturedEnvVars = append(capturedEnvVars, envVar)
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil
		},
	}

	svc := newTestDeploymentTemplateService(t, templateRepo, deploymentRepo, envVarRepo, projectRepo)

	req := CreateDeploymentFromTemplateRequest{
		ProjectID:    "project-id-1",
		TemplateID:   "template-id-1",
		Name:         "test-deploy",
		ExtraEnvVars: []ExtraEnvVar{{Key: "EXTRA_KEY", Value: "extra-val"}}, // テンプレート外の追加env_var
	}
	_, err := svc.CreateDeploymentFromTemplate(context.Background(), "test-user-id", req)
	if err != nil {
		t.Fatalf("CreateDeploymentFromTemplate がエラーを返しました: %v", err)
	}
	if len(capturedEnvVars) != 1 { // ExtraEnvVars が1件作成されたことを確認する
		t.Fatalf("期待する作成件数: 1, 実際: %d", len(capturedEnvVars))
	}
	if capturedEnvVars[0].TemplateID != nil { // TemplateID が設定されていないことを確認する（手動追加扱い）
		t.Errorf("ExtraEnvVars の TemplateID は nil であるべきですが、%v が設定されています", *capturedEnvVars[0].TemplateID)
	}
}
