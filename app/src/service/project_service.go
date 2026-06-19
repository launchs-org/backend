package service

import (
	"app/k8s"
	"app/logger"
	"app/models"
	"app/repository"
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"

	"gorm.io/gorm"
)

// ProjectService は Project の CRUD ビジネスロジックを定義するインターフェース
type ProjectService interface {
	CreateProject(ctx context.Context, userID string, req CreateProjectRequest) (*models.Project, error)        // project を作成する
	ListProjects(ctx context.Context, userID string) ([]*models.Project, error)                                 // project 一覧を取得する
	GetProject(ctx context.Context, projectID string) (*models.Project, error)                                  // project を取得する
	UpdateProject(ctx context.Context, projectID string, req UpdateProjectRequest) (*models.Project, error)     // project を更新する
	DeleteProject(ctx context.Context, projectID string) error                                                  // project を削除する
}

// CreateProjectRequest は POST /projects のリクエスト構造体
type CreateProjectRequest struct {
	Name string `json:"name"` // プロジェクト名（k8s namespace 名にもなる）
}

// UpdateProjectRequest は PUT /projects/:id のリクエスト構造体
type UpdateProjectRequest struct {
	Name *string `json:"name"` // nil の場合は更新しない
}

// projectServiceImpl は ProjectService の実装
type projectServiceImpl struct {
	db                    *gorm.DB                                 // データベース接続（トランザクション開始に使用する）
	projectRepo           repository.ProjectRepository             // project リポジトリ
	harborCredentialRepo  repository.HarborCredentialRepository    // harbor credential リポジトリ
	deploymentRepo        repository.DeploymentRepository          // deployment リポジトリ
	ingressRouteRepo      repository.IngressRouteRepository        // ingress_route リポジトリ（プロジェクト削除時に使用）
	ingressRouteRouteRepo repository.IngressRouteRouteRepository   // ingress_route_route リポジトリ（プロジェクト削除時に使用）
	userQuotaRepo         repository.UserQuotaRepository           // user_quota リポジトリ（Quotaチェック用）
	k8sClient             k8sclient.Interface                      // k8s クライアント
	dynamicClient         dynamic.Interface                        // dynamic クライアント（IngressRoute 削除用）
	harborClient          *k8s.HarborClient                        // Harbor API クライアント（管理用 robot）
}

// NewProjectService は ProjectService の実装を返す
func NewProjectService(
	db *gorm.DB,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	deploymentRepo repository.DeploymentRepository,
	ingressRouteRepo repository.IngressRouteRepository,
	ingressRouteRouteRepo repository.IngressRouteRouteRepository,
	userQuotaRepo repository.UserQuotaRepository,
	k8sClient k8sclient.Interface,
	dynamicClient dynamic.Interface,
	harborClient *k8s.HarborClient,
) ProjectService {
	return &projectServiceImpl{
		db:                    db,                    // DB 接続を注入する
		projectRepo:           projectRepo,           // project リポジトリを注入する
		harborCredentialRepo:  harborCredentialRepo,  // harbor credential リポジトリを注入する
		deploymentRepo:        deploymentRepo,         // deployment リポジトリを注入する
		ingressRouteRepo:      ingressRouteRepo,       // ingress_route リポジトリを注入する
		ingressRouteRouteRepo: ingressRouteRouteRepo,  // ingress_route_route リポジトリを注入する
		userQuotaRepo:         userQuotaRepo,          // user_quota リポジトリを注入する
		k8sClient:             k8sClient,              // k8s クライアントを注入する
		dynamicClient:         dynamicClient,          // dynamic クライアントを注入する
		harborClient:          harborClient,           // Harbor クライアントを注入する
	}
}

// CreateProject は Project を作成し、Harbor project・robot account と k8s namespace を同時に作成する
// 外部リソース作成失敗時は補償処理で作成済みリソースを削除する
func (svc *projectServiceImpl) CreateProject(ctx context.Context, userID string, req CreateProjectRequest) (*models.Project, error) {
	if err := CheckProjectQuota(ctx, svc.userQuotaRepo, userID); err != nil { // プロジェクト数のQuotaチェックを行う
		return nil, err // Quota超過エラーを返す
	}

	var createdProject *models.Project

	// DB トランザクションを開始する
	err := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Project レコードを作成する
		projectID := uuid.New().String()                          // プロジェクト ID を事前生成する
		projectData := &models.Project{
			ID:        projectID,                                     // UUID を明示セットする
			UserID:    userID,                                        // ユーザーIDを設定する
			Name:      req.Name,                                      // プロジェクト名を設定する
			Namespace: "project-" + projectID,                        // namespace にプロジェクト ID を使う
			Status:    models.ProjectStatusProvisioning,              // 初期ステータスを設定する
		}
		if err := svc.projectRepo.Create(ctx, tx, projectData); err != nil {
			return fmt.Errorf("project レコードの作成に失敗しました: %w", err)
		}

		// Harbor project を作成する（失敗時は DB ロールバック）
		if err := svc.harborClient.CreateHarborProject(ctx, projectID); err != nil { // Harbor プロジェクト名にプロジェクト ID を使う
			return fmt.Errorf("harbor project の作成に失敗しました: %w", err)
		}

		// Harbor robot account を作成する（失敗時は管理用 robot で Harbor project を補償削除して DB ロールバック）
		robotCredential, err := svc.harborClient.CreateHarborRobotAccount(ctx, projectID) // Harbor プロジェクト名にプロジェクト ID を使う
		if err != nil {
			// robot account 未作成なので管理用 robot で補償削除する
			_ = svc.harborClient.DeleteHarborProject(ctx, projectID, svc.harborClient.AdminCredential()) // ベストエフォートで補償削除する
			return fmt.Errorf("harbor robot account の作成に失敗しました: %w", err)
		}

		// HarborCredential レコードを DB に保存する（失敗時は project 専用 robot で Harbor project を補償削除して DB ロールバック）
		credentialData := &models.HarborCredential{
			ProjectID:      projectData.ID,              // プロジェクト ID を設定する
			RobotName:      robotCredential.Name,        // robot アカウント名を設定する
			RobotSecret:    robotCredential.Secret,      // シークレットを設定する
			HarborEndpoint: svc.harborClient.Endpoint(), // エンドポイントを設定する
		}
		if err := svc.harborCredentialRepo.Create(ctx, tx, credentialData); err != nil {
			_ = svc.harborClient.DeleteHarborProject(ctx, projectID, *robotCredential) // ベストエフォートで補償削除する
			return fmt.Errorf("harbor credential レコードの作成に失敗しました: %w", err)
		}

		// k8s namespace を作成する（失敗時は project 専用 robot で Harbor project を補償削除して DB ロールバック）
		if err := k8s.CreateNamespace(ctx, svc.k8sClient, projectData.Namespace); err != nil { // プロジェクト ID ベースの namespace 名を使う
			_ = svc.harborClient.DeleteHarborProject(ctx, projectID, *robotCredential) // ベストエフォートで補償削除する
			return fmt.Errorf("k8s namespace の作成に失敗しました: %w", err)
		}

		// すべての作成が成功したら status を active に更新する
		if err := svc.projectRepo.UpdateStatus(ctx, tx, projectData, models.ProjectStatusActive); err != nil {
			_ = svc.harborClient.DeleteHarborProject(ctx, projectID, *robotCredential)         // ベストエフォートで補償削除する
			_ = k8s.DeleteNamespace(ctx, svc.k8sClient, projectData.Namespace)                // ベストエフォートで補償削除する
			return fmt.Errorf("project ステータスの更新に失敗しました: %w", err)
		}
		projectData.Status = models.ProjectStatusActive // ローカルの値も更新する
		createdProject = projectData                    // 外側の変数に結果を格納する
		return nil
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

// DeleteProject は project を deleting 状態にし、配下の全 Deployment を削除してから k8s Namespace を削除する
// Namespace 削除後は WatchNamespaces goroutine が Project レコードを DB から削除する
func (svc *projectServiceImpl) DeleteProject(ctx context.Context, projectID string) error {
	var projectData *models.Project                                           // goroutine に渡すため外側で宣言する
	var deploymentList []models.Deployment                                    // goroutine に渡すため外側で宣言する
	var credentialData *models.HarborCredential                               // goroutine に渡すため外側で宣言する

	// トランザクション内で project status を deleting に更新し、全 Deployment を deleting に変更する
	err := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// project を取得する
		fetchedProject, err := svc.projectRepo.FindByID(ctx, tx, projectID)
		if err != nil {
			return fmt.Errorf("project の取得に失敗しました: %w", err)
		}
		projectData = fetchedProject // 外側の変数に格納する

		// project status を deleting に更新する
		if err := svc.projectRepo.UpdateStatus(ctx, tx, projectData, models.ProjectStatusDeleting); err != nil {
			return fmt.Errorf("project ステータスの更新に失敗しました: %w", err)
		}

		// 配下の全 Deployment を取得する
		fetchedDeployments, err := svc.deploymentRepo.FindAllByProjectID(ctx, projectID)
		if err != nil {
			return fmt.Errorf("deployment 一覧の取得に失敗しました: %w", err)
		}
		deploymentList = fetchedDeployments // 外側の変数に格納する

		// 全 Deployment の status を deleting に更新する
		for deploymentIndex := range deploymentList {
			deploymentList[deploymentIndex].Status = models.DeploymentStatusDeleting // ステータスを deleting に変更する
			if err := svc.deploymentRepo.Save(ctx, &deploymentList[deploymentIndex]); err != nil {
				return fmt.Errorf("deployment ステータスの更新に失敗しました (id=%s): %w", deploymentList[deploymentIndex].ID, err)
			}
		}

		// Harbor 認証情報を取得する（goroutine 内で Harbor project 削除に使用する）
		fetchedCredential, err := svc.harborCredentialRepo.FindByProjectID(ctx, tx, projectID)
		if err != nil {
			return fmt.Errorf("harbor credential の取得に失敗しました: %w", err)
		}
		credentialData = fetchedCredential // 外側の変数に格納する

		return nil
	})
	if err != nil {
		return err // トランザクションエラーを返す
	}

	// 全 Deployment の k8s リソース削除と Namespace 削除を goroutine で非同期に実行する
	// Namespace 削除後は WatchNamespaces が Project レコードを DB から削除する
	go svc.deleteProjectResources(projectData, deploymentList, credentialData)

	return nil // HTTP レスポンスはここで返し、削除処理はバックグラウンドで継続する
}

// deleteProjectResources は全 Deployment の k8s リソースを削除し、完了後に k8s Namespace を削除する
func (svc *projectServiceImpl) deleteProjectResources(projectData *models.Project, deploymentList []models.Deployment, credentialData *models.HarborCredential) {
	bgCtx := context.Background() // HTTP リクエストのコンテキストとは独立した context を使う

	// 全 Deployment の k8s リソースを並行削除する
	var waitGroup sync.WaitGroup // 全 Deployment 削除完了を待つ WaitGroup
	for deploymentIndex := range deploymentList {
		waitGroup.Add(1)
		go func(deploymentData models.Deployment) {
			defer waitGroup.Done()
			svc.deleteDeploymentK8sResources(bgCtx, projectData.Namespace, deploymentData) // k8s リソースを削除する
		}(deploymentList[deploymentIndex])
	}
	waitGroup.Wait() // 全 Deployment の k8s リソース削除完了を待つ

	// プロジェクトに紐づく IngressRoute を削除する（存在する場合のみ）
	ingressRouteData, ingressRouteErr := svc.ingressRouteRepo.FindByProjectID(bgCtx, projectData.ID) // IngressRoute を取得する
	if ingressRouteErr == nil && ingressRouteData != nil {                                            // IngressRoute が存在する場合
		if deleteRoutesErr := svc.ingressRouteRouteRepo.DeleteByIngressRouteID(bgCtx, nil, ingressRouteData.ID); deleteRoutesErr != nil { // 関連ルートエントリを全削除する
			logger.PrintErr("deleteProjectResources: ingress_route_route の削除に失敗しました: " + deleteRoutesErr.Error())
		}
		if k8sErr := k8s.DeleteIngressRoute(bgCtx, svc.dynamicClient, projectData.Namespace, ingressRouteData.ID); k8sErr != nil { // k8s IngressRoute を削除する
			_ = k8sErr // k8s 上に存在しない場合もあるため無視して継続する
		}
		if deleteErr := svc.ingressRouteRepo.Delete(bgCtx, ingressRouteData.ID); deleteErr != nil { // DB レコードを削除する
			logger.PrintErr("deleteProjectResources: ingress_route の削除に失敗しました: " + deleteErr.Error())
		}
	}

	// Harbor project を削除する
	if err := svc.harborClient.DeleteHarborProject(bgCtx, projectData.ID, k8s.HarborRobotCredential{ // Harbor プロジェクト名はプロジェクト ID
		Name:   credentialData.RobotName,   // DB に保存した robot 名を使う
		Secret: credentialData.RobotSecret, // DB に保存したシークレットを使う
	}); err != nil {
		logger.PrintErr("deleteProjectResources: harbor project の削除に失敗しました: " + err.Error())
	}

	// HarborCredential レコードを削除する
	if err := svc.harborCredentialRepo.DeleteByProjectID(bgCtx, svc.db, projectData.ID); err != nil {
		logger.PrintErr("deleteProjectResources: harbor credential レコードの削除に失敗しました: " + err.Error())
	}

	// k8s Namespace を削除する（削除後は WatchNamespaces goroutine が Project レコードを DB から削除する）
	if err := k8s.DeleteNamespace(bgCtx, svc.k8sClient, projectData.Namespace); err != nil {
		logger.PrintErr("deleteProjectResources: k8s namespace の削除に失敗しました: " + err.Error())
	}

	logger.Println("deleteProjectResources: namespace 削除完了 (projectID=" + projectData.ID + ", namespace=" + projectData.Namespace + ")")
}

// deleteDeploymentK8sResources は 1 つの Deployment に紐づく k8s リソースをすべて削除する
func (svc *projectServiceImpl) deleteDeploymentK8sResources(ctx context.Context, namespace string, deploymentData models.Deployment) {
	// k8s Deployment リソースを削除する（存在しない場合もあるため無視して継続する）
	if err := k8s.DeleteDeployment(ctx, svc.k8sClient, namespace, deploymentData.Name); err != nil {
		_ = err
	}
	// k8s Service リソースを削除する
	if err := k8s.DeleteService(ctx, svc.k8sClient, namespace, deploymentData.Name); err != nil {
		_ = err
	}
	// k8s ConfigMap を削除する
	if err := k8s.DeleteConfigMap(ctx, svc.k8sClient, namespace, deploymentData.Name); err != nil {
		_ = err
	}
	// k8s Secret を削除する
	if err := k8s.DeleteSecret(ctx, svc.k8sClient, namespace, deploymentData.Name); err != nil {
		_ = err
	}
}
