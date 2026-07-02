package service

import (
	"context"
	"fmt"

	"handler/k8s"
	"handler/models"
	"handler/repository"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"
	"k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
)

// ProjectService は Project の CRUD ビジネスロジックを定義するインターフェース
type ProjectService interface {
	CreateProject(ctx context.Context, userID string, req CreateProjectRequest) (*models.Project, error)           // project を作成する
	ListProjects(ctx context.Context, userID string) ([]*models.Project, error)                                    // project 一覧を取得する
	GetProject(ctx context.Context, projectID string) (*models.Project, error)                                     // project を取得する
	UpdateProject(ctx context.Context, projectID string, req UpdateProjectRequest) (*models.Project, error)        // project を更新する
	DeleteProject(ctx context.Context, projectID string) error                                                     // project を削除する
	GetProjectQuota(ctx context.Context, userID string, projectID string) (*k8s.HarborProjectQuota, error)        // project のストレージクォータを取得する
}

// CreateProjectRequest は POST /projects のリクエスト構造体
type CreateProjectRequest struct {
	Name string `json:"name"` // プロジェクト名（k8s namespace 名にもなる）
}

// UpdateProjectRequest は PUT /projects/:id のリクエスト構造体
type UpdateProjectRequest struct {
	Name *string `json:"name"` // nil の場合は更新しない
}

// CreateProjectWorkflowInput は CreateProject Temporal Workflow への入力
type CreateProjectWorkflowInput struct {
	ProjectID string // 作成対象プロジェクトの ID（あらかじめ DB に provisioning 状態で作成済み）
}

// DeleteProjectWorkflowInput は DeleteProject Temporal Workflow への入力
type DeleteProjectWorkflowInput struct {
	ProjectID string // 削除対象プロジェクトの ID
}

// projectServiceImpl は ProjectService の実装
type projectServiceImpl struct {
	db                   *gorm.DB                              // データベース接続（トランザクション開始に使用する）
	projectRepo          repository.ProjectRepository          // project リポジトリ
	harborCredentialRepo repository.HarborCredentialRepository // harbor credential リポジトリ
	deploymentRepo       repository.DeploymentRepository       // deployment リポジトリ
	buildRepo            repository.DeploymentBuildRepository  // deployment_build リポジトリ（Project 削除時のビルド履歴削除に使用）
	ingressRouteRepo     repository.IngressRouteRepository     // ingress_route リポジトリ
	userQuotaRepo        repository.UserQuotaRepository        // user_quota リポジトリ（Quotaチェック用）
	k8sClient            k8sclient.Interface                   // k8s クライアント（GetProjectQuota 等に使用）
	dynamicClient        dynamic.Interface                     // dynamic クライアント（IngressRoute 削除用）
	harborClient         *k8s.HarborClient                     // Harbor API クライアント（管理用 robot）
	harborStorageLimit   int64                                 // Harbor プロジェクトのストレージ上限（バイト）
	temporalClient       temporalclient.Client                 // Temporal クライアント（Workflow 起動用）
}

// NewProjectService は ProjectService の実装を返す
func NewProjectService(
	db *gorm.DB,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	deploymentRepo repository.DeploymentRepository,
	buildRepo repository.DeploymentBuildRepository,
	ingressRouteRepo repository.IngressRouteRepository,
	userQuotaRepo repository.UserQuotaRepository,
	k8sClient k8sclient.Interface,
	dynamicClient dynamic.Interface,
	harborClient *k8s.HarborClient,
	harborStorageLimit int64,
	temporalClient temporalclient.Client,
) ProjectService {
	return &projectServiceImpl{
		db:                   db,                   // DB 接続を注入する
		projectRepo:          projectRepo,          // project リポジトリを注入する
		harborCredentialRepo: harborCredentialRepo, // harbor credential リポジトリを注入する
		deploymentRepo:       deploymentRepo,       // deployment リポジトリを注入する
		buildRepo:            buildRepo,            // deployment_build リポジトリを注入する
		ingressRouteRepo:     ingressRouteRepo,     // ingress_route リポジトリを注入する
		userQuotaRepo:        userQuotaRepo,        // user_quota リポジトリを注入する
		k8sClient:            k8sClient,            // k8s クライアントを注入する
		dynamicClient:        dynamicClient,        // dynamic クライアントを注入する
		harborClient:         harborClient,         // Harbor クライアントを注入する
		harborStorageLimit:   harborStorageLimit,   // Harbor ストレージ上限を注入する
		temporalClient:       temporalClient,       // Temporal クライアントを注入する
	}
}

// CreateProject は Project レコードを DB に provisioning 状態で作成し、
// Harbor・Namespace 作成は Temporal CreateProjectWorkflow に委譲する（非同期）
func (svc *projectServiceImpl) CreateProject(ctx context.Context, userID string, req CreateProjectRequest) (*models.Project, error) {
	if err := CheckProjectQuota(ctx, svc.userQuotaRepo, userID); err != nil { // プロジェクト数のQuotaチェックを行う
		return nil, err // Quota超過エラーを返す
	}

	var createdProject *models.Project // 作成した project を格納する変数を定義する

	err := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		projectID := uuid.New().String() // プロジェクト ID を事前生成する
		projectData := &models.Project{
			ID:        projectID,                       // UUID を明示セットする
			UserID:    userID,                          // ユーザーIDを設定する
			Name:      req.Name,                        // プロジェクト名を設定する
			Namespace: "project-" + projectID,          // namespace にプロジェクト ID を使う
			Status:    models.ProjectStatusProvisioning, // 初期ステータスを設定する（Workflow 完了後 active になる）
		}
		if err := svc.projectRepo.Create(ctx, tx, projectData); err != nil { // DB に project レコードを作成する
			return fmt.Errorf("project レコードの作成に失敗しました: %w", err) // 作成エラーを返す
		}

		workflowOptions := temporalclient.StartWorkflowOptions{
			ID:        "create-project-" + projectID, // WorkflowID を設定して冪等性を保証する
			TaskQueue: "controller-queue",             // controller Worker のタスクキューを指定する
		}
		workflowInput := CreateProjectWorkflowInput{ProjectID: projectID} // Workflow 入力を構築する
		_, startErr := svc.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "CreateProjectWorkflow", workflowInput) // Workflow を起動する
		if startErr != nil {
			return fmt.Errorf("project 作成 workflow の起動に失敗しました: %w", startErr) // 起動エラーを返す
		}

		createdProject = projectData // 外側の変数に結果を格納する
		return nil                   // トランザクションをコミットする
	})
	if err != nil {
		return nil, err // トランザクションエラーを返す
	}
	return createdProject, nil // 作成した project を返す
}

// ListProjects は userID に紐づく project 一覧を返す
func (svc *projectServiceImpl) ListProjects(ctx context.Context, userID string) ([]*models.Project, error) {
	return svc.projectRepo.FindAllByUserID(ctx, userID) // リポジトリ経由で取得する
}

// GetProject は projectID に対応する project を返す
func (svc *projectServiceImpl) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	return svc.projectRepo.FindByID(ctx, svc.db, projectID) // リポジトリ経由で取得する
}

// UpdateProject は projectID の project 名を部分更新する
func (svc *projectServiceImpl) UpdateProject(ctx context.Context, projectID string, req UpdateProjectRequest) (*models.Project, error) {
	projectData, err := svc.projectRepo.FindByID(ctx, svc.db, projectID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	if req.Name != nil {
		projectData.Name = *req.Name // 名前を更新する
	}

	if err := svc.projectRepo.Save(ctx, projectData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return projectData, nil // 更新後の project を返す
}

// DeleteProject は project を deleting 状態にし、Temporal DeleteProjectWorkflow に委譲して
// Harbor・Namespace・DB レコード削除を非同期に実行する
func (svc *projectServiceImpl) DeleteProject(ctx context.Context, projectID string) error {
	err := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		projectData, err := svc.projectRepo.FindByID(ctx, tx, projectID) // project を取得する
		if err != nil {
			return fmt.Errorf("project の取得に失敗しました: %w", err) // 取得エラーを返す
		}

		if err := svc.projectRepo.UpdateStatus(ctx, tx, projectData, models.ProjectStatusDeleting); err != nil { // project status を deleting に更新する
			return fmt.Errorf("project ステータスの更新に失敗しました: %w", err) // 更新エラーを返す
		}

		workflowOptions := temporalclient.StartWorkflowOptions{
			ID:        "delete-project-" + projectID, // WorkflowID を設定して冪等性を保証する
			TaskQueue: "controller-queue",             // controller Worker のタスクキューを指定する
		}
		workflowInput := DeleteProjectWorkflowInput{ProjectID: projectID} // Workflow 入力を構築する
		_, startErr := svc.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "DeleteProjectWorkflow", workflowInput) // Workflow を起動する
		if startErr != nil {
			return fmt.Errorf("project 削除 workflow の起動に失敗しました: %w", startErr) // 起動エラーを返す
		}

		return nil // トランザクションをコミットする
	})
	return err // エラーを返す
}

// GetProjectQuota は projectID に対応する Harbor プロジェクトのストレージクォータを返す
func (svc *projectServiceImpl) GetProjectQuota(ctx context.Context, userID string, projectID string) (*k8s.HarborProjectQuota, error) {
	projectData, err := svc.projectRepo.FindByID(ctx, svc.db, projectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // 所有権チェックを行う
		return nil, ErrForbidden // 権限なしエラーを返す
	}

	credentialData, err := svc.harborCredentialRepo.FindByProjectID(ctx, svc.db, projectID) // Harbor 認証情報を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	quotaData, err := svc.harborClient.GetProjectQuota(ctx, projectID, k8s.HarborRobotCredential{ // Harbor API でクォータを取得する
		Name:   credentialData.RobotName,   // robot アカウント名を設定する
		Secret: credentialData.RobotSecret, // シークレットを設定する
	})
	if err != nil {
		return nil, err // クォータ取得エラーを返す
	}
	return quotaData, nil // クォータ情報を返す
}
