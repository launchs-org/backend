package service

import (
	"context"
	"errors"
	"fmt"

	"handler/k8s"
	"handler/models"
	"handler/repository"
)

// ErrImageInUse はイメージが Deployment から参照されているため削除できない場合のエラー
var ErrImageInUse = errors.New("image is currently referenced by a deployment")

// ImageService はイメージ管理のビジネスロジックを定義するインターフェース
type ImageService interface {
	ListImagesByProject(ctx context.Context, userID string, projectID string) ([]models.Image, error) // イメージ一覧を取得する（project 単位）
	GetImage(ctx context.Context, userID string, imageID string) (*models.Image, error)                // イメージ情報を取得する
	DeleteImage(ctx context.Context, userID string, projectID string, imageID string) error             // イメージを削除する（参照チェック・Harbor削除を含む）
}

// imageServiceImpl は ImageService の実装
type imageServiceImpl struct {
	imageRepo            repository.ImageRepository             // image リポジトリ
	deploymentRepo       repository.DeploymentRepository       // deployment リポジトリ（参照チェック用）
	projectRepo          repository.ProjectRepository          // project リポジトリ（所有権チェック用）
	harborCredentialRepo repository.HarborCredentialRepository // harbor credential リポジトリ
	buildRepo            repository.DeploymentBuildRepository  // build リポジトリ（Harbor タグ解決用）
	harborClient         *k8s.HarborClient                     // Harbor API クライアント（イメージ削除用）
}

// NewImageService は ImageService の実装を返す
func NewImageService(
	imageRepo repository.ImageRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	buildRepo repository.DeploymentBuildRepository,
	harborClient *k8s.HarborClient,
) ImageService {
	return &imageServiceImpl{
		imageRepo:            imageRepo,            // image リポジトリを注入する
		deploymentRepo:       deploymentRepo,       // deployment リポジトリを注入する
		projectRepo:          projectRepo,          // project リポジトリを注入する
		harborCredentialRepo: harborCredentialRepo, // harbor credential リポジトリを注入する
		buildRepo:            buildRepo,            // build リポジトリを注入する
		harborClient:         harborClient,         // Harbor クライアントを注入する
	}
}

// ListImagesByProject は projectID に紐づくイメージ一覧を返す
func (svc *imageServiceImpl) ListImagesByProject(ctx context.Context, userID string, projectID string) ([]models.Image, error) {
	// 1. 所有権チェック
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	// 2. イメージ一覧を取得して返す
	imageList, err := svc.imageRepo.FindAllByProjectID(ctx, projectID) // project 単位でイメージ一覧を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return imageList, nil // イメージ一覧を返す
}

// GetImage は指定したイメージレコードを返す
func (svc *imageServiceImpl) GetImage(ctx context.Context, userID string, imageID string) (*models.Image, error) {
	imageData, err := svc.imageRepo.FindByID(ctx, imageID) // イメージレコードを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, imageData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	return imageData, nil // イメージレコードを返す
}

// DeleteImage はイメージを削除する（参照チェック・Harborタグ単位削除を含む）
func (svc *imageServiceImpl) DeleteImage(ctx context.Context, userID string, projectID string, imageID string) error {
	// 1. 所有権チェック
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}

	// 2. イメージレコードを取得する
	imageData, err := svc.imageRepo.FindByID(ctx, imageID) // イメージを取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if imageData.ProjectID != projectID { // 別プロジェクトのイメージへのアクセスを禁止する
		return ErrForbidden
	}

	// 3. 参照チェック: projectID 配下の全 Deployment を取得し、image_id / pending_image_id が一致するものがないか確認する
	deploymentList, err := svc.deploymentRepo.FindAllByProjectID(ctx, projectID) // deployment 一覧を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	for _, deploymentData := range deploymentList { // 各 deployment の参照を確認する
		if deploymentData.ImageID != nil && *deploymentData.ImageID == imageID { // 現在参照中の場合
			return ErrImageInUse // 使用中エラーを返す
		}
		if deploymentData.PendingImageID != nil && *deploymentData.PendingImageID == imageID { // pending で参照中の場合
			return ErrImageInUse // 使用中エラーを返す
		}
	}

	// 4. Harbor削除の分岐（BuildID の有無で分岐する）
	if imageData.BuildID != nil { // railpack/dockerfileビルド経由（Harbor上に実体が存在する）
		buildData, buildErr := svc.buildRepo.FindByID(ctx, *imageData.BuildID) // ビルドレコードを取得する
		if buildErr != nil {
			return fmt.Errorf("ビルドレコードの取得に失敗しました: %w", buildErr) // 取得エラーを返す
		}
		credentialData, credErr := svc.harborCredentialRepo.FindByProjectIDNoTx(ctx, projectID) // Harbor 認証情報を取得する
		if credErr == nil {                                                                       // 認証情報が取得できた場合のみ削除を試みる
			repositoryName := buildData.ID // デフォルトはビルド ID をリポジトリ名として使う
			if buildData.DeploymentID != nil {
				repositoryName = *buildData.DeploymentID // Deployment ID が存在する場合はそれを使う
			}
			credential := k8s.HarborRobotCredential{
				Name:   credentialData.RobotName,   // robot アカウント名を設定する
				Secret: credentialData.RobotSecret, // シークレットを設定する
			}
			if harborErr := svc.harborClient.DeleteHarborImage(ctx, projectID, repositoryName, buildData.ID, credential); harborErr != nil { // Harbor イメージをタグ単位で削除する
				return fmt.Errorf("Harbor イメージの削除に失敗しました: %w", harborErr) // 削除エラーを返す
			}
		}
	}
	// BuildID が nil の場合（DockerHub等、外部URL直接指定）は Harbor API を一切呼び出さない

	// 5. DB レコードを削除する
	if err := svc.imageRepo.Delete(ctx, imageData); err != nil { // イメージレコードを削除する
		return err // 削除エラーを返す
	}
	return nil // 正常終了
}
