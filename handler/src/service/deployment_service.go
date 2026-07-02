package service

import (
	"context"
	"errors"
	"fmt"

	"handler/models"
	"handler/repository"

	temporalclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"
	k8sclient "k8s.io/client-go/kubernetes"
)

// DeploymentService は Deployment CRUD のビジネスロジックを定義するインターフェース
type DeploymentService interface {
	ListDeployments(ctx context.Context, projectID string) ([]models.Deployment, error)                                                         // deployment 一覧を取得する
	CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (*models.Deployment, error)                                              // deployment を作成する
	GetDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)                                          // deployment を取得する
	UpdateDeployment(ctx context.Context, userID string, deploymentID string, req UpdateDeploymentRequest) (*models.Deployment, error)          // deployment を更新する
	DeleteDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)                                       // deployment を削除（deleting 状態に変更）する
	DiscardPending(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)                                         // deployment の pending フィールドを全クリアする
	GetService(ctx context.Context, userID string, deploymentID string) (*models.Service, error)                                                // service 設定を取得する
	CreateService(ctx context.Context, userID string, deploymentID string, req CreateServiceRequest) (*models.Service, error)                   // service を作成する
	UpdateService(ctx context.Context, userID string, deploymentID string, req UpdateServiceRequest) (*models.Service, error)                   // service の pending フィールドを更新する
	DeleteService(ctx context.Context, userID string, deploymentID string) error                                                                // service を削除する
}

// CreateDeploymentRequest は POST /projects/:id/deployments のリクエスト構造体
type CreateDeploymentRequest struct {
	ProjectID           string   // プロジェクト ID
	Name                string   `json:"name"`              // デプロイメント名
	Type                string   `json:"type"`              // image_url / dockerfile / railpack
	ImageURL            string   `json:"image_url"`         // image_url 専用
	GithubRepoURL       string   `json:"github_repo_url"`   // GitHub リポジトリ URL
	GithubBranch        string   `json:"github_branch"`     // GitHub ブランチ名
	GithubCommitSHA     string   `json:"github_commit_sha"` // GitHub コミット SHA
	GithubRepoDirectory string   `json:"build_directory"`   // ビルド作業ディレクトリ
	DockerfilePath      string   `json:"dockerfile_path"`   // Dockerfile パス
	InstanceSize        string   `json:"instance_size"`     // インスタンスサイズ
	Replicas            int32    `json:"replicas"`          // レプリカ数
}

// CreateServiceRequest は POST /deployments/:id/service のリクエスト構造体
type CreateServiceRequest struct {
	Port       int    `json:"port"`        // 公開ポート番号
	TargetPort int    `json:"target_port"` // コンテナ内ポート番号
	Type       string `json:"type"`        // ClusterIP / NodePort / LoadBalancer
}

// UpdateServiceRequest は PUT /deployments/:id/service のリクエスト構造体
type UpdateServiceRequest struct {
	Port       *int `json:"port"`        // nil の場合は更新しない
	TargetPort *int `json:"target_port"` // nil の場合は更新しない
}

// UpdateDeploymentRequest は PUT /deployments/:id のリクエスト構造体
type UpdateDeploymentRequest struct {
	ImageURL            *string  `json:"image_url"`         // nil の場合は更新しない
	GithubRepoURL       *string  `json:"github_repo_url"`   // nil の場合は更新しない
	GithubBranch        *string  `json:"github_branch"`     // nil の場合は更新しない
	GithubCommitSHA     *string  `json:"github_commit_sha"` // nil の場合は更新しない
	GithubRepoDirectory *string  `json:"build_directory"`   // nil の場合は更新しない
	DockerfilePath      *string  `json:"dockerfile_path"`   // nil の場合は更新しない
	InstanceSize        *string  `json:"instance_size"`     // nil の場合は更新しない
	Replicas            *int32   `json:"replicas"`          // nil の場合は更新しない
	Command             []string `json:"command"`           // nil の場合は更新しない
	Args                []string `json:"args"`              // nil の場合は更新しない
}

// DeleteDeploymentWorkflowInput は DeleteDeployment Temporal Workflow への入力
type DeleteDeploymentWorkflowInput struct {
	DeploymentID string // 削除対象デプロイメントの ID
}

// deploymentServiceImpl は DeploymentService の実装
type deploymentServiceImpl struct {
	deploymentRepo   repository.DeploymentRepository      // deployment リポジトリ
	serviceRepo      repository.ServiceRepository         // service リポジトリ
	projectRepo      repository.ProjectRepository         // project リポジトリ（所有権チェック用）
	envVarMountRepo  repository.EnvVarMountRepository     // env_var_mount リポジトリ
	volumeMountRepo  repository.VolumeMountRepository     // volume_mount リポジトリ
	applyHistoryRepo repository.ApplyHistoryRepository    // apply_history リポジトリ（not_init 削除時の DB クリーンアップ用）
	buildRepo        repository.DeploymentBuildRepository // build リポジトリ（not_init 削除時の DB クリーンアップ用）
	imageRepo        repository.ImageRepository            // image リポジトリ（image_url 直接指定時の Image レコード作成用）
	userQuotaRepo    repository.UserQuotaRepository       // user_quota リポジトリ（Quotaチェック用）
	k8sClient        k8sclient.Interface                  // k8s クライアント（リソース存在確認用）
	temporalClient   WorkflowStarter                     // Temporal クライアント（DeleteDeployment Workflow 起動用）
}

// NewDeploymentService は DeploymentService の実装を返す
func NewDeploymentService(
	deploymentRepo repository.DeploymentRepository,
	serviceRepo repository.ServiceRepository,
	projectRepo repository.ProjectRepository,
	envVarMountRepo repository.EnvVarMountRepository,
	volumeMountRepo repository.VolumeMountRepository,
	applyHistoryRepo repository.ApplyHistoryRepository,
	buildRepo repository.DeploymentBuildRepository,
	imageRepo repository.ImageRepository,
	userQuotaRepo repository.UserQuotaRepository,
	k8sClient k8sclient.Interface,
	temporalClient WorkflowStarter,
) DeploymentService {
	return &deploymentServiceImpl{
		deploymentRepo:  deploymentRepo,   // deployment リポジトリを注入する
		serviceRepo:     serviceRepo,      // service リポジトリを注入する
		projectRepo:     projectRepo,      // project リポジトリを注入する
		envVarMountRepo: envVarMountRepo,  // env_var_mount リポジトリを注入する
		volumeMountRepo: volumeMountRepo,  // volume_mount リポジトリを注入する
		applyHistoryRepo: applyHistoryRepo, // apply_history リポジトリを注入する
		buildRepo:       buildRepo,        // build リポジトリを注入する
		imageRepo:       imageRepo,        // image リポジトリを注入する
		userQuotaRepo:   userQuotaRepo,    // user_quota リポジトリを注入する
		k8sClient:       k8sClient,        // k8s クライアントを注入する
		temporalClient:  temporalClient,   // Temporal クライアントを注入する
	}
}

// ListDeployments は projectID に紐づく deployment 一覧を返す
func (svc *deploymentServiceImpl) ListDeployments(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return svc.deploymentRepo.FindAllByProjectID(ctx, projectID) // リポジトリ経由で取得する
}

// CreateDeployment は Deployment レコードを作成する
func (svc *deploymentServiceImpl) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (*models.Deployment, error) {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, req.ProjectID) // project を取得してuserIDを解決する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := CheckDeploymentQuota(ctx, svc.userQuotaRepo, projectData.UserID); err != nil { // デプロイメント数のQuotaチェックを行う
		return nil, err // Quota超過エラーを返す
	}

	// デフォルト値を設定する
	if req.InstanceSize == "" {
		req.InstanceSize = "small" // インスタンスサイズのデフォルトを設定する
	}
	if req.Replicas == 0 {
		req.Replicas = 1 // レプリカ数のデフォルトを設定する
	}
	if req.DockerfilePath == "" {
		req.DockerfilePath = "./Dockerfile" // Dockerfile パスのデフォルトを設定する
	}
	if req.GithubRepoDirectory == "" {
		req.GithubRepoDirectory = "./" // ビルドディレクトリのデフォルトを設定する
	}

	// ImageURL が指定されている場合は即 pending、Githubビルド系は not_init にする
	initialStatus := models.DeploymentStatusPending // イメージURL直接指定の場合は pending から開始する
	if req.ImageURL == "" {                          // ImageURL が未指定の場合はビルドが必要なため not_init にする
		initialStatus = models.DeploymentStatusNotInit
	}

	// Deployment レコードを作成する
	deploymentData := &models.Deployment{
		ProjectID:                  req.ProjectID,                               // プロジェクト ID を設定する
		Name:                       req.Name,                                    // デプロイメント名を設定する
		Type:                       models.DeploymentType(req.Type),             // デプロイメントタイプを設定する
		Status:                     initialStatus,                               // 初期ステータスを設定する
		AppStatus:                  models.AppStatusPending,                     // 初期アプリステータスを設定する
		GithubRepoURL:              req.GithubRepoURL,                           // 作成時点の値を確定フィールドにも保持する
		PendingGithubRepoURL:       req.GithubRepoURL,                          // pending に設定する
		GithubBranch:               req.GithubBranch,                            // 作成時点の値を確定フィールドにも保持する
		PendingGithubBranch:        req.GithubBranch,                           // pending に設定する
		GithubCommitSHA:            req.GithubCommitSHA,                         // 作成時点の値を確定フィールドにも保持する
		PendingGithubCommitSHA:     req.GithubCommitSHA,                        // pending に設定する
		GithubRepoDirectory:        req.GithubRepoDirectory,                     // 作成時点の値を確定フィールドにも保持する
		PendingGithubRepoDirectory: req.GithubRepoDirectory,                    // pending に設定する
		PendingDockerfilePath:      req.DockerfilePath,                         // pending に設定する
		PendingInstanceSize:        req.InstanceSize,                           // pending に設定する
		PendingReplicas:            req.Replicas,                               // pending に設定する
	}

	if err := svc.deploymentRepo.Create(ctx, deploymentData); err != nil { // リポジトリ経由で Deployment レコードを作成する
		return nil, err // 作成エラーを返す
	}

	if req.ImageURL != "" { // 外部イメージURL直接指定の場合は Image レコードを作成する
		imageData := &models.Image{
			ProjectID: req.ProjectID, // プロジェクト ID を設定する
			BuildID:   nil,           // ビルドを経由しない直接指定のため nil
			ImageURL:  req.ImageURL,  // 指定された URL を設定する
		}
		if err := svc.imageRepo.Create(ctx, imageData); err != nil { // Image レコードを作成する
			return nil, err // 作成エラーを返す
		}
		if err := svc.deploymentRepo.UpdatePendingImageID(ctx, deploymentData.ID, imageData.ID); err != nil { // pending_image_id を更新する
			return nil, err // 更新エラーを返す
		}
		deploymentData.PendingImageID = &imageData.ID // 呼び出し元に返す値にも反映する
	}

	return deploymentData, nil // 作成した deployment を返す
}

// GetDeployment は deploymentID に対応する deployment を返す
func (svc *deploymentServiceImpl) GetDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	return deploymentData, nil
}

// UpdateDeployment は送られてきたフィールドのみ pending_*** を更新する
func (svc *deploymentServiceImpl) UpdateDeployment(ctx context.Context, userID string, deploymentID string, req UpdateDeploymentRequest) (*models.Deployment, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}

	if deploymentData.Status == models.DeploymentStatusNotInit { // not_init 状態では設定変更を禁止する
		return nil, errors.New("not_init 状態のデプロイメントは設定変更できません。先にビルドを実行してください")
	}

	if req.Replicas != nil { // replicas が指定されている場合のみQuotaチェックを行う
		if err := CheckReplicasQuota(ctx, svc.userQuotaRepo, userID, *req.Replicas); err != nil { // レプリカ数のQuotaチェックを行う
			return nil, err // Quota超過エラーを返す
		}
	}

	// 送られてきたフィールドのみ pending_*** に書き込む
	if req.ImageURL != nil { // 外部イメージURL直接指定の場合は Image レコードを作成してから pending_image_id を更新する
		imageData := &models.Image{
			ProjectID: deploymentData.ProjectID, // プロジェクト ID を設定する
			BuildID:   nil,                      // ビルドを経由しない直接指定のため nil
			ImageURL:  *req.ImageURL,             // 指定された URL を設定する
		}
		if err := svc.imageRepo.Create(ctx, imageData); err != nil { // Image レコードを作成する
			return nil, err // 作成エラーを返す
		}
		deploymentData.PendingImageID = &imageData.ID // pending_image_id を更新する
	}
	if req.GithubRepoURL != nil {
		deploymentData.PendingGithubRepoURL = *req.GithubRepoURL // pending github_repo_url を更新する
	}
	if req.GithubBranch != nil {
		deploymentData.PendingGithubBranch = *req.GithubBranch // pending github_branch を更新する
	}
	if req.GithubCommitSHA != nil {
		deploymentData.PendingGithubCommitSHA = *req.GithubCommitSHA // pending github_commit_sha を更新する
	}
	if req.GithubRepoDirectory != nil {
		deploymentData.PendingGithubRepoDirectory = *req.GithubRepoDirectory // pending build_directory を更新する
	}
	if req.DockerfilePath != nil {
		deploymentData.PendingDockerfilePath = *req.DockerfilePath // pending dockerfile_path を更新する
	}
	if req.InstanceSize != nil {
		deploymentData.PendingInstanceSize = *req.InstanceSize // pending instance_size を更新する
	}
	if req.Replicas != nil {
		deploymentData.PendingReplicas = *req.Replicas // pending replicas を更新する
	}
	if req.Command != nil {
		deploymentData.PendingCommand = req.Command // pending command を更新する
	}
	if req.Args != nil {
		deploymentData.PendingArgs = req.Args // pending args を更新する
	}

	if err := svc.deploymentRepo.Save(ctx, deploymentData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return deploymentData, nil // 更新後の deployment を返す
}

// DiscardPending は deployment の pending_* フィールドを現在の適用済み値に戻してクリアする
func (svc *deploymentServiceImpl) DiscardPending(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}

	if deploymentData.Status == models.DeploymentStatusNotInit { // not_init 状態では discard を禁止する
		return nil, errors.New("not_init 状態のデプロイメントは discard できません")
	}

	// pending フィールドを現在の適用済み値で上書きしてクリアする
	deploymentData.PendingImageID             = deploymentData.ImageID             // pending_image_id を現在値に戻す
	deploymentData.PendingGithubRepoURL       = deploymentData.GithubRepoURL       // pending_github_repo_url を現在値に戻す
	deploymentData.PendingGithubBranch        = deploymentData.GithubBranch        // pending_github_branch を現在値に戻す
	deploymentData.PendingGithubCommitSHA     = deploymentData.GithubCommitSHA     // pending_github_commit_sha を現在値に戻す
	deploymentData.PendingGithubRepoDirectory = deploymentData.GithubRepoDirectory // pending_github_repo_directory を現在値に戻す
	deploymentData.PendingDockerfilePath      = deploymentData.DockerfilePath      // pending_dockerfile_path を現在値に戻す
	deploymentData.PendingInstanceSize        = deploymentData.InstanceSize        // pending_instance_size を現在値に戻す
	deploymentData.PendingReplicas            = deploymentData.Replicas            // pending_replicas を現在値に戻す
	deploymentData.PendingCommand             = deploymentData.Command             // pending_command を現在値に戻す
	deploymentData.PendingArgs                = deploymentData.Args                // pending_args を現在値に戻す

	if err := svc.deploymentRepo.Save(ctx, deploymentData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return deploymentData, nil // 更新後の deployment を返す
}

// DeleteDeployment は deployment のステータスを deleting に変更し、k8s リソースを削除する
func (svc *deploymentServiceImpl) DeleteDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}

	// not_init 状態は k8s にリソースが存在しないため、直接 DB クリーンアップを実行する
	if deploymentData.Status == models.DeploymentStatusNotInit {
		if err := svc.deploymentRepo.ClearCurrentBuildID(ctx, deploymentData.ID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) { // current_build_id を NULL にして外部キー制約を解除する
			return nil, err // クリアエラーを返す
		}
		if err := svc.volumeMountRepo.DeleteAllByDeploymentID(ctx, nil, deploymentData.ID); err != nil { // volume_mount レコードを削除する
			return nil, err // 削除エラーを返す
		}
		if err := svc.deploymentRepo.Delete(ctx, deploymentData.ID); err != nil { // deployment レコードを削除する
			return nil, err // 削除エラーを返す
		}
		return deploymentData, nil // 削除完了を返す
	}

	// k8s リソースが存在する deployment は Temporal DeleteDeploymentWorkflow に委譲する
	deploymentData.Status         = models.DeploymentStatusDeleting // ステータスを deleting に変更する
	deploymentData.DeleteProgress = "削除ワークフローを開始中"          // 初期進捗を設定する
	if err := svc.deploymentRepo.Save(ctx, deploymentData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}

	workflowOptions := temporalclient.StartWorkflowOptions{
		ID:        "delete-deployment-" + deploymentID, // WorkflowID を設定して冪等性を保証する
		TaskQueue: "controller-queue",                  // controller Worker のタスクキューを指定する
	}
	workflowInput := DeleteDeploymentWorkflowInput{DeploymentID: deploymentID} // Workflow 入力を構築する
	_, startErr := svc.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "DeleteDeploymentWorkflow", workflowInput) // Workflow を起動する
	if startErr != nil {
		return nil, fmt.Errorf("deployment 削除 workflow の起動に失敗しました: %w", startErr) // 起動エラーを返す
	}

	return deploymentData, nil // 更新後の deployment を返す
}

// GetService は deploymentID に紐づく service 設定を返す
func (svc *deploymentServiceImpl) GetService(ctx context.Context, userID string, deploymentID string) (*models.Service, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	return svc.serviceRepo.FindByDeploymentID(ctx, deploymentID) // リポジトリ経由で service を取得する
}

// CreateService は deploymentID に紐づく service を作成する
func (svc *deploymentServiceImpl) CreateService(ctx context.Context, userID string, deploymentID string, req CreateServiceRequest) (*models.Service, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	serviceType := models.ServiceType(req.Type)         // リクエストのタイプを変換する
	if serviceType == "" {                               // タイプが未指定の場合はデフォルトを設定する
		serviceType = models.ServiceTypeClusterIP        // デフォルトは ClusterIP にする
	}
	serviceData := &models.Service{
		DeploymentID:      deploymentID,                  // deployment ID を設定する
		PendingPort:       req.Port,                      // pending_port を設定する（apply で反映される）
		PendingTargetPort: req.TargetPort,                // pending_target_port を設定する
		Type:              serviceType,                   // サービスタイプを設定する
		Status:            models.ServiceStatusPending,   // 初期ステータスを設定する
	}
	if err := svc.serviceRepo.Create(ctx, serviceData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return serviceData, nil // 作成した service を返す
}

// UpdateService は送られてきたフィールドのみ pending_* を更新する
func (svc *deploymentServiceImpl) UpdateService(ctx context.Context, userID string, deploymentID string, req UpdateServiceRequest) (*models.Service, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	serviceData, err := svc.serviceRepo.FindByDeploymentID(ctx, deploymentID) // リポジトリ経由で service を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if req.Port != nil {
		serviceData.PendingPort = *req.Port // pending_port を更新する
	}
	if req.TargetPort != nil {
		serviceData.PendingTargetPort = *req.TargetPort // pending_target_port を更新する
	}
	if err := svc.serviceRepo.Update(ctx, serviceData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return serviceData, nil // 更新後の service を返す
}

// DeleteService は pending_port=0・pending_target_port=0 にして apply 待ちにする（レコードは残す）
func (svc *deploymentServiceImpl) DeleteService(ctx context.Context, userID string, deploymentID string) error {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return err
	}
	serviceData, err := svc.serviceRepo.FindByDeploymentID(ctx, deploymentID) // service を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	serviceData.PendingPort = 0                         // pending_port を 0 にして無効化を予約する
	serviceData.PendingTargetPort = 0                   // pending_target_port を 0 にする
	serviceData.Status = models.ServiceStatusDeleting   // 削除待ち状態に変更する（apply で k8s から削除される）
	return svc.serviceRepo.Update(ctx, serviceData)     // DB に保存する
}

// ErrDeploymentNotFound は deployment が見つからない場合のエラー
var ErrDeploymentNotFound = gorm.ErrRecordNotFound

// ErrForbidden は操作対象リソースの所有者でない場合のエラー
var ErrForbidden = errors.New("forbidden")

// checkOwnership は project の UserID と userID が一致するか確認する
func (svc *deploymentServiceImpl) checkOwnership(ctx context.Context, userID string, projectID string) error {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}
	return nil
}
