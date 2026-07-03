package service

import (
	"context"
	"errors"
	"fmt"
	"os"

	"handler/k8s"
	"handler/models"
	"handler/repository"

	"go.temporal.io/sdk/client"
	temporalerr "go.temporal.io/sdk/temporal"
	"gorm.io/gorm"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
)

// ErrDuplicateEnvKey は apply 時に環境変数キーが重複している場合のエラー
var ErrDuplicateEnvKey = errors.New("duplicate env key: same key exists in env_var_mounts")

// ErrAlreadyApplying は apply 中の deployment に再 apply しようとした場合のエラー
var ErrAlreadyApplying = errors.New("already applying")

// ErrNotInitialized は not_init 状態の deployment に apply しようとした場合のエラー
var ErrNotInitialized = errors.New("deployment is not initialized: build must succeed first")

// ApplyWorkflowInput は Temporal ApplyWorkflow への入力（controller 側と同じ定義）
type ApplyWorkflowInput struct {
	DeploymentID string // apply 対象のデプロイメント ID
	BaseDomain   string // ホスト再生成に使うベースドメイン
}

// ApplyServiceInterface は apply サービスのインターフェース
type ApplyServiceInterface interface {
	Apply(ctx context.Context, userID string, deploymentID string) (*ApplyResult, error)                        // apply を実行する
	ApplyProject(ctx context.Context, userID string, projectID string) (*ApplyProjectResult, error)             // project 配下の Deployment・IngressRoute を一括 apply する
	GetProjectPendingSummary(ctx context.Context, userID string, projectID string) (*ProjectPendingSummary, error) // project 配下の pending 件数を集計する
	ListApplyHistories(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) // apply 履歴一覧を取得する
}

// ProjectPendingSummary は project 配下の pending 状況を集計した結果を表す構造体
type ProjectPendingSummary struct {
	HasPending               bool `json:"has_pending"`                 // pending が1件でもあるか
	PendingDeploymentCount   int  `json:"pending_deployment_count"`    // pending がある deployment 件数
	PendingIngressRouteCount int  `json:"pending_ingress_route_count"` // pending / deleting の ingress_route 件数
}

// ApplyProjectFailure は一括 apply で失敗した Deployment を表す構造体
type ApplyProjectFailure struct {
	DeploymentID string `json:"deployment_id"` // apply に失敗した deployment ID
	Error        string `json:"error"`         // 失敗理由
}

// ApplyProjectResult は ApplyProject 処理の結果を表す構造体
type ApplyProjectResult struct {
	AppliedDeploymentCount int                   `json:"applied_deployment_count"` // apply に成功した deployment 件数
	FailedDeploymentList   []ApplyProjectFailure `json:"failed_deployment_list"`   // apply に失敗した deployment 一覧
	IngressRouteApplied    bool                  `json:"ingress_route_applied"`    // IngressRoute の apply を実行したか
}

// WorkflowStarter は Temporal Workflow を起動・操作するための最小インターフェース（テストモック用）
type WorkflowStarter interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) // Workflow を起動する
	CancelWorkflow(ctx context.Context, workflowID string, runID string) error                                                                       // Workflow をキャンセルする
}

// ApplyService は apply のコアロジックを実装するサービス
type ApplyService struct {
	DB                *gorm.DB                          // データベース接続（トランザクション管理用）
	K8s               k8sclient.Interface               // k8s クライアント（ApplyProject で使用）
	DynamicClient     dynamic.Interface                 // dynamic クライアント（Traefik CRD 用）
	DeploymentRepo    repository.DeploymentRepository   // deployment リポジトリ
	ApplyHistoryRepo  repository.ApplyHistoryRepository // apply_history リポジトリ
	ProjectRepository repository.ProjectRepository      // project リポジトリ
	ServiceRepo       repository.ServiceRepository      // service リポジトリ
	IngressRouteRepo  repository.IngressRouteRepository // ingress_route リポジトリ
	PathRuleRepo      repository.PathRuleRepository     // path_rule リポジトリ
	UserQuotaRepo     repository.UserQuotaRepository    // user_quota リポジトリ（Quotaチェック用）
	TemporalClient    WorkflowStarter                   // Temporal クライアント（Apply Workflow 起動用）
	BaseDomain        string                            // ホスト再生成に使うベースドメイン
}

// ApplyResult は Apply 処理の結果を表す構造体
type ApplyResult struct {
	WorkflowID string // Temporal Workflow ID
}

// NewApplyService は ApplyService を生成して返す
func NewApplyService(
	db *gorm.DB,
	k8sClient k8sclient.Interface,
	dynamicClient dynamic.Interface,
	deploymentRepo repository.DeploymentRepository,
	applyHistoryRepo repository.ApplyHistoryRepository,
	projectRepository repository.ProjectRepository,
	serviceRepo repository.ServiceRepository,
	ingressRouteRepo repository.IngressRouteRepository,
	pathRuleRepo repository.PathRuleRepository,
	userQuotaRepo repository.UserQuotaRepository,
	temporalClient WorkflowStarter,
	baseDomain string,
) *ApplyService {
	return &ApplyService{ // 依存を注入して返す
		DB:                db,
		K8s:               k8sClient,
		DynamicClient:     dynamicClient,
		DeploymentRepo:    deploymentRepo,
		ApplyHistoryRepo:  applyHistoryRepo,
		ProjectRepository: projectRepository,
		ServiceRepo:       serviceRepo,
		IngressRouteRepo:  ingressRouteRepo,
		PathRuleRepo:      pathRuleRepo,
		UserQuotaRepo:     userQuotaRepo,
		TemporalClient:    temporalClient,
		BaseDomain:        baseDomain,
	}
}

// Apply は deployment の apply を Temporal Workflow として非同期に開始する
// バリデーション（所有権・ステータス確認）のみここで行い、実際の K8s 操作は controller の ApplyWorkflow に委譲する
func (applyService *ApplyService) Apply(ctx context.Context, userID string, deploymentID string) (*ApplyResult, error) {
	// 1. SELECT FOR UPDATE で排他ロックを取得しながらバリデーションを行う
	var workflowID string // 起動した Workflow の ID を格納する変数を定義する

	err := applyService.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		deploymentData, err := applyService.DeploymentRepo.FindByIDForUpdate(ctx, tx, deploymentID) // FOR UPDATE ロック付きで deployment を取得する
		if err != nil {
			return fmt.Errorf("deployment not found: %w", err) // 取得エラーを返す
		}

		ownerProjectData, ownerErr := applyService.ProjectRepository.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
		if ownerErr != nil {
			return fmt.Errorf("project not found: %w", ownerErr) // 取得エラーを返す
		}
		if ownerProjectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
			return ErrForbidden
		}

		if deploymentData.AppStatus == models.AppStatusDeploying { // 既に apply 中の場合は競合エラーを返す
			return ErrAlreadyApplying
		}
		if deploymentData.Status == models.DeploymentStatusDeleting { // 削除中の場合は競合エラーを返す
			return ErrAlreadyApplying
		}
		if deploymentData.Status == models.DeploymentStatusNotInit { // not_init 状態では apply を禁止する
			return ErrNotInitialized
		}

		replicas := deploymentData.PendingReplicas // pending の replicas を使う
		if replicas == 0 {                         // pending が 0 の場合は current 値を使う
			replicas = deploymentData.Replicas
		}
		if replicas == 0 { // current も 0 の場合はデフォルト値を設定する
			replicas = 1
		}
		if err := CheckReplicasQuota(ctx, applyService.UserQuotaRepo, userID, replicas); err != nil { // レプリカ数のQuotaチェックを行う
			return err // Quota超過エラーを返す
		}

		workflowID = "apply-" + deploymentID // Workflow ID を生成する（冪等性を保証する）

		// 2. Temporal Workflow を非同期で起動する（K8s 操作は controller の ApplyWorkflow に委譲する）
		workflowOptions := client.StartWorkflowOptions{
			ID:        workflowID,                                 // WorkflowID を設定して冪等性を保証する
			TaskQueue: "controller-queue",                         // controller Worker のタスクキューを指定する
		}
		baseDomain := applyService.BaseDomain // ベースドメインを取得する
		if baseDomain == "" {                 // 環境変数が未設定の場合はフォールバックする
			baseDomain = os.Getenv("BASE_DOMAIN")
		}
		workflowInput := ApplyWorkflowInput{
			DeploymentID: deploymentID, // apply 対象の deployment ID を設定する
			BaseDomain:   baseDomain,   // ベースドメインを設定する
		}
		_, startErr := applyService.TemporalClient.ExecuteWorkflow(ctx, workflowOptions, "ApplyWorkflow", workflowInput) // Workflow を起動する
		if startErr != nil {
			if isAlreadyStartedErr(startErr) { // 同一 WorkflowID が既に実行中の場合は二重 apply エラーを返す
				return ErrAlreadyApplying
			}
			return fmt.Errorf("workflow 起動に失敗しました: %w", startErr) // 起動エラーを返す
		}

		return nil // トランザクションをコミットする
	})
	if err != nil {
		return nil, err // エラーを返す
	}

	return &ApplyResult{WorkflowID: workflowID}, nil // Workflow ID を含む結果を返す
}

// isAlreadyStartedErr は Temporal の WorkflowExecutionAlreadyStarted エラーを判定する
func isAlreadyStartedErr(err error) bool {
	if err == nil { // nil の場合は false を返す
		return false
	}
	return temporalerr.IsWorkflowExecutionAlreadyStartedError(err) // Temporal の判定ヘルパーを使用する
}

// ApplyProject は projectID 配下の全 Deployment の pending 変更と、全 IngressRoute・PathRule を一括で k8s に apply する
// Deployment 群を先に apply し、その後に IngressRoute を apply する（Pod が新しい状態になってからルーティングを切り替える）
// 一部の Deployment の apply が失敗してもスキップして処理を継続する
func (applyService *ApplyService) ApplyProject(ctx context.Context, userID string, projectID string) (*ApplyProjectResult, error) {
	projectData, err := applyService.ProjectRepository.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}
	if projectData.UserID != userID { // 所有者チェック
		return nil, ErrForbidden // 所有者でない場合は禁止エラーを返す
	}

	result := &ApplyProjectResult{ // 結果を格納する構造体を初期化する
		FailedDeploymentList: []ApplyProjectFailure{},
	}

	deploymentList, err := applyService.DeploymentRepo.FindAllByProjectID(ctx, projectID) // project 配下の全 deployment を取得する
	if err != nil {
		return nil, fmt.Errorf("deployment 一覧取得に失敗しました: %w", err) // 取得エラーを返す
	}

	for _, deploymentData := range deploymentList { // pending がある deployment のみ apply する
		if !hasPendingChanges(&deploymentData) { // pending がない場合はスキップする
			continue
		}
		if _, applyErr := applyService.Apply(ctx, userID, deploymentData.ID); applyErr != nil { // 既存の Deployment apply ロジックを呼び出す
			result.FailedDeploymentList = append(result.FailedDeploymentList, ApplyProjectFailure{ // 失敗した deployment を記録して継続する
				DeploymentID: deploymentData.ID,
				Error:        applyErr.Error(),
			})
			continue
		}
		result.AppliedDeploymentCount++ // apply 成功件数をカウントする
	}

	ingressRouteList, err := applyService.IngressRouteRepo.FindAllByProjectID(ctx, projectID) // 全 IngressRoute を取得する
	if err != nil {
		return nil, fmt.Errorf("ingress_route 一覧取得に失敗しました: %w", err) // 取得エラーを返す
	}

	for _, ingressRouteData := range ingressRouteList { // 各 IngressRoute に対して apply 処理を行う
		if applyErr := applyService.applySingleIngressRoute(ctx, projectData.Namespace, ingressRouteData); applyErr != nil { // 個別 IngressRoute を apply する
			return nil, applyErr // エラーをそのまま返す
		}
	}
	result.IngressRouteApplied = true // IngressRoute の apply が完了したことを記録する

	return result, nil // 一括 apply の結果を返す
}

// GetProjectPendingSummary は projectID 配下の Deployment・IngressRoute の pending 件数を集計する
func (applyService *ApplyService) GetProjectPendingSummary(ctx context.Context, userID string, projectID string) (*ProjectPendingSummary, error) {
	projectData, err := applyService.ProjectRepository.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}
	if projectData.UserID != userID { // 所有者チェック
		return nil, ErrForbidden // 所有者でない場合は禁止エラーを返す
	}

	summary := &ProjectPendingSummary{} // 集計結果を格納する構造体を初期化する

	deploymentList, err := applyService.DeploymentRepo.FindAllByProjectID(ctx, projectID) // project 配下の全 deployment を取得する
	if err != nil {
		return nil, fmt.Errorf("deployment 一覧取得に失敗しました: %w", err) // 取得エラーを返す
	}
	for _, deploymentData := range deploymentList { // pending がある deployment をカウントする
		if hasPendingChanges(&deploymentData) {
			summary.PendingDeploymentCount++
		}
	}

	ingressRouteList, err := applyService.IngressRouteRepo.FindAllByProjectID(ctx, projectID) // project 配下の全 ingress_route を取得する
	if err != nil {
		return nil, fmt.Errorf("ingress_route 一覧取得に失敗しました: %w", err) // 取得エラーを返す
	}
	for _, ingressRouteData := range ingressRouteList { // pending がある ingress_route をカウントする
		ingressRoutePending, pendingErr := applyService.hasIngressRoutePendingChanges(ctx, ingressRouteData) // 個別に pending 有無を判定する
		if pendingErr != nil {
			return nil, pendingErr // 判定エラーを返す
		}
		if ingressRoutePending {
			summary.PendingIngressRouteCount++
		}
	}

	summary.HasPending = summary.PendingDeploymentCount > 0 || summary.PendingIngressRouteCount > 0 // いずれかに pending があれば true にする
	return summary, nil
}

// hasIngressRoutePendingChanges は IngressRoute 単体に未適用の pending 変更があるかを判定する
func (applyService *ApplyService) hasIngressRoutePendingChanges(ctx context.Context, ingressRouteData *models.IngressRoute) (bool, error) {
	if ingressRouteData.PendingName != "" { // 名前変更が保留中の場合
		return true, nil
	}
	pendingPathRuleList, err := applyService.PathRuleRepo.FindPendingByIngressRouteID(ctx, ingressRouteData.ID) // pending の PathRule を取得する
	if err != nil {
		return false, fmt.Errorf("pending path_rule 取得に失敗しました: %w", err) // 取得エラーを返す
	}
	if len(pendingPathRuleList) > 0 { // pending の PathRule がある場合
		return true, nil
	}
	deletingPathRuleList, err := applyService.PathRuleRepo.FindDeletingByIngressRouteID(ctx, ingressRouteData.ID) // deleting の PathRule を取得する
	if err != nil {
		return false, fmt.Errorf("deleting path_rule 取得に失敗しました: %w", err) // 取得エラーを返す
	}
	return len(deletingPathRuleList) > 0, nil // deleting の PathRule があれば true を返す
}

// hasPendingChanges は deployment に未適用の pending 変更があるかを判定する
func hasPendingChanges(deploymentData *models.Deployment) bool {
	if deploymentData.PendingImageID != nil && (deploymentData.ImageID == nil || *deploymentData.PendingImageID != *deploymentData.ImageID) { // イメージの変更
		return true
	}
	if deploymentData.PendingGithubRepoURL != "" && deploymentData.PendingGithubRepoURL != deploymentData.GithubRepoURL { // リポジトリURLの変更
		return true
	}
	if deploymentData.PendingGithubBranch != "" && deploymentData.PendingGithubBranch != deploymentData.GithubBranch { // ブランチの変更
		return true
	}
	if deploymentData.PendingGithubCommitSHA != "" && deploymentData.PendingGithubCommitSHA != deploymentData.GithubCommitSHA { // コミットSHAの変更
		return true
	}
	if deploymentData.PendingGithubRepoDirectory != "" && deploymentData.PendingGithubRepoDirectory != deploymentData.GithubRepoDirectory { // リポジトリディレクトリの変更
		return true
	}
	if deploymentData.PendingDockerfilePath != "" && deploymentData.PendingDockerfilePath != deploymentData.DockerfilePath { // Dockerfileパスの変更
		return true
	}
	if deploymentData.PendingInstanceSize != "" && deploymentData.PendingInstanceSize != deploymentData.InstanceSize { // インスタンスサイズの変更
		return true
	}
	if deploymentData.PendingReplicas != 0 && deploymentData.PendingReplicas != deploymentData.Replicas { // レプリカ数の変更
		return true
	}
	return false // pending 変更なし
}

// applySingleIngressRoute は1件の IngressRoute を k8s に apply する
func (applyService *ApplyService) applySingleIngressRoute(ctx context.Context, namespace string, ingressRouteData *models.IngressRoute) error {
	// status=deleting の場合は k8s から削除して DB レコードも物理削除する
	if ingressRouteData.Status == models.IngressRouteStatusDeleting {
		if delErr := k8s.DeleteIngressRoute(ctx, applyService.DynamicClient, namespace, ingressRouteData.ID); delErr != nil { // k8s IngressRoute を削除する
			if !k8serrors.IsNotFound(delErr) { // k8s に存在しない場合は無視して続行する（未 apply のまま削除する場合）
				return fmt.Errorf("k8s ingress_route delete: %w", delErr) // 削除エラーを返す
			}
		}
		if delErr := k8s.DeleteMiddleware(ctx, applyService.DynamicClient, namespace, ingressRouteData.ID); delErr != nil { // k8s Middleware を削除する
			if !k8serrors.IsNotFound(delErr) { // k8s に存在しない場合は無視して続行する
				return fmt.Errorf("k8s middleware delete: %w", delErr) // 削除エラーを返す
			}
		}
		allPathRuleList, allErr := applyService.PathRuleRepo.FindByIngressRouteID(ctx, ingressRouteData.ID) // 全 PathRule を取得する
		if allErr != nil {
			return fmt.Errorf("path_rule 一覧取得に失敗しました: %w", allErr) // 取得エラーを返す
		}
		for _, pathRuleItem := range allPathRuleList { // PathRule を全件物理削除する
			if deleteErr := applyService.PathRuleRepo.Delete(ctx, nil, pathRuleItem.ID); deleteErr != nil { // 物理削除する
				return fmt.Errorf("path_rule 削除に失敗しました: %w", deleteErr) // 削除エラーを返す
			}
		}
		if deleteErr := applyService.IngressRouteRepo.Delete(ctx, nil, ingressRouteData.ID); deleteErr != nil { // IngressRoute を物理削除する
			return fmt.Errorf("ingress_route 削除に失敗しました: %w", deleteErr) // 削除エラーを返す
		}
		return nil // 削除完了
	}

	// PendingName が設定されている場合はホスト名を再生成して昇格する
	if ingressRouteData.PendingName != "" {
		baseDomain := applyService.BaseDomain          // ベースドメインを取得する
		if baseDomain == "" {                          // 環境変数が未設定の場合はフォールバックする
			baseDomain = os.Getenv("BASE_DOMAIN")
		}
		newHost, newHostErr := generateUniqueHostForApply(ctx, applyService.IngressRouteRepo, ingressRouteData.PendingName, baseDomain) // 新ホスト名を生成する
		if newHostErr != nil {
			return fmt.Errorf("ホスト名の再生成に失敗しました: %w", newHostErr) // 生成エラーを返す
		}
		ingressRouteData.Name = ingressRouteData.PendingName // 名前を昇格する
		ingressRouteData.Host = newHost                      // ホスト名を更新する
		ingressRouteData.PendingName = ""                    // pending をクリアする
		if updateErr := applyService.IngressRouteRepo.Update(ctx, nil, ingressRouteData); updateErr != nil { // DB に保存する
			return fmt.Errorf("ingress_route 名前昇格の保存に失敗しました: %w", updateErr) // 保存エラーを返す
		}
	}

	pathRuleList, err := applyService.PathRuleRepo.FindActiveAndPendingByIngressRouteID(ctx, ingressRouteData.ID) // active/pending の PathRule 一覧を取得する
	if err != nil {
		return fmt.Errorf("path_rule 一覧の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	if len(pathRuleList) == 0 {
		// PathRule が 0 件の場合は k8s から IngressRoute を削除する
		if delErr := k8s.DeleteIngressRoute(ctx, applyService.DynamicClient, namespace, ingressRouteData.ID); delErr != nil { // k8s IngressRoute を削除する
			if !k8serrors.IsNotFound(delErr) { // k8s に存在しない場合は無視して続行する
				return fmt.Errorf("k8s ingress_route delete: %w", delErr) // 削除エラーを返す
			}
		}
	} else {
		// PathRule を集約して Service 情報を解決し k8s に apply する
		pathRuleSpecList := make([]k8s.PathRuleSpec, 0, len(pathRuleList)) // PathRuleSpec 一覧を初期化する
		stripPrefixList := make([]string, 0)                               // strip 対象プレフィックス一覧を初期化する
		for _, pathRuleItem := range pathRuleList {
			pathRuleService, serviceErr := applyService.ServiceRepo.FindByServiceID(ctx, pathRuleItem.ServiceID) // PathRule が指す Service を取得する
			if serviceErr != nil {
				return fmt.Errorf("path_rule の Service 取得に失敗しました (service_id=%s): %w", pathRuleItem.ServiceID, serviceErr) // 取得エラーを返す
			}
			pathRuleServicePort := pathRuleService.PendingPort // pending_port を優先して使う
			if pathRuleServicePort == 0 {                      // pending が 0 の場合は current 値を使う
				pathRuleServicePort = pathRuleService.Port
			}
			pathRuleSpecList = append(pathRuleSpecList, k8s.PathRuleSpec{ // PathRuleSpec を追加する
				PathPrefix:  pathRuleItem.PathPrefix,
				ServiceName: pathRuleService.ID + "-svc", // k8s Service 名は Service UUID ベース（Deployment 名変更の影響を受けない）
				ServicePort: pathRuleServicePort,
				StripPrefix: pathRuleItem.StripPrefix, // strip_prefix フラグを設定する
			})
			if pathRuleItem.StripPrefix { // strip_prefix が有効な場合はプレフィックスを収集する
				stripPrefixList = append(stripPrefixList, pathRuleItem.PathPrefix)
			}
		}
		if applyErr := k8s.ApplyMiddleware(ctx, applyService.DynamicClient, ingressRouteData.ID, namespace, stripPrefixList); applyErr != nil { // Middleware を apply する（strip 0 件でも空 prefixes で作成する）
			return fmt.Errorf("k8s middleware apply: %w", applyErr) // apply エラーを返す
		}
		if applyErr := k8s.ApplyIngressRoute(ctx, applyService.DynamicClient, *ingressRouteData, namespace, pathRuleSpecList); applyErr != nil { // k8s に IngressRoute を apply する
			return fmt.Errorf("k8s ingress_route apply: %w", applyErr) // apply エラーを返す
		}
	}

	// IngressRoute の status を active に更新する
	if updateErr := applyService.IngressRouteRepo.UpdateStatus(ctx, ingressRouteData.ID, models.IngressRouteStatusActive, ingressRouteData.K8sStatus); updateErr != nil { // status を active に更新する
		return fmt.Errorf("ingress_route status 更新に失敗しました: %w", updateErr) // 更新エラーを返す
	}

	// PathRule の pending→active 昇格・deleting→物理削除を行う
	pendingPathRuleList, pendingErr := applyService.PathRuleRepo.FindPendingByIngressRouteID(ctx, ingressRouteData.ID) // pending の PathRule を取得する
	if pendingErr != nil {
		return fmt.Errorf("pending path_rule 取得に失敗しました: %w", pendingErr) // 取得エラーを返す
	}
	for _, pendingPathRule := range pendingPathRuleList { // pending の PathRule を active に昇格する
		if updateErr := applyService.PathRuleRepo.UpdateStatus(ctx, nil, pendingPathRule.ID, models.PathRuleStatusActive); updateErr != nil { // status を active に更新する
			return fmt.Errorf("path_rule status 更新に失敗しました: %w", updateErr) // 更新エラーを返す
		}
	}

	deletingPathRuleList, deletingErr := applyService.PathRuleRepo.FindDeletingByIngressRouteID(ctx, ingressRouteData.ID) // deleting の PathRule を取得する
	if deletingErr != nil {
		return fmt.Errorf("deleting path_rule 取得に失敗しました: %w", deletingErr) // 取得エラーを返す
	}
	for _, deletingPathRule := range deletingPathRuleList { // deleting の PathRule を物理削除する
		if deleteErr := applyService.PathRuleRepo.Delete(ctx, nil, deletingPathRule.ID); deleteErr != nil { // 物理削除する
			return fmt.Errorf("path_rule 削除に失敗しました: %w", deleteErr) // 削除エラーを返す
		}
	}

	return nil // 正常終了
}

// generateUniqueHostForApply は apply.go 内でホスト衝突チェック付きの新ホスト名を生成するパッケージレベル関数
func generateUniqueHostForApply(ctx context.Context, ingressRouteRepo repository.IngressRouteRepository, name string, baseDomain string) (string, error) {
	for retryIndex := 0; retryIndex < 5; retryIndex++ { // 最大5回リトライする
		suffix, suffixErr := generateRandomSuffix(8) // 8文字のランダムサフィックスを生成する
		if suffixErr != nil {
			return "", suffixErr // 生成エラーを返す
		}
		host := fmt.Sprintf("%s-%s.%s", name, suffix, baseDomain) // {name}-{suffix}.{baseDomain} 形式でホストを生成する
		exists, existsErr := ingressRouteRepo.ExistsByHost(ctx, nil, host) // ホストの重複チェック
		if existsErr != nil {
			return "", existsErr // 確認エラーを返す
		}
		if !exists { // 重複なしの場合は生成したホストを返す
			return host, nil
		}
	}
	return "", fmt.Errorf("ホスト名の生成に失敗しました（最大リトライ回数に達しました）") // リトライ上限に達した場合はエラーを返す
}

// ListApplyHistories は deploymentID に紐づく apply 履歴一覧を返す
func (applyService *ApplyService) ListApplyHistories(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
	deploymentData, err := applyService.DeploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	projectData, err := applyService.ProjectRepository.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	if projectData.UserID != userID { // 所有者でない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	historyList, err := applyService.ApplyHistoryRepo.FindAllByDeploymentID(ctx, deploymentID) // 履歴一覧を取得する
	return historyList, err                                                                     // 結果とエラーを返す
}
