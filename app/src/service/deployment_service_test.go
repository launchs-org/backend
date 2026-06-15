package service

import (
	"app/models"
	"context"
	"errors"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/apimachinery/pkg/runtime"
)

// mockDeploymentRepository は DeploymentRepository のテスト用モック実装
type mockDeploymentRepository struct {
	createFunc                        func(ctx context.Context, deployment *models.Deployment) error
	findByIDFunc                      func(ctx context.Context, deploymentID string) (*models.Deployment, error)
	findByIDForUpdateFunc             func(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error)
	findAllByProjectIDFunc            func(ctx context.Context, projectID string) ([]models.Deployment, error)
	saveFunc                          func(ctx context.Context, deployment *models.Deployment) error
	updatesFunc                       func(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error
	updatePendingGithubCommitSHAFunc  func(ctx context.Context, deploymentID string, commitSHA string) error
}

func (mock *mockDeploymentRepository) Create(ctx context.Context, deployment *models.Deployment) error {
	return mock.createFunc(ctx, deployment) // モック関数を呼び出す
}

func (mock *mockDeploymentRepository) FindByID(ctx context.Context, deploymentID string) (*models.Deployment, error) {
	return mock.findByIDFunc(ctx, deploymentID) // モック関数を呼び出す
}

func (mock *mockDeploymentRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, deploymentID string) (*models.Deployment, error) {
	if mock.findByIDForUpdateFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDForUpdateFunc(ctx, tx, deploymentID)
	}
	return nil, nil // デフォルトは nil を返す
}

func (mock *mockDeploymentRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return mock.findAllByProjectIDFunc(ctx, projectID) // モック関数を呼び出す
}

func (mock *mockDeploymentRepository) Save(ctx context.Context, deployment *models.Deployment) error {
	return mock.saveFunc(ctx, deployment) // モック関数を呼び出す
}

func (mock *mockDeploymentRepository) Updates(ctx context.Context, tx *gorm.DB, deployment *models.Deployment, values map[string]interface{}) error {
	if mock.updatesFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updatesFunc(ctx, tx, deployment, values)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockDeploymentRepository) UpdateAppStatus(ctx context.Context, deploymentID string, appStatus models.AppStatus) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepository) UpdateK8sStatus(ctx context.Context, deploymentID string, k8sStatus datatypes.JSON) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepository) UpdatePendingImageURL(ctx context.Context, deploymentID string, imageURL string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

func (mock *mockDeploymentRepository) UpdatePendingGithubCommitSHA(ctx context.Context, deploymentID string, commitSHA string) error {
	if mock.updatePendingGithubCommitSHAFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updatePendingGithubCommitSHAFunc(ctx, deploymentID, commitSHA)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockDeploymentRepository) Delete(ctx context.Context, deploymentID string) error {
	return nil // テストでは使用しないためデフォルト nil を返す
}

// mockIngressRouteRepository は IngressRouteRepository のテスト用モック実装
type mockIngressRouteRepository struct {
	createFunc              func(ctx context.Context, ingressRoute *models.IngressRoute) error
	findByIDFunc            func(ctx context.Context, ingressRouteID string) (*models.IngressRoute, error)
	findByDeploymentIDFunc  func(ctx context.Context, deploymentID string) (*models.IngressRoute, error)
	updateFunc              func(ctx context.Context, ingressRoute *models.IngressRoute) error
	updateStatusFunc        func(ctx context.Context, ingressRouteID string, status models.IngressRouteStatus, k8sStatus datatypes.JSON) error
}

func (mock *mockIngressRouteRepository) Create(ctx context.Context, ingressRoute *models.IngressRoute) error {
	if mock.createFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createFunc(ctx, ingressRoute)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockIngressRouteRepository) FindByID(ctx context.Context, ingressRouteID string) (*models.IngressRoute, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, ingressRouteID)
	}
	return &models.IngressRoute{ID: ingressRouteID}, nil // デフォルトは空の ingress_route を返す
}

func (mock *mockIngressRouteRepository) FindByDeploymentID(ctx context.Context, deploymentID string) (*models.IngressRoute, error) {
	if mock.findByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDFunc(ctx, deploymentID)
	}
	return &models.IngressRoute{DeploymentID: deploymentID}, nil // デフォルトは空の ingress_route を返す
}

func (mock *mockIngressRouteRepository) Update(ctx context.Context, ingressRoute *models.IngressRoute) error {
	if mock.updateFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateFunc(ctx, ingressRoute)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockIngressRouteRepository) UpdateStatus(ctx context.Context, ingressRouteID string, status models.IngressRouteStatus, k8sStatus datatypes.JSON) error {
	if mock.updateStatusFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateStatusFunc(ctx, ingressRouteID, status, k8sStatus)
	}
	return nil // デフォルトは nil を返す
}

// mockServiceRepository は ServiceRepository のテスト用モック実装
type mockServiceRepository struct {
	createFunc              func(ctx context.Context, service *models.Service) error
	findByDeploymentIDFunc  func(ctx context.Context, deploymentID string) (*models.Service, error)
	findByServiceIDFunc     func(ctx context.Context, serviceID string) (*models.Service, error)
	updateFunc              func(ctx context.Context, service *models.Service) error
	updateStatusFunc        func(ctx context.Context, serviceID string, status models.ServiceStatus, k8sStatus datatypes.JSON) error
}

func (mock *mockServiceRepository) Create(ctx context.Context, service *models.Service) error {
	return mock.createFunc(ctx, service) // モック関数を呼び出す
}

func (mock *mockServiceRepository) FindByDeploymentID(ctx context.Context, deploymentID string) (*models.Service, error) {
	if mock.findByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDFunc(ctx, deploymentID)
	}
	return &models.Service{DeploymentID: deploymentID}, nil // デフォルトは空の service を返す
}

func (mock *mockServiceRepository) FindByServiceID(ctx context.Context, serviceID string) (*models.Service, error) {
	if mock.findByServiceIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByServiceIDFunc(ctx, serviceID)
	}
	return &models.Service{ID: serviceID}, nil // デフォルトは空の service を返す
}

func (mock *mockServiceRepository) Update(ctx context.Context, service *models.Service) error {
	if mock.updateFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateFunc(ctx, service)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockServiceRepository) UpdateStatus(ctx context.Context, serviceID string, status models.ServiceStatus, k8sStatus datatypes.JSON) error {
	if mock.updateStatusFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateStatusFunc(ctx, serviceID, status, k8sStatus)
	}
	return nil // デフォルトは nil を返す
}

// mockProjectRepository は ProjectRepository のテスト用モック実装（所有権チェック用）
type mockProjectRepository struct {
	findByIDNoTxFunc func(ctx context.Context, projectID string) (*models.Project, error)
}

func (mock *mockProjectRepository) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}

func (mock *mockProjectRepository) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *mockProjectRepository) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	if mock.findByIDNoTxFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDNoTxFunc(ctx, projectID)
	}
	return &models.Project{UserID: "test-user-id"}, nil // デフォルトは所有者として返す
}

func (mock *mockProjectRepository) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *mockProjectRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil // 使用しない
}

func (mock *mockProjectRepository) Save(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}

func (mock *mockProjectRepository) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *mockProjectRepository) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}

func (mock *mockProjectRepository) DeleteNoTx(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}


// newTestDeploymentService はテスト用のデフォルト DeploymentService を生成するヘルパー関数
func newTestDeploymentService(deploymentRepo *mockDeploymentRepository, serviceRepo *mockServiceRepository, projectRepo *mockProjectRepository, ingressRouteRepo *mockIngressRouteRepository) DeploymentService {
	fakeK8sClient := k8sfake.NewSimpleClientset()                                                                          // fake k8s クライアントを生成する
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())                                            // fake dynamic クライアントを生成する
	return NewDeploymentService(deploymentRepo, serviceRepo, projectRepo, ingressRouteRepo, &mockEnvVarMountRepository{}, &mockVolumeMountRepository{}, fakeK8sClient, fakeDynamicClient) // サービスを生成する
}

// TestCreateDeployment_正常に作成されpendingフィールドに値が入る は POST で全フィールドが pending_*** に入ることを確認する
func TestCreateDeployment_正常に作成されpendingフィールドに値が入る(t *testing.T) {
	var capturedDeployment *models.Deployment // キャプチャした deployment を格納する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		createFunc: func(ctx context.Context, deployment *models.Deployment) error {
			capturedDeployment = deployment // deployment をキャプチャする
			deployment.ID = "deployment-id-1" // ID を付与する
			return nil
		},
	}
	serviceRepo := &mockServiceRepository{
		createFunc: func(ctx context.Context, svc *models.Service) error {
			return nil // 正常終了を返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	req := CreateDeploymentRequest{
		ProjectID:    "project-id-1",  // プロジェクト ID を設定する
		Name:         "my-app",        // デプロイメント名を設定する
		Type:         "image_url",     // タイプを設定する
		ImageURL:     "nginx:latest",  // image_url を設定する
		InstanceSize: "small",         // インスタンスサイズを設定する
		Replicas:     2,               // レプリカ数を設定する
	}

	result, err := deploymentSvc.CreateDeployment(context.Background(), req) // サービスを実行する
	if err != nil {
		t.Fatalf("CreateDeployment がエラーを返しました: %v", err)
	}
	if result.Status != models.DeploymentStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", result.Status)
	}
	if result.AppStatus != models.AppStatusPending { // app_status が pending であることを確認する
		t.Errorf("期待する app_status: pending, 実際の app_status: %s", result.AppStatus)
	}
	if capturedDeployment.PendingImageURL != "nginx:latest" { // pending_image_url が設定されていることを確認する
		t.Errorf("期待する pending_image_url: nginx:latest, 実際の pending_image_url: %s", capturedDeployment.PendingImageURL)
	}
	if capturedDeployment.PendingInstanceSize != "small" { // pending_instance_size が設定されていることを確認する
		t.Errorf("期待する pending_instance_size: small, 実際の pending_instance_size: %s", capturedDeployment.PendingInstanceSize)
	}
	if capturedDeployment.PendingReplicas != 2 { // pending_replicas が設定されていることを確認する
		t.Errorf("期待する pending_replicas: 2, 実際の pending_replicas: %d", capturedDeployment.PendingReplicas)
	}
}

// TestCreateDeployment_デフォルト値が適用される はデフォルト値が正しく設定されることを確認する
func TestCreateDeployment_デフォルト値が適用される(t *testing.T) {
	var capturedDeployment *models.Deployment // キャプチャした deployment を格納する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		createFunc: func(ctx context.Context, deployment *models.Deployment) error {
			capturedDeployment = deployment // deployment をキャプチャする
			return nil
		},
	}
	serviceRepo := &mockServiceRepository{
		createFunc: func(ctx context.Context, svc *models.Service) error {
			return nil // 正常終了を返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	req := CreateDeploymentRequest{
		ProjectID: "project-id-1", // 最低限の情報のみ設定する
		Name:      "my-app",
		Type:      "image_url",
	}

	_, err := deploymentSvc.CreateDeployment(context.Background(), req) // サービスを実行する
	if err != nil {
		t.Fatalf("CreateDeployment がエラーを返しました: %v", err)
	}
	if capturedDeployment.PendingInstanceSize != "small" { // instance_size のデフォルト値を確認する
		t.Errorf("期待する pending_instance_size: small, 実際の pending_instance_size: %s", capturedDeployment.PendingInstanceSize)
	}
	if capturedDeployment.PendingReplicas != 1 { // replicas のデフォルト値を確認する
		t.Errorf("期待する pending_replicas: 1, 実際の pending_replicas: %d", capturedDeployment.PendingReplicas)
	}
	if capturedDeployment.PendingDockerfilePath != "./Dockerfile" { // dockerfile_path のデフォルト値を確認する
		t.Errorf("期待する pending_dockerfile_path: ./Dockerfile, 実際の pending_dockerfile_path: %s", capturedDeployment.PendingDockerfilePath)
	}
	if capturedDeployment.PendingGithubRepoDirectory != "./" { // build_directory のデフォルト値を確認する
		t.Errorf("期待する pending_github_repo_directory: ./, 実際の pending_github_repo_directory: %s", capturedDeployment.PendingGithubRepoDirectory)
	}
}

// TestCreateDeployment_Serviceレコードも作成される は Service レコードが同時に作成されることを確認する
func TestCreateDeployment_Serviceレコードも作成される(t *testing.T) {
	var capturedService *models.Service // キャプチャした service を格納する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		createFunc: func(ctx context.Context, deployment *models.Deployment) error {
			deployment.ID = "deployment-id-1" // ID を付与する
			return nil
		},
	}
	serviceRepo := &mockServiceRepository{
		createFunc: func(ctx context.Context, svc *models.Service) error {
			capturedService = svc // service をキャプチャする
			return nil
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	req := CreateDeploymentRequest{ProjectID: "project-id-1", Name: "my-app", Type: "image_url"}

	_, err := deploymentSvc.CreateDeployment(context.Background(), req) // サービスを実行する
	if err != nil {
		t.Fatalf("CreateDeployment がエラーを返しました: %v", err)
	}
	if capturedService == nil { // Service レコードが作成されていることを確認する
		t.Fatal("Service レコードが作成されていません")
	}
	if capturedService.DeploymentID != "deployment-id-1" { // deployment_id が設定されていることを確認する
		t.Errorf("期待する deployment_id: deployment-id-1, 実際の deployment_id: %s", capturedService.DeploymentID)
	}
	if capturedService.Status != models.ServiceStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", capturedService.Status)
	}
}

// TestCreateDeployment_Deployment作成失敗時にエラーを返す は Deployment 作成失敗時にエラーが返ることを確認する
func TestCreateDeployment_Deployment作成失敗時にエラーを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		createFunc: func(ctx context.Context, deployment *models.Deployment) error {
			return errors.New("DB エラー") // 作成失敗を返す
		},
	}
	serviceRepo := &mockServiceRepository{
		createFunc: func(ctx context.Context, svc *models.Service) error {
			return nil
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	req := CreateDeploymentRequest{ProjectID: "project-id-1", Name: "my-app", Type: "image_url"}

	_, err := deploymentSvc.CreateDeployment(context.Background(), req) // サービスを実行する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("エラーが返るべきですが nil が返りました")
	}
}

// TestCreateDeployment_Service作成失敗時にエラーを返す は Service 作成失敗時にエラーが返ることを確認する
func TestCreateDeployment_Service作成失敗時にエラーを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		createFunc: func(ctx context.Context, deployment *models.Deployment) error {
			deployment.ID = "deployment-id-1" // ID を付与する
			return nil
		},
	}
	serviceRepo := &mockServiceRepository{
		createFunc: func(ctx context.Context, svc *models.Service) error {
			return errors.New("DB エラー") // Service 作成失敗を返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	req := CreateDeploymentRequest{ProjectID: "project-id-1", Name: "my-app", Type: "image_url"}

	_, err := deploymentSvc.CreateDeployment(context.Background(), req) // サービスを実行する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("エラーが返るべきですが nil が返りました")
	}
}

// TestUpdateDeployment_送ったフィールドのみpendingが更新される は送ったフィールドのみ更新されることを確認する
func TestUpdateDeployment_送ったフィールドのみpendingが更新される(t *testing.T) {
	originalDeployment := &models.Deployment{
		ID:                  "deployment-id-1",
		PendingImageURL:     "nginx:1.24",  // 更新前の値を設定する
		PendingInstanceSize: "small",
		PendingReplicas:     1,
	}

	var savedDeployment *models.Deployment // 保存された deployment を格納する変数を定義する
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return originalDeployment, nil // 元の deployment を返す
		},
		saveFunc: func(ctx context.Context, deployment *models.Deployment) error {
			savedDeployment = deployment // 保存された deployment をキャプチャする
			return nil
		},
	}
	serviceRepo := &mockServiceRepository{}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	newImageURL := "nginx:1.25"                                        // 更新する image_url を設定する
	req := UpdateDeploymentRequest{
		ImageURL: &newImageURL, // image_url のみ送る
	}

	_, err := deploymentSvc.UpdateDeployment(context.Background(), "test-user-id", "deployment-id-1", req) // サービスを実行する
	if err != nil {
		t.Fatalf("UpdateDeployment がエラーを返しました: %v", err)
	}
	if savedDeployment.PendingImageURL != "nginx:1.25" { // image_url が更新されていることを確認する
		t.Errorf("期待する pending_image_url: nginx:1.25, 実際の pending_image_url: %s", savedDeployment.PendingImageURL)
	}
	if savedDeployment.PendingInstanceSize != "small" { // 送っていない instance_size が変化していないことを確認する
		t.Errorf("instance_size は変化しないはずですが変化しています: %s", savedDeployment.PendingInstanceSize)
	}
	if savedDeployment.PendingReplicas != 1 { // 送っていない replicas が変化していないことを確認する
		t.Errorf("replicas は変化しないはずですが変化しています: %d", savedDeployment.PendingReplicas)
	}
}

// TestUpdateDeployment_存在しないdeploymentはエラーを返す は存在しない deployment ID でエラーが返ることを確認する
func TestUpdateDeployment_存在しないdeploymentはエラーを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // レコードが存在しないエラーを返す
		},
	}
	serviceRepo := &mockServiceRepository{}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する
	req := UpdateDeploymentRequest{}

	_, err := deploymentSvc.UpdateDeployment(context.Background(), "test-user-id", "nonexistent", req) // サービスを実行する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("エラーが返るべきですが nil が返りました")
	}
}

// TestDeleteDeployment_statusがdeletingになる は status が deleting に変更されることを確認する
func TestDeleteDeployment_statusがdeletingになる(t *testing.T) {
	originalDeployment := &models.Deployment{
		ID:     "deployment-id-1",
		Status: models.DeploymentStatusRunning, // 更新前の status を設定する
	}

	var savedDeployment *models.Deployment // 保存された deployment を格納する変数を定義する
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return originalDeployment, nil // 元の deployment を返す
		},
		saveFunc: func(ctx context.Context, deployment *models.Deployment) error {
			savedDeployment = deployment // 保存された deployment をキャプチャする
			return nil
		},
	}
	serviceRepo := &mockServiceRepository{}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する

	result, err := deploymentSvc.DeleteDeployment(context.Background(), "test-user-id", "deployment-id-1") // サービスを実行する
	if err != nil {
		t.Fatalf("DeleteDeployment がエラーを返しました: %v", err)
	}
	if result.Status != models.DeploymentStatusDeleting { // status が deleting であることを確認する
		t.Errorf("期待する status: deleting, 実際の status: %s", result.Status)
	}
	if savedDeployment.Status != models.DeploymentStatusDeleting { // 保存された status が deleting であることを確認する
		t.Errorf("保存された status が deleting ではありません: %s", savedDeployment.Status)
	}
}

// TestDeleteDeployment_存在しないdeploymentはエラーを返す は存在しない deployment ID でエラーが返ることを確認する
func TestDeleteDeployment_存在しないdeploymentはエラーを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // レコードが存在しないエラーを返す
		},
	}
	serviceRepo := &mockServiceRepository{}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する

	_, err := deploymentSvc.DeleteDeployment(context.Background(), "test-user-id", "nonexistent") // サービスを実行する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("エラーが返るべきですが nil が返りました")
	}
}

// TestDeleteDeployment_k8sリソースが削除される は DeleteDeployment で k8s Deployment が削除されることを確認する
func TestDeleteDeployment_k8sリソースが削除される(t *testing.T) {
	originalDeployment := &models.Deployment{
		ID:        "deployment-id-k8s",
		Name:      "test-deploy",
		ProjectID: "project-id-1",
		Status:    models.DeploymentStatusRunning, // 更新前の status を設定する
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return originalDeployment, nil // 元の deployment を返す
		},
		saveFunc: func(ctx context.Context, deployment *models.Deployment) error {
			return nil // 保存成功を返す
		},
	}
	serviceRepo := &mockServiceRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "test-user-id", Namespace: "test-ns"}, nil // namespace 付きの project を返す
		},
	}
	ingressRouteRepo := &mockIngressRouteRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) (*models.IngressRoute, error) {
			return nil, gorm.ErrRecordNotFound // IngressRoute が存在しない場合を想定する
		},
	}

	fakeK8sClient := k8sfake.NewSimpleClientset()                             // fake k8s クライアントを生成する
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()) // fake dynamic クライアントを生成する
	deploymentSvc := NewDeploymentService(deploymentRepo, serviceRepo, projectRepo, ingressRouteRepo, &mockEnvVarMountRepository{}, &mockVolumeMountRepository{}, fakeK8sClient, fakeDynamicClient) // サービスを生成する

	result, err := deploymentSvc.DeleteDeployment(context.Background(), "test-user-id", "deployment-id-k8s") // サービスを実行する
	if err != nil {
		t.Fatalf("DeleteDeployment がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}
	if result.Status != models.DeploymentStatusDeleting { // status が deleting であることを確認する
		t.Errorf("期待する status: deleting, 実際の status: %s", result.Status)
	}
}

// TestListDeployments_正常に一覧が返る は deployment 一覧が返ることを確認する
func TestListDeployments_正常に一覧が返る(t *testing.T) {
	expectedList := []models.Deployment{
		{ID: "deployment-id-1", Name: "app-1", ProjectID: "project-id-1"},
		{ID: "deployment-id-2", Name: "app-2", ProjectID: "project-id-1"},
	}

	deploymentRepo := &mockDeploymentRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]models.Deployment, error) {
			return expectedList, nil // 期待する一覧を返す
		},
	}
	serviceRepo := &mockServiceRepository{}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, &mockProjectRepository{}, &mockIngressRouteRepository{}) // サービスを生成する

	result, err := deploymentSvc.ListDeployments(context.Background(), "project-id-1") // サービスを実行する
	if err != nil {
		t.Fatalf("ListDeployments がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // deployment 数を確認する
		t.Errorf("期待する deployment 数: 2, 実際の deployment 数: %d", len(result))
	}
}

// TestGetService_正常にService設定が取得される は GetService で service が返ることを確認する
func TestGetService_正常にService設定が取得される(t *testing.T) {
	expectedService := &models.Service{
		DeploymentID: "deployment-id-1", // デプロイメント ID を設定する
		Port:         8080,              // ポート番号を設定する
		TargetPort:   3000,              // ターゲットポートを設定する
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	serviceRepo := &mockServiceRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) (*models.Service, error) {
			return expectedService, nil // service を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, projectRepo, &mockIngressRouteRepository{}) // サービスを生成する

	result, err := deploymentSvc.GetService(context.Background(), "test-user-id", "deployment-id-1") // サービスを実行する
	if err != nil {
		t.Fatalf("GetService がエラーを返しました: %v", err)
	}
	if result.Port != 8080 { // ポート番号が一致することを確認する
		t.Errorf("期待する port: 8080, 実際の port: %d", result.Port)
	}
	if result.TargetPort != 3000 { // ターゲットポートが一致することを確認する
		t.Errorf("期待する target_port: 3000, 実際の target_port: %d", result.TargetPort)
	}
}

// TestGetService_他ユーザーのDeploymentはErrForbiddenを返す は所有者でない場合に ErrForbidden が返ることを確認する
func TestGetService_他ユーザーのDeploymentはErrForbiddenを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	serviceRepo := &mockServiceRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "other-user-id"}, nil // 別ユーザーとして返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, projectRepo, &mockIngressRouteRepository{}) // サービスを生成する

	_, err := deploymentSvc.GetService(context.Background(), "test-user-id", "deployment-id-1") // サービスを実行する
	if !errors.Is(err, ErrForbidden) { // ErrForbidden が返ることを確認する
		t.Errorf("ErrForbidden が返るべきですが実際のエラー: %v", err)
	}
}

// TestUpdateService_pendingフィールドが更新される は UpdateService で pending フィールドが更新されることを確認する
func TestUpdateService_pendingフィールドが更新される(t *testing.T) {
	originalService := &models.Service{
		DeploymentID:      "deployment-id-1",
		PendingPort:       8080, // 更新前の値を設定する
		PendingTargetPort: 3000, // 更新前の値を設定する
	}

	var savedService *models.Service // 保存された service を格納する変数を定義する
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	serviceRepo := &mockServiceRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) (*models.Service, error) {
			return originalService, nil // 元の service を返す
		},
		updateFunc: func(ctx context.Context, service *models.Service) error {
			savedService = service // 保存された service をキャプチャする
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, projectRepo, &mockIngressRouteRepository{}) // サービスを生成する
	newPort := 9090                                                                   // 更新するポート番号を設定する
	req := UpdateServiceRequest{
		Port: &newPort, // port のみ送る
	}

	result, err := deploymentSvc.UpdateService(context.Background(), "test-user-id", "deployment-id-1", req) // サービスを実行する
	if err != nil {
		t.Fatalf("UpdateService がエラーを返しました: %v", err)
	}
	if result.PendingPort != 9090 { // pending_port が更新されていることを確認する
		t.Errorf("期待する pending_port: 9090, 実際の pending_port: %d", result.PendingPort)
	}
	if savedService.PendingTargetPort != 3000 { // 送っていない pending_target_port が変化していないことを確認する
		t.Errorf("pending_target_port は変化しないはずですが変化しています: %d", savedService.PendingTargetPort)
	}
}

// TestUpdateService_他ユーザーのDeploymentはErrForbiddenを返す は所有者でない場合に ErrForbidden が返ることを確認する
func TestUpdateService_他ユーザーのDeploymentはErrForbiddenを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	serviceRepo := &mockServiceRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "other-user-id"}, nil // 別ユーザーとして返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, serviceRepo, projectRepo, &mockIngressRouteRepository{}) // サービスを生成する
	req := UpdateServiceRequest{}

	_, err := deploymentSvc.UpdateService(context.Background(), "test-user-id", "deployment-id-1", req) // サービスを実行する
	if !errors.Is(err, ErrForbidden) { // ErrForbidden が返ることを確認する
		t.Errorf("ErrForbidden が返るべきですが実際のエラー: %v", err)
	}
}

// TestCreateIngressRoute_正常に作成される は POST で IngressRoute レコードが作成されることを確認する
func TestCreateIngressRoute_正常に作成される(t *testing.T) {
	var capturedIngressRoute *models.IngressRoute // キャプチャした ingress_route を格納する変数を定義する

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	ingressRouteRepo := &mockIngressRouteRepository{
		createFunc: func(ctx context.Context, ingressRoute *models.IngressRoute) error {
			capturedIngressRoute = ingressRoute // ingress_route をキャプチャする
			return nil                          // 正常終了を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, &mockServiceRepository{}, projectRepo, ingressRouteRepo) // サービスを生成する
	req := CreateIngressRouteRequest{
		Host:       "example.launchs.org", // ホスト名を設定する
		PathPrefix: "/",                   // パスプレフィックスを設定する
		Port:       8080,                  // ポート番号を設定する
		TLSEnabled: true,                  // TLS を有効にする
	}

	result, err := deploymentSvc.CreateIngressRoute(context.Background(), "test-user-id", "deployment-id-1", req) // サービスを実行する
	if err != nil {
		t.Fatalf("CreateIngressRoute がエラーを返しました: %v", err)
	}
	if result.Status != models.IngressRouteStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", result.Status)
	}
	if capturedIngressRoute.Host != "example.launchs.org" { // host が設定されていることを確認する
		t.Errorf("期待する host: example.launchs.org, 実際の host: %s", capturedIngressRoute.Host)
	}
	if capturedIngressRoute.Port != 8080 { // port が設定されていることを確認する
		t.Errorf("期待する port: 8080, 実際の port: %d", capturedIngressRoute.Port)
	}
	if !capturedIngressRoute.TLSEnabled { // tls_enabled が設定されていることを確認する
		t.Errorf("期待する tls_enabled: true, 実際の tls_enabled: %v", capturedIngressRoute.TLSEnabled)
	}
}

// TestCreateIngressRoute_他ユーザーのDeploymentはErrForbiddenを返す は所有者でない場合に ErrForbidden が返ることを確認する
func TestCreateIngressRoute_他ユーザーのDeploymentはErrForbiddenを返す(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "other-user-id"}, nil // 別ユーザーとして返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, &mockServiceRepository{}, projectRepo, &mockIngressRouteRepository{}) // サービスを生成する
	req := CreateIngressRouteRequest{Host: "example.launchs.org", Port: 8080}

	_, err := deploymentSvc.CreateIngressRoute(context.Background(), "test-user-id", "deployment-id-1", req) // サービスを実行する
	if !errors.Is(err, ErrForbidden) { // ErrForbidden が返ることを確認する
		t.Errorf("ErrForbidden が返るべきですが実際のエラー: %v", err)
	}
}

// TestGetIngressRoute_正常にIngress設定が取得される は GetIngressRoute で ingress_route が返ることを確認する
func TestGetIngressRoute_正常にIngress設定が取得される(t *testing.T) {
	expectedIngressRoute := &models.IngressRoute{
		DeploymentID: "deployment-id-1",   // デプロイメント ID を設定する
		Host:         "example.launchs.org", // ホスト名を設定する
		Port:         8080,                // ポート番号を設定する
	}

	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	ingressRouteRepo := &mockIngressRouteRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) (*models.IngressRoute, error) {
			return expectedIngressRoute, nil // ingress_route を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, &mockServiceRepository{}, projectRepo, ingressRouteRepo) // サービスを生成する

	result, err := deploymentSvc.GetIngressRoute(context.Background(), "test-user-id", "deployment-id-1") // サービスを実行する
	if err != nil {
		t.Fatalf("GetIngressRoute がエラーを返しました: %v", err)
	}
	if result.Host != "example.launchs.org" { // host が一致することを確認する
		t.Errorf("期待する host: example.launchs.org, 実際の host: %s", result.Host)
	}
	if result.Port != 8080 { // port が一致することを確認する
		t.Errorf("期待する port: 8080, 実際の port: %d", result.Port)
	}
}

// TestUpdateIngressRoute_pendingフィールドが更新される は UpdateIngressRoute で pending フィールドが更新されることを確認する
func TestUpdateIngressRoute_pendingフィールドが更新される(t *testing.T) {
	originalIngressRoute := &models.IngressRoute{
		DeploymentID:      "deployment-id-1",
		PendingPathPrefix: "/old", // 更新前の値を設定する
		PendingPort:       8080,   // 更新前の値を設定する
	}

	var savedIngressRoute *models.IngressRoute // 保存された ingress_route を格納する変数を定義する
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	ingressRouteRepo := &mockIngressRouteRepository{
		findByDeploymentIDFunc: func(ctx context.Context, deploymentID string) (*models.IngressRoute, error) {
			return originalIngressRoute, nil // 元の ingress_route を返す
		},
		updateFunc: func(ctx context.Context, ingressRoute *models.IngressRoute) error {
			savedIngressRoute = ingressRoute // 保存された ingress_route をキャプチャする
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	deploymentSvc := newTestDeploymentService(deploymentRepo, &mockServiceRepository{}, projectRepo, ingressRouteRepo) // サービスを生成する
	newPathPrefix := "/new"                                                                                          // 更新するパスプレフィックスを設定する
	req := UpdateIngressRouteRequest{
		PathPrefix: &newPathPrefix, // path_prefix のみ送る
	}

	result, err := deploymentSvc.UpdateIngressRoute(context.Background(), "test-user-id", "deployment-id-1", req) // サービスを実行する
	if err != nil {
		t.Fatalf("UpdateIngressRoute がエラーを返しました: %v", err)
	}
	if result.PendingPathPrefix != "/new" { // pending_path_prefix が更新されていることを確認する
		t.Errorf("期待する pending_path_prefix: /new, 実際の pending_path_prefix: %s", result.PendingPathPrefix)
	}
	if savedIngressRoute.PendingPort != 8080 { // 送っていない pending_port が変化していないことを確認する
		t.Errorf("pending_port は変化しないはずですが変化しています: %d", savedIngressRoute.PendingPort)
	}
}
