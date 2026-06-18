package service

import (
	"app/k8s"
	"app/models"
	"app/repository"
	"context"
	"errors"

	"gorm.io/gorm"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"
)

// DeploymentService は Deployment CRUD のビジネスロジックを定義するインターフェース
type DeploymentService interface {
	ListDeployments(ctx context.Context, projectID string) ([]models.Deployment, error)                                                         // deployment 一覧を取得する
	CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (*models.Deployment, error)                                              // deployment を作成する
	GetDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)                                          // deployment を取得する
	UpdateDeployment(ctx context.Context, userID string, deploymentID string, req UpdateDeploymentRequest) (*models.Deployment, error)          // deployment を更新する
	DeleteDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)                                       // deployment を削除（deleting 状態に変更）する
	GetService(ctx context.Context, userID string, deploymentID string) (*models.Service, error)                                                // service 設定を取得する
	UpdateService(ctx context.Context, userID string, deploymentID string, req UpdateServiceRequest) (*models.Service, error)                   // service の pending フィールドを更新する
	GetIngressRoute(ctx context.Context, userID string, deploymentID string) (*models.IngressRoute, error)                                      // ingress_route 設定を取得する
	CreateIngressRoute(ctx context.Context, userID string, deploymentID string, req CreateIngressRouteRequest) (*models.IngressRoute, error)    // ingress_route を作成する
	UpdateIngressRoute(ctx context.Context, userID string, deploymentID string, req UpdateIngressRouteRequest) (*models.IngressRoute, error)    // ingress_route の pending フィールドを更新する
	DeleteService(ctx context.Context, userID string, deploymentID string) error                                                                // service を削除する
	DeleteIngressRoute(ctx context.Context, userID string, deploymentID string) error                                                           // ingress_route を削除する
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

// UpdateServiceRequest は PUT /deployments/:id/service のリクエスト構造体
type UpdateServiceRequest struct {
	Port       *int `json:"port"`        // nil の場合は更新しない
	TargetPort *int `json:"target_port"` // nil の場合は更新しない
}

// CreateIngressRouteRequest は POST /deployments/:id/ingress-route のリクエスト構造体
type CreateIngressRouteRequest struct {
	Host                string `json:"host"`                 // ホスト名
	PathPrefix          string `json:"path_prefix"`          // パスプレフィックス
	Port                int    `json:"port"`                 // 転送先ポート番号
	TLSEnabled          bool   `json:"tls_enabled"`          // TLS 有効化フラグ
	CertificateResolver string `json:"certificate_resolver"` // 証明書リゾルバー名
}

// UpdateIngressRouteRequest は PUT /deployments/:id/ingress-route のリクエスト構造体
type UpdateIngressRouteRequest struct {
	PathPrefix          *string `json:"path_prefix"`          // nil の場合は更新しない
	Port                *int    `json:"port"`                 // nil の場合は更新しない
	TLSEnabled          *bool   `json:"tls_enabled"`          // nil の場合は更新しない
	CertificateResolver *string `json:"certificate_resolver"` // nil の場合は更新しない
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

// deploymentServiceImpl は DeploymentService の実装
type deploymentServiceImpl struct {
	deploymentRepo   repository.DeploymentRepository   // deployment リポジトリ
	serviceRepo      repository.ServiceRepository      // service リポジトリ
	projectRepo      repository.ProjectRepository      // project リポジトリ（所有権チェック用）
	ingressRouteRepo repository.IngressRouteRepository // ingress_route リポジトリ
	envVarMountRepo  repository.EnvVarMountRepository  // env_var_mount リポジトリ
	volumeMountRepo  repository.VolumeMountRepository  // volume_mount リポジトリ
	userQuotaRepo    repository.UserQuotaRepository    // user_quota リポジトリ（Quotaチェック用）
	k8sClient        k8sclient.Interface               // k8s クライアント（リソース削除用）
	dynamicClient    dynamic.Interface                 // dynamic クライアント（IngressRoute 削除用）
	baseDomain       string                            // IngressRoute のホスト自動生成に使うベースドメイン
}

// NewDeploymentService は DeploymentService の実装を返す
func NewDeploymentService(
	deploymentRepo repository.DeploymentRepository,
	serviceRepo repository.ServiceRepository,
	projectRepo repository.ProjectRepository,
	ingressRouteRepo repository.IngressRouteRepository,
	envVarMountRepo repository.EnvVarMountRepository,
	volumeMountRepo repository.VolumeMountRepository,
	userQuotaRepo repository.UserQuotaRepository,
	k8sClient k8sclient.Interface,
	dynamicClient dynamic.Interface,
	baseDomain string,
) DeploymentService {
	return &deploymentServiceImpl{
		deploymentRepo:   deploymentRepo,   // deployment リポジトリを注入する
		serviceRepo:      serviceRepo,      // service リポジトリを注入する
		projectRepo:      projectRepo,      // project リポジトリを注入する
		ingressRouteRepo: ingressRouteRepo, // ingress_route リポジトリを注入する
		envVarMountRepo:  envVarMountRepo,  // env_var_mount リポジトリを注入する
		volumeMountRepo:  volumeMountRepo,  // volume_mount リポジトリを注入する
		userQuotaRepo:    userQuotaRepo,    // user_quota リポジトリを注入する
		k8sClient:        k8sClient,        // k8s クライアントを注入する
		dynamicClient:    dynamicClient,    // dynamic クライアントを注入する
		baseDomain:       baseDomain,       // ベースドメインを注入する
	}
}

// ListDeployments は projectID に紐づく deployment 一覧を返す
func (svc *deploymentServiceImpl) ListDeployments(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return svc.deploymentRepo.FindAllByProjectID(ctx, projectID) // リポジトリ経由で取得する
}

// CreateDeployment は Deployment レコードと Service レコードを作成する
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

	// Deployment レコードを作成する
	deploymentData := &models.Deployment{
		ProjectID:                  req.ProjectID,                               // プロジェクト ID を設定する
		Name:                       req.Name,                                    // デプロイメント名を設定する
		Type:                       models.DeploymentType(req.Type),             // デプロイメントタイプを設定する
		Status:                     models.DeploymentStatusPending,              // 初期ステータスを設定する
		AppStatus:                  models.AppStatusPending,                     // 初期アプリステータスを設定する
		PendingImageURL:            req.ImageURL,                                // pending に設定する
		PendingGithubRepoURL:       req.GithubRepoURL,                          // pending に設定する
		PendingGithubBranch:        req.GithubBranch,                           // pending に設定する
		PendingGithubCommitSHA:     req.GithubCommitSHA,                        // pending に設定する
		PendingGithubRepoDirectory: req.GithubRepoDirectory,                    // pending に設定する
		PendingDockerfilePath:      req.DockerfilePath,                         // pending に設定する
		PendingInstanceSize:        req.InstanceSize,                           // pending に設定する
		PendingReplicas:            req.Replicas,                               // pending に設定する
	}

	// TODO: Deployment と Service の作成をトランザクションでまとめ、Service 作成失敗時に Deployment もロールバックする
	if err := svc.deploymentRepo.Create(ctx, deploymentData); err != nil { // リポジトリ経由で Deployment レコードを作成する
		return nil, err // 作成エラーを返す
	}

	// Service レコードを同時に作成する（ports は空）
	serviceData := &models.Service{
		DeploymentID: deploymentData.ID,           // デプロイメント ID を設定する
		Status:       models.ServiceStatusPending, // 初期ステータスを設定する
	}
	if err := svc.serviceRepo.Create(ctx, serviceData); err != nil { // リポジトリ経由で Service レコードを作成する
		return nil, err // 作成エラーを返す
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

	if req.Replicas != nil { // replicas が指定されている場合のみQuotaチェックを行う
		if err := CheckReplicasQuota(ctx, svc.userQuotaRepo, userID, *req.Replicas); err != nil { // レプリカ数のQuotaチェックを行う
			return nil, err // Quota超過エラーを返す
		}
	}

	// 送られてきたフィールドのみ pending_*** に書き込む
	if req.ImageURL != nil {
		deploymentData.PendingImageURL = *req.ImageURL // pending image_url を更新する
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

// DeleteDeployment は deployment のステータスを deleting に変更し、k8s リソースを削除する
func (svc *deploymentServiceImpl) DeleteDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}

	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // namespace 解決のために project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	namespace := projectData.Namespace // namespace を取得する

	deploymentData.Status = models.DeploymentStatusDeleting                    // ステータスを deleting に変更する
	if err := svc.deploymentRepo.Save(ctx, deploymentData); err != nil {       // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}

	// k8s リソースを順に削除する（エラーは警告ログとして記録し処理を継続する）
	if k8sErr := k8s.DeleteDeployment(ctx, svc.k8sClient, namespace, deploymentData.Name); k8sErr != nil { // k8s Deployment を削除する
		_ = k8sErr // k8s 上に存在しない場合もあるため無視して継続する
	}
	if k8sErr := k8s.DeleteService(ctx, svc.k8sClient, namespace, deploymentData.Name); k8sErr != nil { // k8s Service を削除する
		_ = k8sErr // k8s 上に存在しない場合もあるため無視して継続する
	}
	if k8sErr := k8s.DeleteConfigMap(ctx, svc.k8sClient, namespace, deploymentData.Name); k8sErr != nil { // k8s ConfigMap を削除する
		_ = k8sErr // k8s 上に存在しない場合もあるため無視して継続する
	}
	if k8sErr := k8s.DeleteSecret(ctx, svc.k8sClient, namespace, deploymentData.Name); k8sErr != nil { // k8s Secret を削除する
		_ = k8sErr // k8s 上に存在しない場合もあるため無視して継続する
	}

	// IngressRoute が存在する場合は削除する
	ingressRouteData, ingressRouteErr := svc.ingressRouteRepo.FindByDeploymentID(ctx, deploymentID) // IngressRoute を取得する
	if ingressRouteErr == nil && ingressRouteData != nil {                                            // IngressRoute が存在する場合
		if k8sErr := k8s.DeleteIngressRoute(ctx, svc.dynamicClient, namespace, ingressRouteData.ID); k8sErr != nil { // k8s IngressRoute を削除する
			_ = k8sErr // k8s 上に存在しない場合もあるため無視して継続する
		}
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

// GetIngressRoute は deploymentID に紐づく ingress_route 設定を返す
func (svc *deploymentServiceImpl) GetIngressRoute(ctx context.Context, userID string, deploymentID string) (*models.IngressRoute, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	return svc.ingressRouteRepo.FindByDeploymentID(ctx, deploymentID) // リポジトリ経由で ingress_route を取得する
}

// CreateIngressRoute は deploymentID に紐づく ingress_route を作成する
func (svc *deploymentServiceImpl) CreateIngressRoute(ctx context.Context, userID string, deploymentID string, req CreateIngressRouteRequest) (*models.IngressRoute, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	host := req.Host                                                 // リクエストのホスト名を使う
	if host == "" {                                                  // ホストが未指定の場合は自動生成する
		host = deploymentID + "." + svc.baseDomain                   // deploymentID.baseDomain 形式で生成する
	}
	ingressRouteData := &models.IngressRoute{
		DeploymentID:        deploymentID,           // deployment ID を設定する
		Host:                host,                   // ホスト名を設定する（自動生成 or 指定値）
		PathPrefix:          req.PathPrefix,         // パスプレフィックスを設定する
		Port:                req.Port,               // ポート番号を設定する
		TLSEnabled:          req.TLSEnabled,         // TLS 有効化フラグを設定する
		CertificateResolver: req.CertificateResolver, // 証明書リゾルバーを設定する
		Status:              models.IngressRouteStatusPending, // 初期ステータスを設定する
	}
	if err := svc.ingressRouteRepo.Create(ctx, ingressRouteData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return ingressRouteData, nil // 作成した ingress_route を返す
}

// UpdateIngressRoute は送られてきたフィールドのみ pending_* を更新する
func (svc *deploymentServiceImpl) UpdateIngressRoute(ctx context.Context, userID string, deploymentID string, req UpdateIngressRouteRequest) (*models.IngressRoute, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return nil, err
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByDeploymentID(ctx, deploymentID) // リポジトリ経由で ingress_route を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if req.PathPrefix != nil {
		ingressRouteData.PendingPathPrefix = *req.PathPrefix // pending_path_prefix を更新する
	}
	if req.Port != nil {
		ingressRouteData.PendingPort = *req.Port // pending_port を更新する
	}
	if req.TLSEnabled != nil {
		ingressRouteData.PendingTLSEnabled = req.TLSEnabled // pending_tls_enabled を更新する
	}
	if req.CertificateResolver != nil {
		ingressRouteData.PendingCertificateResolver = *req.CertificateResolver // pending_certificate_resolver を更新する
	}
	if err := svc.ingressRouteRepo.Update(ctx, ingressRouteData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return ingressRouteData, nil // 更新後の ingress_route を返す
}

// DeleteService は deploymentID に紐づく k8s Service と DB レコードを削除する
func (svc *deploymentServiceImpl) DeleteService(ctx context.Context, userID string, deploymentID string) error {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return err
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // namespace 解決のために project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	serviceData, err := svc.serviceRepo.FindByDeploymentID(ctx, deploymentID) // service を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	k8sServiceName := deploymentData.Name + "-svc"                                                                    // k8s Service 名を生成する
	if err := k8s.DeleteService(ctx, svc.k8sClient, projectData.Namespace, k8sServiceName); err != nil {             // k8s から Service を削除する
		return err // 削除エラーを返す
	}
	return svc.serviceRepo.Delete(ctx, serviceData.ID) // DB レコードを削除する
}

// DeleteIngressRoute は deploymentID に紐づく k8s IngressRoute と DB レコードを削除する
func (svc *deploymentServiceImpl) DeleteIngressRoute(ctx context.Context, userID string, deploymentID string) error {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して所有権チェック用に使う
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkOwnership(ctx, userID, deploymentData.ProjectID); err != nil { // 所有権を確認する
		return err
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // namespace 解決のために project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByDeploymentID(ctx, deploymentID) // ingress_route を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := k8s.DeleteIngressRoute(ctx, svc.dynamicClient, projectData.Namespace, ingressRouteData.ID); err != nil { // k8s から IngressRoute を削除する
		return err // 削除エラーを返す
	}
	return svc.ingressRouteRepo.Delete(ctx, ingressRouteData.ID) // DB レコードを削除する
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
