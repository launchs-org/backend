package service

import (
	"app/models"
	"context"
	"errors"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// mockVolumeRepository は VolumeRepository のテスト用モック実装
type mockVolumeRepository struct {
	createFunc             func(ctx context.Context, tx *gorm.DB, volume *models.Volume) error
	findByIDFunc           func(ctx context.Context, volumeID string) (*models.Volume, error)
	findAllByProjectIDFunc func(ctx context.Context, projectID string) ([]*models.Volume, error)
	deleteFunc             func(ctx context.Context, tx *gorm.DB, volume *models.Volume) error
	updateStatusFunc       func(ctx context.Context, volumeID string, status models.VolumeStatus, k8sStatus datatypes.JSON) error
}

func (mock *mockVolumeRepository) Create(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
	return mock.createFunc(ctx, tx, volume) // モック関数を呼び出す
}

func (mock *mockVolumeRepository) FindByID(ctx context.Context, volumeID string) (*models.Volume, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, volumeID)
	}
	return &models.Volume{ID: volumeID, ProjectID: "project-id-1"}, nil // デフォルトは volume を返す
}

func (mock *mockVolumeRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]*models.Volume, error) {
	if mock.findAllByProjectIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllByProjectIDFunc(ctx, projectID)
	}
	return []*models.Volume{}, nil // デフォルトは空一覧を返す
}

func (mock *mockVolumeRepository) Delete(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
	if mock.deleteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteFunc(ctx, tx, volume)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockVolumeRepository) UpdateStatus(ctx context.Context, volumeID string, status models.VolumeStatus, k8sStatus datatypes.JSON) error {
	if mock.updateStatusFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateStatusFunc(ctx, volumeID, status, k8sStatus)
	}
	return nil // デフォルトは nil を返す
}

// mockVolumeMountRepository は VolumeMountRepository のテスト用モック実装
type mockVolumeMountRepository struct {
	createFunc                        func(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error
	findByIDFunc                      func(ctx context.Context, mountID string) (*models.VolumeMount, error)
	findAllByDeploymentIDFunc         func(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error)
	findAllByVolumeIDFunc             func(ctx context.Context, volumeID string) ([]*models.VolumeMount, error)
	findByDeploymentIDAndMountPathFunc func(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error)
	deleteFunc                        func(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error
}

func (mock *mockVolumeMountRepository) Create(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	if mock.createFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createFunc(ctx, tx, mount)
	}
	return nil // デフォルトは nil を返す
}

func (mock *mockVolumeMountRepository) FindByID(ctx context.Context, mountID string) (*models.VolumeMount, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, mountID)
	}
	return &models.VolumeMount{ID: mountID, DeploymentID: "deployment-id-1"}, nil // デフォルトはマウント設定を返す
}

func (mock *mockVolumeMountRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error) {
	if mock.findAllByDeploymentIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllByDeploymentIDFunc(ctx, deploymentID)
	}
	return []*models.VolumeMount{}, nil // デフォルトは空一覧を返す
}

func (mock *mockVolumeMountRepository) FindAllByVolumeID(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) {
	if mock.findAllByVolumeIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findAllByVolumeIDFunc(ctx, volumeID)
	}
	return []*models.VolumeMount{}, nil // デフォルトは空一覧を返す
}

func (mock *mockVolumeMountRepository) FindByDeploymentIDAndMountPath(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) {
	if mock.findByDeploymentIDAndMountPathFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByDeploymentIDAndMountPathFunc(ctx, deploymentID, mountPath)
	}
	return nil, gorm.ErrRecordNotFound // デフォルトはレコードなしを返す
}

func (mock *mockVolumeMountRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount, status models.VolumeMountStatus) error {
	return nil // テストでは使用しない
}

func (mock *mockVolumeMountRepository) DeleteAllByDeploymentID(ctx context.Context, tx *gorm.DB, deploymentID string) error {
	return nil // テストでは使用しない
}

func (mock *mockVolumeMountRepository) Delete(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
	if mock.deleteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteFunc(ctx, tx, mount)
	}
	return nil // デフォルトは nil を返す
}

// TestListVolumes_正常に一覧が取得される は ListVolumes が volume 一覧を返すことを確認する
func TestListVolumes_正常に一覧が取得される_service(t *testing.T) {
	expectedList := []*models.Volume{
		{ID: "volume-id-1", ProjectID: "project-id-1", Name: "vol-a", SizeMB: 512},  // volume 1件目
		{ID: "volume-id-2", ProjectID: "project-id-1", Name: "vol-b", SizeMB: 1024}, // volume 2件目
	}

	volumeRepo := &mockVolumeRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]*models.Volume, error) {
			return expectedList, nil // 一覧を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewVolumeService(nil, volumeRepo, &mockVolumeMountRepository{}, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する（db は未使用）

	result, err := svc.ListVolumes(context.Background(), "test-user-id", "project-id-1") // 一覧を取得する
	if err != nil {
		t.Fatalf("ListVolumes がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
}

// TestListVolumes_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestListVolumes_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	volumeRepo := &mockVolumeRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewVolumeService(nil, volumeRepo, &mockVolumeMountRepository{}, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	_, err := svc.ListVolumes(context.Background(), "other-user-id", "project-id-1") // 他ユーザーとして一覧を取得する
	if err != ErrForbidden {                                                           // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestCreateVolume_正常にvolumeが作成される は CreateVolume が volume を作成することを確認する
func TestCreateVolume_正常にvolumeが作成される_service(t *testing.T) {
	var capturedVolume *models.Volume // キャプチャした volume を格納する変数を定義する

	volumeRepo := &mockVolumeRepository{
		createFunc: func(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
			capturedVolume = volume       // volume をキャプチャする
			volume.ID = "new-volume-id"  // ID を付与する
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t)                                                                                    // テスト用 DB を準備する
	fakeK8sClient := k8sfake.NewSimpleClientset()                                                                   // fake k8s クライアントを生成する（PVC 作成に使用する）
	svc := NewVolumeService(db, volumeRepo, &mockVolumeMountRepository{}, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, fakeK8sClient, "") // サービスを生成する

	req := CreateVolumeRequest{Name: "my-volume", SizeMB: 512} // リクエストを定義する
	result, err := svc.CreateVolume(context.Background(), "test-user-id", "project-id-1", req) // volume を作成する
	if err != nil {
		t.Fatalf("CreateVolume がエラーを返しました: %v", err)
	}
	if result.ID != "new-volume-id" { // ID が付与されていることを確認する
		t.Errorf("期待する ID: new-volume-id, 実際の ID: %s", result.ID)
	}
	if capturedVolume.Name != "my-volume" { // 名前が正しく設定されていることを確認する
		t.Errorf("期待する Name: my-volume, 実際の Name: %s", capturedVolume.Name)
	}
	if capturedVolume.SizeMB != 512 { // サイズが正しく設定されていることを確認する
		t.Errorf("期待する SizeMB: 512, 実際の SizeMB: %d", capturedVolume.SizeMB)
	}
	if capturedVolume.ProjectID != "project-id-1" { // ProjectID が正しく設定されていることを確認する
		t.Errorf("期待する ProjectID: project-id-1, 実際の ProjectID: %s", capturedVolume.ProjectID)
	}
	if capturedVolume.Status != models.VolumeStatusPending { // ステータスが pending であることを確認する
		t.Errorf("期待する Status: pending, 実際の Status: %s", capturedVolume.Status)
	}
}

// TestCreateVolume_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestCreateVolume_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	volumeRepo := &mockVolumeRepository{}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewVolumeService(nil, volumeRepo, &mockVolumeMountRepository{}, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	req := CreateVolumeRequest{Name: "vol", SizeMB: 512} // リクエストを定義する
	_, err := svc.CreateVolume(context.Background(), "other-user-id", "project-id-1", req) // 他ユーザーとして作成する
	if err != ErrForbidden {                                                                 // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestDeleteVolume_mounted状態のマウントが存在する場合はErrVolumeMountedCannotDeleteを返す はガード処理を確認する
func TestDeleteVolume_mounted状態のマウントが存在する場合はErrVolumeMountedCannotDeleteを返す_service(t *testing.T) {
	volumeRepo := &mockVolumeRepository{
		findByIDFunc: func(ctx context.Context, volumeID string) (*models.Volume, error) {
			return &models.Volume{ID: volumeID, ProjectID: "project-id-1"}, nil // volume を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}
	volumeMountRepo := &mockVolumeMountRepository{
		findAllByVolumeIDFunc: func(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) {
			return []*models.VolumeMount{ // mounted 状態のマウントを返す
				{ID: "mount-id-1", VolumeID: volumeID, Status: models.VolumeMountStatusMounted},
			}, nil
		},
	}

	svc := NewVolumeService(nil, volumeRepo, volumeMountRepo, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	err := svc.DeleteVolume(context.Background(), "test-user-id", "volume-id-1") // 削除を試みる
	if !errors.Is(err, ErrVolumeMountedCannotDelete) {                            // ErrVolumeMountedCannotDelete であることを確認する
		t.Errorf("期待するエラー: ErrVolumeMountedCannotDelete, 実際のエラー: %v", err)
	}
}

// TestDeleteVolume_pendingのみのマウントは削除成功してPVCも削除される はガード通過と PVC 削除を確認する
func TestDeleteVolume_pendingのみのマウントは削除成功してPVCも削除される_service(t *testing.T) {
	deleteCalled := false // リポジトリの Delete が呼ばれたか記録する

	volumeRepo := &mockVolumeRepository{
		findByIDFunc: func(ctx context.Context, volumeID string) (*models.Volume, error) {
			return &models.Volume{ID: "volume-id-1", ProjectID: "project-id-1"}, nil // volume を返す
		},
		deleteFunc: func(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
			deleteCalled = true // 呼ばれたことを記録する
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id", Namespace: "ns-test"}, nil // 所有者として返す
		},
	}
	volumeMountRepo := &mockVolumeMountRepository{
		findAllByVolumeIDFunc: func(ctx context.Context, volumeID string) ([]*models.VolumeMount, error) {
			return []*models.VolumeMount{ // pending 状態のみのマウントを返す
				{ID: "mount-id-1", VolumeID: volumeID, Status: models.VolumeMountStatusPending},
			}, nil
		},
	}

	db := setupApplyTestDB(t)                   // テスト用 DB を準備する
	fakeK8sClient := k8sfake.NewSimpleClientset() // fake k8s クライアントを生成する
	svc := NewVolumeService(db, volumeRepo, volumeMountRepo, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, fakeK8sClient, "") // サービスを生成する

	err := svc.DeleteVolume(context.Background(), "test-user-id", "volume-id-1") // 削除を実行する
	if err != nil {                                                                // エラーが発生した場合は失敗する
		t.Fatalf("DeleteVolume がエラーを返しました: %v", err)
	}
	if !deleteCalled { // リポジトリの Delete が呼ばれていない場合は失敗する
		t.Error("volumeRepo.Delete が呼ばれていません")
	}
}

// TestDeleteVolume_マウントなしは削除成功してPVCも削除される はマウントが存在しない場合の正常系を確認する
func TestDeleteVolume_マウントなしは削除成功してPVCも削除される_service(t *testing.T) {
	deleteCalled := false // リポジトリの Delete が呼ばれたか記録する

	volumeRepo := &mockVolumeRepository{
		findByIDFunc: func(ctx context.Context, volumeID string) (*models.Volume, error) {
			return &models.Volume{ID: "volume-id-1", ProjectID: "project-id-1"}, nil // volume を返す
		},
		deleteFunc: func(ctx context.Context, tx *gorm.DB, volume *models.Volume) error {
			deleteCalled = true // 呼ばれたことを記録する
			return nil
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id", Namespace: "ns-test"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t)                   // テスト用 DB を準備する
	fakeK8sClient := k8sfake.NewSimpleClientset() // fake k8s クライアントを生成する
	svc := NewVolumeService(db, &mockVolumeRepository{
		findByIDFunc: volumeRepo.findByIDFunc,
		deleteFunc:   volumeRepo.deleteFunc,
	}, &mockVolumeMountRepository{}, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, fakeK8sClient, "") // サービスを生成する

	err := svc.DeleteVolume(context.Background(), "test-user-id", "volume-id-1") // 削除を実行する
	if err != nil {                                                                // エラーが発生した場合は失敗する
		t.Fatalf("DeleteVolume がエラーを返しました: %v", err)
	}
	if !deleteCalled { // リポジトリの Delete が呼ばれていない場合は失敗する
		t.Error("volumeRepo.Delete が呼ばれていません")
	}
}

// TestDeleteVolume_他ユーザーはErrForbiddenを返す は所有者でないユーザーが ErrForbidden を受け取ることを確認する
func TestDeleteVolume_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	volumeRepo := &mockVolumeRepository{
		findByIDFunc: func(ctx context.Context, volumeID string) (*models.Volume, error) {
			return &models.Volume{ID: volumeID, ProjectID: "project-id-1"}, nil // volume を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewVolumeService(nil, volumeRepo, &mockVolumeMountRepository{}, &mockDeploymentRepository{}, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	err := svc.DeleteVolume(context.Background(), "other-user-id", "volume-id-1") // 他ユーザーとして削除する
	if err != ErrForbidden {                                                        // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestListVolumeMounts_正常に一覧が取得される は ListVolumeMounts がマウント一覧を返すことを確認する
func TestListVolumeMounts_正常に一覧が取得される_service(t *testing.T) {
	expectedList := []*models.VolumeMount{
		{ID: "mount-id-1", DeploymentID: "deployment-id-1", VolumeID: "volume-id-1", MountPath: "/data"},  // マウント設定 1件目
		{ID: "mount-id-2", DeploymentID: "deployment-id-1", VolumeID: "volume-id-2", MountPath: "/logs"}, // マウント設定 2件目
	}

	volumeMountRepo := &mockVolumeMountRepository{
		findAllByDeploymentIDFunc: func(ctx context.Context, deploymentID string) ([]*models.VolumeMount, error) {
			return expectedList, nil // 一覧を返す
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewVolumeService(nil, &mockVolumeRepository{}, volumeMountRepo, deploymentRepo, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	result, err := svc.ListVolumeMounts(context.Background(), "test-user-id", "deployment-id-1") // 一覧を取得する
	if err != nil {
		t.Fatalf("ListVolumeMounts がエラーを返しました: %v", err)
	}
	if len(result) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(result))
	}
}

// TestListVolumeMounts_他ユーザーはErrForbiddenを返す は他ユーザーが ErrForbidden を受け取ることを確認する
func TestListVolumeMounts_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewVolumeService(nil, &mockVolumeRepository{}, &mockVolumeMountRepository{}, deploymentRepo, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	_, err := svc.ListVolumeMounts(context.Background(), "other-user-id", "deployment-id-1") // 他ユーザーとして一覧を取得する
	if err != ErrForbidden {                                                                   // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestCreateVolumeMount_正常にマウント設定が作成される は CreateVolumeMount がマウント設定を作成することを確認する
func TestCreateVolumeMount_正常にマウント設定が作成される_service(t *testing.T) {
	var capturedMount *models.VolumeMount // キャプチャしたマウント設定を格納する変数を定義する

	volumeMountRepo := &mockVolumeMountRepository{
		createFunc: func(ctx context.Context, tx *gorm.DB, mount *models.VolumeMount) error {
			capturedMount = mount         // マウント設定をキャプチャする
			mount.ID = "new-mount-id"    // ID を付与する
			return nil
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	db := setupApplyTestDB(t)                                                                                      // テスト用 DB を準備する
	svc := NewVolumeService(db, &mockVolumeRepository{}, volumeMountRepo, deploymentRepo, projectRepo, &noopUserQuotaRepository{}, nil, "")             // サービスを生成する

	req := CreateVolumeMountRequest{VolumeID: "volume-id-1", MountPath: "/data"} // リクエストを定義する
	result, err := svc.CreateVolumeMount(context.Background(), "test-user-id", "deployment-id-1", req)            // マウント設定を作成する
	if err != nil {
		t.Fatalf("CreateVolumeMount がエラーを返しました: %v", err)
	}
	if result.ID != "new-mount-id" { // ID が付与されていることを確認する
		t.Errorf("期待する ID: new-mount-id, 実際の ID: %s", result.ID)
	}
	if capturedMount.VolumeID != "volume-id-1" { // VolumeID が正しく設定されていることを確認する
		t.Errorf("期待する VolumeID: volume-id-1, 実際の VolumeID: %s", capturedMount.VolumeID)
	}
	if capturedMount.MountPath != "/data" { // MountPath が正しく設定されていることを確認する
		t.Errorf("期待する MountPath: /data, 実際の MountPath: %s", capturedMount.MountPath)
	}
	if capturedMount.Status != models.VolumeMountStatusPending { // ステータスが pending であることを確認する
		t.Errorf("期待する Status: pending, 実際の Status: %s", capturedMount.Status)
	}
}

// TestCreateVolumeMount_重複MountPathはErrDuplicateVolumeMountを返す は同一 MountPath が重複した場合に ErrDuplicateVolumeMount を返すことを確認する
func TestCreateVolumeMount_重複MountPathはErrDuplicateVolumeMountを返す_service(t *testing.T) {
	volumeMountRepo := &mockVolumeMountRepository{
		findByDeploymentIDAndMountPathFunc: func(ctx context.Context, deploymentID string, mountPath string) (*models.VolumeMount, error) {
			return &models.VolumeMount{ID: "existing-mount-id"}, nil // 既存のマウント設定を返す（重複）
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "test-user-id"}, nil // 所有者として返す
		},
	}

	svc := NewVolumeService(nil, &mockVolumeRepository{}, volumeMountRepo, deploymentRepo, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	req := CreateVolumeMountRequest{VolumeID: "volume-id-1", MountPath: "/data"} // リクエストを定義する
	_, err := svc.CreateVolumeMount(context.Background(), "test-user-id", "deployment-id-1", req) // 重複マウントパスで作成する
	if err != ErrDuplicateVolumeMount {                                                            // ErrDuplicateVolumeMount であることを確認する
		t.Errorf("期待するエラー: ErrDuplicateVolumeMount, 実際のエラー: %v", err)
	}
}

// TestCreateVolumeMount_他ユーザーはErrForbiddenを返す は他ユーザーが ErrForbidden を受け取ることを確認する
func TestCreateVolumeMount_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewVolumeService(nil, &mockVolumeRepository{}, &mockVolumeMountRepository{}, deploymentRepo, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	req := CreateVolumeMountRequest{VolumeID: "volume-id-1", MountPath: "/data"} // リクエストを定義する
	_, err := svc.CreateVolumeMount(context.Background(), "other-user-id", "deployment-id-1", req) // 他ユーザーとして作成する
	if err != ErrForbidden {                                                                         // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}

// TestDeleteVolumeMount_他ユーザーはErrForbiddenを返す は他ユーザーが ErrForbidden を受け取ることを確認する
func TestDeleteVolumeMount_他ユーザーはErrForbiddenを返す_service(t *testing.T) {
	volumeMountRepo := &mockVolumeMountRepository{
		findByIDFunc: func(ctx context.Context, mountID string) (*models.VolumeMount, error) {
			return &models.VolumeMount{ID: mountID, DeploymentID: "deployment-id-1"}, nil // マウント設定を返す
		},
	}
	deploymentRepo := &mockDeploymentRepository{
		findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
			return &models.Deployment{ID: deploymentID, ProjectID: "project-id-1"}, nil // deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "owner-user-id"}, nil // 別ユーザーが所有するプロジェクトを返す
		},
	}

	svc := NewVolumeService(nil, &mockVolumeRepository{}, volumeMountRepo, deploymentRepo, projectRepo, &noopUserQuotaRepository{}, nil, "") // サービスを生成する

	err := svc.DeleteVolumeMount(context.Background(), "other-user-id", "mount-id-1") // 他ユーザーとして削除する
	if err != ErrForbidden {                                                            // ErrForbidden であることを確認する
		t.Errorf("期待するエラー: ErrForbidden, 実際のエラー: %v", err)
	}
}
