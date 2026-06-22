package service

import (
	"app/k8s"
	"app/models"
	"app/repository"
	"context"
	"errors"

	"gorm.io/gorm"
	k8sclient "k8s.io/client-go/kubernetes"
)

// ErrDuplicateVolumeMount は同一DeploymentID・同一MountPathのマウントが既に存在する場合のエラー
var ErrDuplicateVolumeMount = errors.New("duplicate volume mount: this mount_path is already used in the deployment")

// VolumeService はボリューム CRUD のビジネスロジックを定義するインターフェース
type VolumeService interface {
	ListVolumes(ctx context.Context, userID string, projectID string) ([]*models.Volume, error)                                                        // ボリューム一覧を取得する
	CreateVolume(ctx context.Context, userID string, projectID string, req CreateVolumeRequest) (*models.Volume, error)                                // ボリュームを作成する
	DeleteVolume(ctx context.Context, userID string, volumeID string) error                                                                            // ボリュームを削除する
	ListVolumeMounts(ctx context.Context, userID string, deploymentID string) ([]*models.VolumeMount, error)                                           // ボリュームマウント一覧を取得する
	CreateVolumeMount(ctx context.Context, userID string, deploymentID string, req CreateVolumeMountRequest) (*models.VolumeMount, error)              // ボリュームマウントを作成する
	DeleteVolumeMount(ctx context.Context, userID string, mountID string) error                                                                        // ボリュームマウントを削除する
}

// CreateVolumeRequest は POST /projects/:id/volumes のリクエスト構造体
type CreateVolumeRequest struct {
	Name   string `json:"name"`    // ボリューム名
	SizeMB int    `json:"size_mb"` // ボリュームサイズ（MB）
}

// CreateVolumeMountRequest は POST /deployments/:id/volume-mounts のリクエスト構造体
type CreateVolumeMountRequest struct {
	VolumeID  string `json:"volume_id"`  // マウントするボリュームの ID
	MountPath string `json:"mount_path"` // マウントパス
}

// volumeServiceImpl は VolumeService の実装
type volumeServiceImpl struct {
	db               *gorm.DB                            // データベース接続（トランザクション開始に使用する）
	volumeRepo       repository.VolumeRepository         // volume リポジトリ
	volumeMountRepo  repository.VolumeMountRepository    // volume_mount リポジトリ
	deploymentRepo   repository.DeploymentRepository     // deployment リポジトリ（認可チェックに使用する）
	projectRepo      repository.ProjectRepository        // project リポジトリ（認可チェックに使用する）
	userQuotaRepo    repository.UserQuotaRepository      // user_quota リポジトリ（Quotaチェック用）
	k8sClient        k8sclient.Interface                 // k8s クライアント（PVC 作成に使用する）
	storageClassName string                              // PVC に使用する StorageClass 名
}

// NewVolumeService は VolumeService の実装を返す
func NewVolumeService(
	db *gorm.DB,
	volumeRepo repository.VolumeRepository,
	volumeMountRepo repository.VolumeMountRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	userQuotaRepo repository.UserQuotaRepository,
	k8sClient k8sclient.Interface,
	storageClassName string,
) VolumeService {
	return &volumeServiceImpl{
		db:               db,               // DB 接続を注入する
		volumeRepo:       volumeRepo,       // volume リポジトリを注入する
		volumeMountRepo:  volumeMountRepo,  // volume_mount リポジトリを注入する
		deploymentRepo:   deploymentRepo,   // deployment リポジトリを注入する
		projectRepo:      projectRepo,      // project リポジトリを注入する
		userQuotaRepo:    userQuotaRepo,    // user_quota リポジトリを注入する
		k8sClient:        k8sClient,        // k8s クライアントを注入する
		storageClassName: storageClassName, // StorageClass 名を注入する
	}
}

// checkProjectOwner は projectID に対応する project の UserID と userID を比較し、不一致の場合は ErrForbidden を返す
func (svc *volumeServiceImpl) checkProjectOwner(ctx context.Context, userID string, projectID string) error {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // ユーザー ID が一致しない場合は forbidden を返す
		return ErrForbidden // アクセス拒否エラーを返す
	}
	return nil // 認可チェック成功を返す
}

// ListVolumes は projectID に紐づく volume 一覧を返す
func (svc *volumeServiceImpl) ListVolumes(ctx context.Context, userID string, projectID string) ([]*models.Volume, error) {
	if err := svc.checkProjectOwner(ctx, userID, projectID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}
	return svc.volumeRepo.FindAllByProjectID(ctx, projectID) // リポジトリ経由で一覧を取得する
}

// CreateVolume は volume を作成し、k8s に PVC を作成する
func (svc *volumeServiceImpl) CreateVolume(ctx context.Context, userID string, projectID string, req CreateVolumeRequest) (*models.Volume, error) {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // namespace 解決のために project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // 所有者チェックを行う
		return nil, ErrForbidden
	}
	if err := CheckVolumeSizeQuota(ctx, svc.userQuotaRepo, userID, req.SizeMB); err != nil { // 1ボリューム最大サイズのQuotaチェックを行う
		return nil, err // Quota超過エラーを返す
	}
	if err := CheckVolumeCountQuota(ctx, svc.userQuotaRepo, userID); err != nil { // ボリューム数のQuotaチェックを行う
		return nil, err // Quota超過エラーを返す
	}
	if err := CheckTotalVolumeQuota(ctx, svc.userQuotaRepo, userID, req.SizeMB); err != nil { // ボリューム総容量のQuotaチェックを行う
		return nil, err // Quota超過エラーを返す
	}

	var createdVolume *models.Volume                                             // 結果格納用変数を宣言する
	err = svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {         // トランザクションを開始する
		volumeData := &models.Volume{
			ProjectID: projectID,              // プロジェクト ID を設定する
			Name:      req.Name,               // ボリューム名を設定する
			SizeMB:    req.SizeMB,             // サイズを設定する
			Status:    models.VolumeStatusPending, // ステータスを pending に設定する
		}
		if err := svc.volumeRepo.Create(ctx, tx, volumeData); err != nil { // tx を渡してリポジトリに委譲する
			return err // エラーでロールバックする
		}
		createdVolume = volumeData // 作成した volume を格納する

		// k8s に PVC を作成する（Volume 作成時に即時作成する）
		pvcName := volumeData.ID + "-pvc"                                                                      // PVC 名を VolumeID から生成する
		pvcManifest := k8s.BuildPVCManifest(projectData.Namespace, pvcName, req.SizeMB, svc.storageClassName) // PVC マニフェストを生成する
		if pvcErr := k8s.ApplyPVC(ctx, svc.k8sClient, pvcManifest); pvcErr != nil {                           // k8s に PVC を作成する
			return pvcErr // PVC 作成エラーでロールバックする
		}

		return nil // コミットする
	})
	return createdVolume, err // 結果を返す
}

// DeleteVolume は volumeID に対応する volume を削除する
func (svc *volumeServiceImpl) DeleteVolume(ctx context.Context, userID string, volumeID string) error {
	volumeData, err := svc.volumeRepo.FindByID(ctx, volumeID) // volume を取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	if err := svc.checkProjectOwner(ctx, userID, volumeData.ProjectID); err != nil { // 認可チェックを行う
		return err // エラーを返す
	}

	return svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		return svc.volumeRepo.Delete(ctx, tx, volumeData) // tx を渡してリポジトリに委譲する
	})
}

// checkDeploymentOwner は deploymentID に対応する deployment の ProjectID から project を取得し、UserID を比較する
func (svc *volumeServiceImpl) checkDeploymentOwner(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // ユーザー ID が一致しない場合は forbidden を返す
		return nil, ErrForbidden // アクセス拒否エラーを返す
	}
	return deploymentData, nil // deployment を返す
}

// ListVolumeMounts は deploymentID に紐づくボリュームマウント一覧を返す
func (svc *volumeServiceImpl) ListVolumeMounts(ctx context.Context, userID string, deploymentID string) ([]*models.VolumeMount, error) {
	if _, err := svc.checkDeploymentOwner(ctx, userID, deploymentID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}
	return svc.volumeMountRepo.FindAllByDeploymentID(ctx, deploymentID) // リポジトリ経由で一覧を取得する
}

// CreateVolumeMount はボリュームマウント設定を作成する
func (svc *volumeServiceImpl) CreateVolumeMount(ctx context.Context, userID string, deploymentID string, req CreateVolumeMountRequest) (*models.VolumeMount, error) {
	if _, err := svc.checkDeploymentOwner(ctx, userID, deploymentID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}

	// 同一 DeploymentID・同一 MountPath の重複チェックを行う
	_, dupErr := svc.volumeMountRepo.FindByDeploymentIDAndMountPath(ctx, deploymentID, req.MountPath) // 既存マウントを検索する
	if dupErr == nil {
		return nil, ErrDuplicateVolumeMount // 既に存在する場合は重複エラーを返す
	}
	if !errors.Is(dupErr, gorm.ErrRecordNotFound) {
		return nil, dupErr // 予期せぬエラーを返す
	}

	var createdMount *models.VolumeMount                                              // 結果格納用変数を宣言する
	txErr := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {            // トランザクションを開始する
		mountData := &models.VolumeMount{
			DeploymentID: deploymentID,                     // deployment ID を設定する
			VolumeID:     req.VolumeID,                     // volume ID を設定する
			MountPath:    req.MountPath,                    // マウントパスを設定する
			Status:       models.VolumeMountStatusPending,  // ステータスを pending に設定する
		}
		if err := svc.volumeMountRepo.Create(ctx, tx, mountData); err != nil { // tx を渡してリポジトリに委譲する
			return err // エラーでロールバックする
		}
		createdMount = mountData // 作成したマウント設定を格納する
		return nil               // コミットする
	})
	return createdMount, txErr // 結果を返す
}

// DeleteVolumeMount はボリュームマウント設定を削除する
// pending 状態（未 apply）の場合は即削除、mounted 状態の場合は deleting に変更して apply 待ちにする
func (svc *volumeServiceImpl) DeleteVolumeMount(ctx context.Context, userID string, mountID string) error {
	mountData, err := svc.volumeMountRepo.FindByID(ctx, mountID) // マウント設定を取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	if _, err := svc.checkDeploymentOwner(ctx, userID, mountData.DeploymentID); err != nil { // 認可チェックを行う
		return err // エラーを返す
	}

	return svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		if mountData.Status == models.VolumeMountStatusPending { // pending 状態は k8s に未反映のため即削除する
			return svc.volumeMountRepo.Delete(ctx, tx, mountData) // tx を渡してリポジトリに委譲する
		}
		return svc.volumeMountRepo.UpdateStatus(ctx, tx, mountData, models.VolumeMountStatusDeleting) // deleting に変更して apply 待ちにする
	})
}
