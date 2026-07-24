package service

import (
	"handler/models"
	"handler/repository"
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrDuplicateMount は同一DeploymentID・同一EnvVarIDのマウントが既に存在する場合のエラー
var ErrDuplicateMount = errors.New("duplicate mount: this env_var is already mounted to the deployment")

// EnvVarMountService は環境変数マウント CRUD のビジネスロジックを定義するインターフェース
type EnvVarMountService interface {
	ListEnvVarMounts(ctx context.Context, userID string, deploymentID string) ([]*models.EnvVarMount, error)                                  // マウント設定一覧を取得する
	CreateEnvVarMount(ctx context.Context, userID string, deploymentID string, req CreateEnvVarMountRequest) (*models.EnvVarMount, error)     // マウント設定を作成する
	DeleteEnvVarMount(ctx context.Context, userID string, mountID string) error                                                               // マウント設定を削除する
}

// CreateEnvVarMountRequest は POST /deployments/:id/env-var-mounts のリクエスト構造体
type CreateEnvVarMountRequest struct {
	EnvVarID    string `json:"env_var_id"`   // マウントする環境変数の ID
	OverrideKey string `json:"override_key"` // k8s 側の環境変数名（空の場合は元のキーをそのまま使用する）
}

// envVarMountServiceImpl は EnvVarMountService の実装
type envVarMountServiceImpl struct {
	db               *gorm.DB                              // データベース接続（トランザクション開始に使用する）
	mountRepo        repository.EnvVarMountRepository      // env_var_mount リポジトリ
	deploymentRepo   repository.DeploymentRepository       // deployment リポジトリ（認可チェックに使用する）
	projectRepo      repository.ProjectRepository          // project リポジトリ（認可チェックに使用する）
}

// NewEnvVarMountService は EnvVarMountService の実装を返す
func NewEnvVarMountService(
	db *gorm.DB,
	mountRepo repository.EnvVarMountRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
) EnvVarMountService {
	return &envVarMountServiceImpl{
		db:             db,             // DB 接続を注入する
		mountRepo:      mountRepo,      // mount リポジトリを注入する
		deploymentRepo: deploymentRepo, // deployment リポジトリを注入する
		projectRepo:    projectRepo,    // project リポジトリを注入する
	}
}

// checkDeploymentOwner は deploymentID に対応する deployment の ProjectID から project を取得し、UserID を比較する
func (svc *envVarMountServiceImpl) checkDeploymentOwner(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
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

// ListEnvVarMounts は deploymentID に紐づくマウント設定一覧を返す
func (svc *envVarMountServiceImpl) ListEnvVarMounts(ctx context.Context, userID string, deploymentID string) ([]*models.EnvVarMount, error) {
	if _, err := svc.checkDeploymentOwner(ctx, userID, deploymentID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}
	return svc.mountRepo.FindAllByDeploymentID(ctx, deploymentID) // リポジトリ経由で一覧を取得する
}

// CreateEnvVarMount はマウント設定を作成する
func (svc *envVarMountServiceImpl) CreateEnvVarMount(ctx context.Context, userID string, deploymentID string, req CreateEnvVarMountRequest) (*models.EnvVarMount, error) {
	if _, err := svc.checkDeploymentOwner(ctx, userID, deploymentID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}

	// 同一DeploymentID・同一EnvVarIDの重複チェックを行う
	_, err := svc.mountRepo.FindByDeploymentIDAndEnvVarID(ctx, deploymentID, req.EnvVarID) // 既存マウントを検索する
	if err == nil {
		return nil, ErrDuplicateMount // 既に存在する場合は重複エラーを返す
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err // 予期せぬエラーを返す
	}

	var createdMount *models.EnvVarMount                                             // 結果格納用変数を宣言する
	txErr := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {           // トランザクションを開始する
		mountData := &models.EnvVarMount{
			DeploymentID: deploymentID,   // deployment ID を設定する
			EnvVarID:     req.EnvVarID,   // env_var ID を設定する
			OverrideKey:  req.OverrideKey, // オーバーライドキーを設定する
			Status:       models.EnvVarMountStatusPending, // ステータスを pending に設定する
		}
		if err := svc.mountRepo.Create(ctx, tx, mountData); err != nil { // tx を渡してリポジトリに委譲する
			return err // エラーでロールバックする
		}
		createdMount = mountData // 作成したマウント設定を格納する
		return nil               // コミットする
	})
	return createdMount, txErr // 結果を返す
}

// DeleteEnvVarMount はマウント設定を削除する
// pending 状態（未 apply）の場合は即削除、applied 状態の場合は deleting に変更して apply 待ちにする
func (svc *envVarMountServiceImpl) DeleteEnvVarMount(ctx context.Context, userID string, mountID string) error {
	mountData, err := svc.mountRepo.FindByID(ctx, mountID) // マウント設定を取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	if _, err := svc.checkDeploymentOwner(ctx, userID, mountData.DeploymentID); err != nil { // 認可チェックを行う
		return err // エラーを返す
	}

	return svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		if mountData.Status == models.EnvVarMountStatusPending { // pending 状態は k8s に未反映のため即削除する
			return svc.mountRepo.Delete(ctx, tx, mountData) // tx を渡してリポジトリに委譲する
		}
		return svc.mountRepo.UpdateStatus(ctx, tx, mountData, models.EnvVarMountStatusDeleting) // deleting に変更して apply 待ちにする
	})
}
