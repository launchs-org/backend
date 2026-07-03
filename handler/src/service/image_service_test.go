package service

import (
	"context"
	"errors"
	"testing"

	"handler/models"
)

// TestImageService_ListImagesByProject_正常系 は所有者本人が一覧を取得できることを確認する
func TestImageService_ListImagesByProject_正常系(t *testing.T) {
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageRepo := &mockImageRepository{}
	imageRepo.findAllByProjectIDFunc = func(ctx context.Context, projectID string) ([]models.Image, error) {
		return []models.Image{{ID: "image-1", ProjectID: projectID}}, nil // イメージ一覧を返す
	}

	imageSvc := NewImageService(imageRepo, &mockDeploymentRepository{}, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	imageList, err := imageSvc.ListImagesByProject(context.Background(), "user-1", "project-1") // イメージ一覧を取得する
	if err != nil {
		t.Fatalf("ListImagesByProject() がエラーを返しました: %v", err)
	}
	if len(imageList) != 1 { // 件数を確認する
		t.Errorf("期待するイメージ件数 1、実際の件数 %d", len(imageList))
	}
}

// TestImageService_ListImagesByProject_所有権エラー は所有者以外のアクセスが禁止されることを確認する
func TestImageService_ListImagesByProject_所有権エラー(t *testing.T) {
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(&mockImageRepository{}, &mockDeploymentRepository{}, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	if _, err := imageSvc.ListImagesByProject(context.Background(), "user-2", "project-1"); !errors.Is(err, ErrForbidden) { // 別ユーザーでアクセスする
		t.Errorf("期待するエラー ErrForbidden、実際のエラー %v", err)
	}
}

// TestImageService_GetImage_正常系 は所有者本人がイメージを取得できることを確認する
func TestImageService_GetImage_正常系(t *testing.T) {
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "project-1"}, nil // イメージレコードを返す
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(imageRepo, &mockDeploymentRepository{}, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	imageData, err := imageSvc.GetImage(context.Background(), "user-1", "image-1") // イメージを取得する
	if err != nil {
		t.Fatalf("GetImage() がエラーを返しました: %v", err)
	}
	if imageData.ID != "image-1" { // 取得結果を確認する
		t.Errorf("期待するイメージID image-1、実際のイメージID %s", imageData.ID)
	}
}

// TestImageService_GetImage_所有権エラー は所有者以外のアクセスが禁止されることを確認する
func TestImageService_GetImage_所有権エラー(t *testing.T) {
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "project-1"}, nil // イメージレコードを返す
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(imageRepo, &mockDeploymentRepository{}, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	if _, err := imageSvc.GetImage(context.Background(), "user-2", "image-1"); !errors.Is(err, ErrForbidden) { // 別ユーザーでアクセスする
		t.Errorf("期待するエラー ErrForbidden、実際のエラー %v", err)
	}
}

// TestImageService_DeleteImage_使用中エラー は Deployment から参照中のイメージが削除できないことを確認する
func TestImageService_DeleteImage_使用中エラー(t *testing.T) {
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "project-1"}, nil // イメージレコードを返す
	}
	imageIDValue := "image-1"
	deploymentRepo := &mockDeploymentRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]models.Deployment, error) {
			return []models.Deployment{{ID: "deployment-1", ImageID: &imageIDValue}}, nil // 対象イメージを参照中の deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(imageRepo, deploymentRepo, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	err := imageSvc.DeleteImage(context.Background(), "user-1", "project-1", "image-1") // 使用中のイメージを削除しようとする
	if !errors.Is(err, ErrImageInUse) { // 使用中エラーを確認する
		t.Errorf("期待するエラー ErrImageInUse、実際のエラー %v", err)
	}
}

// TestImageService_DeleteImage_PendingImageIDでも使用中エラーになる は pending_image_id 参照でも削除できないことを確認する
func TestImageService_DeleteImage_PendingImageIDでも使用中エラーになる(t *testing.T) {
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "project-1"}, nil // イメージレコードを返す
	}
	imageIDValue := "image-1"
	deploymentRepo := &mockDeploymentRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]models.Deployment, error) {
			return []models.Deployment{{ID: "deployment-1", PendingImageID: &imageIDValue}}, nil // pending で対象イメージを参照中の deployment を返す
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(imageRepo, deploymentRepo, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	err := imageSvc.DeleteImage(context.Background(), "user-1", "project-1", "image-1") // 使用中のイメージを削除しようとする
	if !errors.Is(err, ErrImageInUse) { // 使用中エラーを確認する
		t.Errorf("期待するエラー ErrImageInUse、実際のエラー %v", err)
	}
}

// TestImageService_DeleteImage_外部URL直接指定はHarborを呼ばずに削除される は BuildID が nil の場合に Harbor API を呼ばないことを確認する
func TestImageService_DeleteImage_外部URL直接指定はHarborを呼ばずに削除される(t *testing.T) {
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "project-1", BuildID: nil}, nil // BuildID が nil のイメージを返す（外部URL直接指定）
	}
	deleteCalled := false
	imageRepo.deleteFunc = func(ctx context.Context, image *models.Image) error {
		deleteCalled = true // DB 削除が呼ばれたことを記録する
		return nil
	}
	deploymentRepo := &mockDeploymentRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]models.Deployment, error) {
			return []models.Deployment{}, nil // 参照している deployment は存在しない
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	// harborClient を nil のまま渡す。BuildID が nil のため呼び出されないはず（呼び出されると nil pointer panic になる）
	imageSvc := NewImageService(imageRepo, deploymentRepo, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	if err := imageSvc.DeleteImage(context.Background(), "user-1", "project-1", "image-1"); err != nil { // イメージを削除する
		t.Fatalf("DeleteImage() がエラーを返しました: %v", err)
	}
	if !deleteCalled { // DB 削除が呼ばれたことを確認する
		t.Error("イメージレコードの削除が呼び出されませんでした")
	}
}

// TestImageService_DeleteImage_Harbor認証情報が取得できない場合はHarbor削除をスキップする は認証情報未設定時に安全にスキップされることを確認する
func TestImageService_DeleteImage_Harbor認証情報が取得できない場合はHarbor削除をスキップする(t *testing.T) {
	buildIDValue := "build-1"
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "project-1", BuildID: &buildIDValue}, nil // BuildID が設定されたイメージを返す（railpack経由）
	}
	deleteCalled := false
	imageRepo.deleteFunc = func(ctx context.Context, image *models.Image) error {
		deleteCalled = true // DB 削除が呼ばれたことを記録する
		return nil
	}
	deploymentRepo := &mockDeploymentRepository{
		findAllByProjectIDFunc: func(ctx context.Context, projectID string) ([]models.Deployment, error) {
			return []models.Deployment{}, nil // 参照している deployment は存在しない
		},
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	buildRepo := &mockDeploymentBuildRepository{}
	buildRepo.findByIDFunc = func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
		return &models.DeploymentBuild{ID: buildID, ProjectID: "project-1"}, nil // ビルドレコードを返す
	}
	// harborCredentialRepo がエラーを返すため、Harbor 削除処理はスキップされ harborClient（nil）は呼ばれない
	harborCredRepo := &mockHarborCredentialRepository{
		findByProjectIDNoTxFunc: func(ctx context.Context, projectID string) (*models.HarborCredential, error) {
			return nil, errors.New("harbor credential が見つかりません") // 認証情報未設定を模す
		},
	}
	imageSvc := NewImageService(imageRepo, deploymentRepo, projectRepo, harborCredRepo, buildRepo, nil) // サービスを生成する（harborClient は nil）

	if err := imageSvc.DeleteImage(context.Background(), "user-1", "project-1", "image-1"); err != nil { // イメージを削除する
		t.Fatalf("DeleteImage() がエラーを返しました: %v", err)
	}
	if !deleteCalled { // DB 削除が呼ばれたことを確認する
		t.Error("イメージレコードの削除が呼び出されませんでした")
	}
}

// TestImageService_DeleteImage_所有権エラー は所有者以外の削除が禁止されることを確認する
func TestImageService_DeleteImage_所有権エラー(t *testing.T) {
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(&mockImageRepository{}, &mockDeploymentRepository{}, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	err := imageSvc.DeleteImage(context.Background(), "user-2", "project-1", "image-1") // 別ユーザーで削除しようとする
	if !errors.Is(err, ErrForbidden) { // 所有権エラーを確認する
		t.Errorf("期待するエラー ErrForbidden、実際のエラー %v", err)
	}
}

// TestImageService_DeleteImage_別プロジェクトのイメージへのアクセス は他プロジェクトのイメージ削除が禁止されることを確認する
func TestImageService_DeleteImage_別プロジェクトのイメージへのアクセス(t *testing.T) {
	imageRepo := &mockImageRepository{}
	imageRepo.findByIDFunc = func(ctx context.Context, imageID string) (*models.Image, error) {
		return &models.Image{ID: imageID, ProjectID: "other-project"}, nil // 別プロジェクトのイメージを返す
	}
	projectRepo := &mockProjectRepository{
		findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return &models.Project{ID: projectID, UserID: "user-1"}, nil // 所有者を user-1 に設定する
		},
	}
	imageSvc := NewImageService(imageRepo, &mockDeploymentRepository{}, projectRepo, &mockHarborCredentialRepository{}, &mockDeploymentBuildRepository{}, nil) // サービスを生成する

	err := imageSvc.DeleteImage(context.Background(), "user-1", "project-1", "image-1") // 別プロジェクトのイメージを削除しようとする
	if !errors.Is(err, ErrForbidden) { // 所有権エラーを確認する
		t.Errorf("期待するエラー ErrForbidden、実際のエラー %v", err)
	}
}
