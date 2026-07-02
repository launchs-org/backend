package activity

import (
	"app/shared/logger"
	"app/shared/models"
	"app/shared/repository"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"controller/k8s"
	"controller/k8s/manifest"

	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gorm.io/gorm"
)

// ApplyActivityInput は Apply 系 Activity への共通入力
type ApplyActivityInput struct {
	DeploymentID string // 対象デプロイメントのID
	BaseDomain   string // ベースドメイン
}

// ApplyActivities は ApplyWorkflow で使われる Activity 群を保持する構造体
type ApplyActivities struct {
	db               *gorm.DB                            // DB接続（トランザクション用）
	k8sClient        k8sclient.Interface                 // k8s クライアント
	dynamicClient    dynamic.Interface                   // dynamic クライアント（Traefik CRD 用）
	deploymentRepo   repository.DeploymentRepository     // deployment リポジトリ
	applyHistoryRepo repository.ApplyHistoryRepository   // apply_history リポジトリ
	projectRepo      repository.ProjectRepository        // project リポジトリ
	serviceRepo      repository.ServiceRepository        // service リポジトリ
	ingressRouteRepo repository.IngressRouteRepository   // ingress_route リポジトリ
	pathRuleRepo     repository.PathRuleRepository       // path_rule リポジトリ
	envVarRepo       repository.EnvVarRepository         // env_var リポジトリ
	envVarMountRepo  repository.EnvVarMountRepository    // env_var_mount リポジトリ
	volumeRepo       repository.VolumeRepository         // volume リポジトリ
	volumeMountRepo  repository.VolumeMountRepository    // volume_mount リポジトリ
}

// NewApplyActivities は ApplyActivities を生成して返す
func NewApplyActivities(
	db *gorm.DB,
	k8sClient k8sclient.Interface,
	dynamicClient dynamic.Interface,
	deploymentRepo repository.DeploymentRepository,
	applyHistoryRepo repository.ApplyHistoryRepository,
	projectRepo repository.ProjectRepository,
	serviceRepo repository.ServiceRepository,
	ingressRouteRepo repository.IngressRouteRepository,
	pathRuleRepo repository.PathRuleRepository,
	envVarRepo repository.EnvVarRepository,
	envVarMountRepo repository.EnvVarMountRepository,
	volumeRepo repository.VolumeRepository,
	volumeMountRepo repository.VolumeMountRepository,
) *ApplyActivities {
	return &ApplyActivities{ // 依存を注入して返す
		db:               db,
		k8sClient:        k8sClient,
		dynamicClient:    dynamicClient,
		deploymentRepo:   deploymentRepo,
		applyHistoryRepo: applyHistoryRepo,
		projectRepo:      projectRepo,
		serviceRepo:      serviceRepo,
		ingressRouteRepo: ingressRouteRepo,
		pathRuleRepo:     pathRuleRepo,
		envVarRepo:       envVarRepo,
		envVarMountRepo:  envVarMountRepo,
		volumeRepo:       volumeRepo,
		volumeMountRepo:  volumeMountRepo,
	}
}

// ApplyResultData は ApplyActivity の実行結果を表す
type ApplyResultData struct {
	ApplyHistoryID       string // apply_history の ID
	AppliedServiceID     string // ClusterIP 同期用の Service ID
	AppliedServiceNamespace string // ClusterIP 同期用の Namespace
}

// ExecuteApply は pending→current 昇格・Manifest生成・k8s Apply・ApplyHistory記録を一括で行う Activity
func (activities *ApplyActivities) ExecuteApply(ctx context.Context, input ApplyActivityInput) (*ApplyResultData, error) {
	var applyResult *ApplyResultData       // 結果を格納する変数を定義する
	var appliedServiceID string            // Apply した Service の ID（ClusterIP 同期に使う）
	var appliedServiceNamespace string     // Apply した Service の Namespace（ClusterIP 同期に使う）

	err := activities.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		// 1. SELECT FOR UPDATE でロックを取得してデプロイメントを取得する
		deploymentData, err := activities.deploymentRepo.FindByIDForUpdate(ctx, tx, input.DeploymentID) // FOR UPDATE ロック付きで deployment を取得する
		if err != nil {
			return fmt.Errorf("deployment not found: %w", err) // 取得エラーを返す
		}

		// 削除中の deployment への apply を防ぐ
		if deploymentData.Status == models.DeploymentStatusDeleting { // 削除中の場合はエラーを返す
			return fmt.Errorf("deployment is deleting") // 削除中エラーを返す
		}

		// 2. Project を取得する（namespace 解決のため）
		projectData, err := activities.projectRepo.FindByID(ctx, tx, deploymentData.ProjectID) // project を取得する
		if err != nil {
			return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
		}

		// 3. pending_*** から使用する実効値を決定する
		imageURL := deploymentData.PendingImageURL // pending の image_url を使う
		if imageURL == "" {                        // pending が空の場合は current 値を使う
			imageURL = deploymentData.ImageURL
		}

		instanceSize := deploymentData.PendingInstanceSize // pending の instance_size を使う
		if instanceSize == "" {                            // pending が空の場合は current 値を使う
			instanceSize = deploymentData.InstanceSize
		}

		replicas := deploymentData.PendingReplicas // pending の replicas を使う
		if replicas == 0 {                         // pending が 0 の場合は current 値を使う
			replicas = deploymentData.Replicas
		}
		if replicas == 0 { // current も 0 の場合はデフォルト値を設定する
			replicas = 1
		}

		command := deploymentData.PendingCommand // pending の command を使う
		if len(command) == 0 {                   // pending が空の場合は current 値を使う
			command = deploymentData.Command
		}

		args := deploymentData.PendingArgs // pending の args を使う
		if len(args) == 0 {                // pending が空の場合は current 値を使う
			args = deploymentData.Args
		}

		// 4. instance_size マスターを取得してマニフェスト生成用データを組み立てる
		var instanceSizeData models.InstanceSize                                                          // instance_size を格納する変数を定義する
		if err := tx.WithContext(ctx).First(&instanceSizeData, "size = ?", instanceSize).Error; err != nil { // instance_size マスターを取得する
			return fmt.Errorf("instance_size '%s' が instance_sizes テーブルに存在しません: %w", instanceSize, err) // レコードが見つからない場合はエラーを返す
		}

		deploymentForManifest := *deploymentData          // manifest 生成用にコピーする
		deploymentForManifest.InstanceSize = instanceSize // 実効 instance_size を設定する
		deploymentForManifest.Replicas = replicas         // 実効 replicas を設定する
		deploymentForManifest.Command = command           // 実効 command を設定する
		deploymentForManifest.Args = args                 // 実効 args を設定する

		// 5. EnvVarMount 一覧を取得して ConfigMap/Secret データを構築する
		envVarMountList, err := activities.envVarMountRepo.FindAllByDeploymentID(ctx, input.DeploymentID) // deployment に紐づくマウント設定一覧を取得する
		if err != nil {
			return fmt.Errorf("env_var_mount list: %w", err) // 取得エラーを返す
		}

		configMapData := map[string]string{} // ConfigMap 用の非シークレット環境変数を格納するマップ
		secretData := map[string][]byte{}    // Secret 用のシークレット環境変数を格納するマップ
		keySet := map[string]bool{}          // キー名重複チェック用のセット
		var duplicateKeyErr error            // 重複キーエラーを一時保存する変数

		for _, mountItem := range envVarMountList { // マウント設定ごとに環境変数を解決する
			envVarData, envVarErr := activities.envVarRepo.FindByID(ctx, mountItem.EnvVarID) // env_var を取得する
			if envVarErr != nil {
				return fmt.Errorf("env_var not found (id=%s): %w", mountItem.EnvVarID, envVarErr) // 取得エラーを返す
			}

			effectiveKey := envVarData.Key   // 実効キー名を決定する（デフォルトは元のキー）
			if mountItem.OverrideKey != "" { // override_key が設定されている場合はそちらを使う
				effectiveKey = mountItem.OverrideKey
			}

			if keySet[effectiveKey] { // キー名が重複している場合はエラーを保存する
				duplicateKeyErr = fmt.Errorf("duplicate env key: key=%s", effectiveKey) // 重複エラーを保存する
				break                                                                    // ループを抜ける
			}
			keySet[effectiveKey] = true // キーをセットに追加する

			if envVarData.IsSecret { // is_secret が true の場合は Secret に分類する
				secretData[effectiveKey] = []byte(envVarData.Value) // Secret データに追加する
			} else {
				configMapData[effectiveKey] = envVarData.Value // ConfigMap データに追加する
			}
		}

		// 5-2. VolumeMount 一覧を取得する
		volumeMountList, volumeMountErr := activities.volumeMountRepo.FindAllByDeploymentID(ctx, input.DeploymentID) // deployment に紐づく VolumeMount 一覧を取得する
		if volumeMountErr != nil {
			return fmt.Errorf("volume_mount list: %w", volumeMountErr) // 取得エラーを返す
		}

		volumeMountValues := make([]models.VolumeMount, len(volumeMountList)) // ポインタスライスを値スライスに変換する
		for mountIndex, mountPtr := range volumeMountList {                   // VolumeMount を値スライスに変換する
			volumeMountValues[mountIndex] = *mountPtr
		}

		// 5-2-2. VolumeMount に紐づく PVC を k8s に apply する（未作成の場合のみ作成する）
		for _, volumeMountItem := range volumeMountList {
			if volumeMountItem.Status == models.VolumeMountStatusDeleting { // deleting は apply 不要なのでスキップする
				continue
			}
			volumeData, volumeErr := activities.volumeRepo.FindByID(ctx, volumeMountItem.VolumeID) // Volume を取得する
			if volumeErr != nil {
				return fmt.Errorf("volume find: %w", volumeErr) // 取得エラーを返す
			}
			pvcName := volumeData.ID + "-pvc"                                                                               // PVC 名を生成する
			pvcManifest := k8s.BuildPVCManifest(projectData.Namespace, pvcName, volumeData.SizeMB, "")                     // PVC マニフェストを生成する
			if pvcErr := k8s.ApplyPVC(ctx, activities.k8sClient, pvcManifest); pvcErr != nil {                             // k8s に PVC を apply する
				applyHistoryRecord := &models.ApplyHistory{DeploymentID: input.DeploymentID, Status: models.ApplyStatusFailed, ErrorMessage: pvcErr.Error(), AppliedAt: time.Now()} // apply_history を生成する
				if err := activities.applyHistoryRepo.Create(ctx, tx, applyHistoryRecord); err != nil {                    // apply_history を作成する
					return fmt.Errorf("apply_history create: %w", err) // 作成エラーを返す
				}
				return fmt.Errorf("k8s pvc apply: %w", pvcErr) // PVC apply エラーを返す
			}
		}

		// 5-3. k8s Deployment マニフェストを生成する
		envVarMountValues := make([]models.EnvVarMount, len(envVarMountList)) // ポインタスライスを値スライスに変換する
		for mountIndex, mountPtr := range envVarMountList {                   // マウント設定を値スライスに変換する
			envVarMountValues[mountIndex] = *mountPtr
		}
		manifestGenerator := &manifest.Generator{ // マニフェストジェネレーターを生成する
			InstanceSizes: map[string]models.InstanceSize{instanceSize: instanceSizeData},
		}
		deploymentManifest := manifestGenerator.GenerateDeployment(deploymentForManifest, projectData.Namespace, imageURL, envVarMountValues, volumeMountValues) // マニフェストを生成する

		// 6. apply_history を INSERT する
		manifestJSON, _ := json.Marshal(deploymentManifest)  // マニフェストを JSON にシリアライズする
		applyHistoryRecord := &models.ApplyHistory{          // apply_history レコードを生成する
			DeploymentID: input.DeploymentID,
			Manifests:    manifestJSON,
			Status:       models.ApplyStatusApplied, // 初期ステータスは applied とする
			AppliedAt:    time.Now(),
		}
		if err := activities.applyHistoryRepo.Create(ctx, tx, applyHistoryRecord); err != nil { // apply_history を作成する
			return fmt.Errorf("apply_history create: %w", err) // 作成エラーを返す
		}

		// 6-2. 重複キーが存在した場合は apply_history を failed にしてエラーを返す
		if duplicateKeyErr != nil { // 重複キーエラーが保存されている場合は処理する
			if updateErr := activities.applyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
				return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
			}
			return duplicateKeyErr // 重複キーエラーを返す
		}

		// 7-0. k8s に ConfigMap を apply する（非シークレット環境変数が存在する場合のみ）
		if len(configMapData) > 0 { // ConfigMap データが存在する場合のみ apply する
			if err := k8s.ApplyConfigMap(ctx, activities.k8sClient, projectData.Namespace, deploymentData.Name, configMapData); err != nil { // k8s に ConfigMap を apply する
				if updateErr := activities.applyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
					return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
				}
				return fmt.Errorf("k8s configmap apply: %w", err) // k8s ConfigMap apply エラーを返す
			}
		}

		// 7-0-2. k8s に Secret を apply する（シークレット環境変数が存在する場合のみ）
		if len(secretData) > 0 { // Secret データが存在する場合のみ apply する
			if err := k8s.ApplySecret(ctx, activities.k8sClient, projectData.Namespace, deploymentData.Name, secretData); err != nil { // k8s に Secret を apply する
				if updateErr := activities.applyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
					return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
				}
				return fmt.Errorf("k8s secret apply: %w", err) // k8s Secret apply エラーを返す
			}
		}

		// 7. k8s に Deployment を apply する
		if err := k8s.ApplyDeployment(ctx, activities.k8sClient, deploymentManifest); err != nil { // k8s に Deployment を apply する
			if updateErr := activities.applyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
				return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
			}
			return fmt.Errorf("k8s deployment apply: %w", err) // k8s Deployment apply エラーを返す
		}

		// 7-2. k8s に Service を apply する（status ベースで操作を決定する）
		var serviceData *models.Service                                                           // Service レコードを格納する変数を宣言する
		serviceData, _ = activities.serviceRepo.FindByDeploymentID(ctx, input.DeploymentID)      // Service レコードを取得する（存在しない場合は nil）
		if serviceData != nil {
			if serviceData.Status == models.ServiceStatusDeleting { // status=deleting の場合は k8s から Service を削除する
				k8sServiceName := serviceData.ID + "-svc"                                                                // Service 名は Service UUID ベース
				if delErr := k8s.DeleteService(ctx, activities.k8sClient, projectData.Namespace, k8sServiceName); delErr != nil { // k8s Service を削除する
					if updateErr := activities.applyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
						return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
					}
					return fmt.Errorf("k8s service delete: %w", delErr) // 削除エラーを返す
				}
			} else if serviceData.PendingPort != 0 { // pending_port が設定されている場合のみ apply する
				serviceManifest := manifestGenerator.GenerateService(*serviceData, deploymentData.Name, projectData.Namespace) // Service マニフェストを生成する
				if err := k8s.ApplyService(ctx, activities.k8sClient, serviceManifest); err != nil {                          // k8s に Service を apply する
					if updateErr := activities.applyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
						return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
					}
					return fmt.Errorf("k8s service apply: %w", err) // k8s Service apply エラーを返す
				}
				appliedServiceID = serviceData.ID             // ClusterIP 同期のために Service ID を記録する
				appliedServiceNamespace = projectData.Namespace // ClusterIP 同期のために Namespace を記録する
			}
		}

		// 8. pending_*** を空にして current 値に昇格させる
		appliedAt := time.Now() // apply 完了時刻を記録する
		updates := map[string]interface{}{
			"image_url":                     imageURL,
			"pending_image_url":             "",
			"instance_size":                 instanceSize,
			"pending_instance_size":         "",
			"replicas":                      replicas,
			"pending_replicas":              0,
			"github_repo_url":               deploymentData.PendingGithubRepoURL,
			"pending_github_repo_url":       "",
			"github_branch":                 deploymentData.PendingGithubBranch,
			"pending_github_branch":         "",
			"github_commit_sha":             deploymentData.PendingGithubCommitSHA,
			"pending_github_commit_sha":     "",
			"github_repo_directory":         deploymentData.PendingGithubRepoDirectory,
			"pending_github_repo_directory": "",
			"dockerfile_path":               deploymentData.PendingDockerfilePath,
			"pending_dockerfile_path":       "",
			"command":                       command,
			"pending_command":               nil,
			"args":                          args,
			"pending_args":                  nil,
			"status":                        models.DeploymentStatusRunning,
			"app_status":                    models.AppStatusDeploying,
			"applied_at":                    &appliedAt,
		}
		if err := activities.deploymentRepo.Updates(ctx, tx, deploymentData, updates); err != nil { // deployment を更新する
			return fmt.Errorf("deployment updates: %w", err) // 更新エラーを返す
		}

		// 9. Service の pending_*** を昇格させる
		if serviceData != nil { // Service レコードが存在する場合のみ昇格する
			if serviceData.Status == models.ServiceStatusDeleting { // status=deleting の場合は port をクリアして pending に戻す
				serviceData.Port = 0
				serviceData.TargetPort = 0
				serviceData.PendingPort = 0
				serviceData.PendingTargetPort = 0
				serviceData.Status = models.ServiceStatusPending // 未設定状態に戻す
			} else if serviceData.PendingPort != 0 { // pending_port が設定されている場合のみ昇格する
				serviceData.Port = serviceData.PendingPort             // pending_port を昇格する
				serviceData.PendingPort = 0                            // pending_port をクリアする
				serviceData.TargetPort = serviceData.PendingTargetPort // pending_target_port を昇格する
				serviceData.PendingTargetPort = 0                      // pending_target_port をクリアする
				serviceData.Status = models.ServiceStatusActive        // active にする
			}
			if err := activities.serviceRepo.Update(ctx, serviceData); err != nil { // Service を更新する
				return fmt.Errorf("service update: %w", err) // 更新エラーを返す
			}
		}

		// 11. EnvVarMount の status を昇格させる（pending → applied、deleting → 物理削除）
		for _, mountItem := range envVarMountList {
			if mountItem.Status == models.EnvVarMountStatusDeleting { // deleting は物理削除する
				if deleteErr := activities.envVarMountRepo.Delete(ctx, tx, mountItem); deleteErr != nil {
					return fmt.Errorf("env_var_mount delete: %w", deleteErr) // 削除エラーを返す
				}
			} else if mountItem.Status == models.EnvVarMountStatusPending { // pending は applied に昇格する
				if updateErr := activities.envVarMountRepo.UpdateStatus(ctx, tx, mountItem, models.EnvVarMountStatusApplied); updateErr != nil {
					return fmt.Errorf("env_var_mount update status: %w", updateErr) // 更新エラーを返す
				}
			}
		}

		// 12. VolumeMount の status を昇格させる（pending → mounted、deleting → 物理削除）
		for _, volumeMountItem := range volumeMountList {
			if volumeMountItem.Status == models.VolumeMountStatusDeleting { // deleting は物理削除する
				if deleteErr := activities.volumeMountRepo.Delete(ctx, tx, volumeMountItem); deleteErr != nil {
					return fmt.Errorf("volume_mount delete: %w", deleteErr) // 削除エラーを返す
				}
			} else { // pending/mounted は mounted に昇格する
				if updateErr := activities.volumeMountRepo.UpdateStatus(ctx, tx, volumeMountItem, models.VolumeMountStatusMounted); updateErr != nil { // status を mounted に変更する
					return fmt.Errorf("volume_mount update status: %w", updateErr) // 更新エラーを返す
				}
			}
		}

		applyResult = &ApplyResultData{ // 結果を設定する
			ApplyHistoryID:          applyHistoryRecord.ID,
			AppliedServiceID:        appliedServiceID,
			AppliedServiceNamespace: appliedServiceNamespace,
		}
		return nil // トランザクションをコミットする
	})

	// トランザクション成功後に k8s から ClusterIP を取得して DB に同期する
	if err == nil && appliedServiceID != "" && appliedServiceNamespace != "" {
		k8sServiceName := appliedServiceID + "-svc"                                                                                        // k8s Service 名を生成する
		k8sSvc, getErr := activities.k8sClient.CoreV1().Services(appliedServiceNamespace).Get(ctx, k8sServiceName, metav1.GetOptions{})    // k8s から Service を取得する
		if getErr == nil && k8sSvc.Spec.ClusterIP != "" {                                                                                  // ClusterIP が割り当て済みの場合のみ保存する
			if updateErr := activities.serviceRepo.UpdateClusterIP(ctx, appliedServiceID, k8sSvc.Spec.ClusterIP); updateErr != nil { // cluster_ip を DB に保存する
				logger.PrintErr("ExecuteApply: cluster_ip 同期に失敗しました: " + updateErr.Error()) // エラーをログ出力する（非致命的）
			}
		}
	}

	return applyResult, err // 結果とエラーを返す
}

// ApplyIngressRoutesInput は IngressRoute apply Activity の入力
type ApplyIngressRoutesInput struct {
	ProjectID  string // 対象プロジェクトのID
	BaseDomain string // ベースドメイン
}

// ApplyIngressRoutes は ProjectID に紐づく IngressRoute を k8s に apply する Activity
func (activities *ApplyActivities) ApplyIngressRoutes(ctx context.Context, input ApplyIngressRoutesInput) error {
	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, input.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	ingressRouteList, err := activities.ingressRouteRepo.FindAllByProjectID(ctx, input.ProjectID) // 全 IngressRoute を取得する
	if err != nil {
		return fmt.Errorf("ingress_route 一覧取得に失敗しました: %w", err) // 取得エラーを返す
	}

	for _, ingressRouteData := range ingressRouteList { // 各 IngressRoute に対して apply 処理を行う
		if applyErr := applySingleIngressRoute(ctx, activities.k8sClient, activities.dynamicClient, activities.pathRuleRepo, activities.serviceRepo, projectData.Namespace, ingressRouteData, input.BaseDomain); applyErr != nil { // 個別 IngressRoute を apply する
			logger.PrintErr("ApplyIngressRoutes: IngressRoute apply に失敗しました (ingressRouteID=" + ingressRouteData.ID + "): " + applyErr.Error()) // エラーをログ出力する
		}
	}
	return nil // 正常終了を返す
}

// applySingleIngressRoute は1つの IngressRoute を k8s に apply する
func applySingleIngressRoute(
	ctx context.Context,
	k8sClient k8sclient.Interface,
	dynamicClient dynamic.Interface,
	pathRuleRepo repository.PathRuleRepository,
	serviceRepo repository.ServiceRepository,
	namespace string,
	ingressRouteData *models.IngressRoute,
	baseDomain string,
) error {
	if ingressRouteData.Status == models.IngressRouteStatusDeleting { // deleting の場合は k8s から削除する
		if err := k8s.DeleteIngressRoute(ctx, dynamicClient, namespace, ingressRouteData.ID); err != nil { // IngressRoute を削除する
			return fmt.Errorf("k8s IngressRoute delete: %w", err) // 削除エラーを返す
		}
		if err := k8s.DeleteMiddleware(ctx, dynamicClient, namespace, ingressRouteData.ID); err != nil { // Middleware を削除する
			logger.PrintErr("applySingleIngressRoute: Middleware 削除に失敗しました: " + err.Error()) // エラーをログ出力する（非致命的）
		}
		return nil // 正常終了を返す
	}

	pathRuleList, err := pathRuleRepo.FindByIngressRouteID(ctx, ingressRouteData.ID) // PathRule 一覧を取得する
	if err != nil {
		return fmt.Errorf("path_rule 一覧取得に失敗しました: %w", err) // 取得エラーを返す
	}

	pathRuleSpecList := make([]k8s.PathRuleSpec, 0, len(pathRuleList)) // PathRuleSpec スライスを初期化する
	stripPrefixList := make([]string, 0)                               // StripPrefix 対象パスのリストを初期化する

	for _, pathRuleData := range pathRuleList { // 各 PathRule に対して PathRuleSpec を構築する
		serviceName := ""   // Service 名を初期化する
		servicePort := 0    // Service ポートを初期化する

		if pathRuleData.ServiceID != "" { // Service が設定されている場合は Service 名・ポートを取得する
			serviceData, serviceErr := serviceRepo.FindByServiceID(ctx, pathRuleData.ServiceID) // Service を取得する
			if serviceErr != nil {
				return fmt.Errorf("service 取得に失敗しました: %w", serviceErr) // 取得エラーを返す
			}
			serviceName = serviceData.ID + "-svc" // Service 名を Service ID ベースで生成する
			servicePort = serviceData.Port         // Service ポートを設定する
		}

		pathRuleSpec := k8s.PathRuleSpec{ // PathRuleSpec を構築する
			PathPrefix:  pathRuleData.PathPrefix,
			ServiceName: serviceName,
			ServicePort: servicePort,
			StripPrefix: pathRuleData.StripPrefix,
		}
		pathRuleSpecList = append(pathRuleSpecList, pathRuleSpec) // PathRuleSpec を追加する

		if pathRuleData.StripPrefix { // StripPrefix が有効な場合はプレフィックスを追加する
			stripPrefixList = append(stripPrefixList, pathRuleData.PathPrefix) // StripPrefix 対象パスを追加する
		}
	}

	if len(stripPrefixList) > 0 { // StripPrefix が存在する場合は Middleware を apply する
		if err := k8s.ApplyMiddleware(ctx, dynamicClient, ingressRouteData.ID, namespace, stripPrefixList); err != nil { // Middleware を apply する
			return fmt.Errorf("k8s Middleware apply: %w", err) // apply エラーを返す
		}
	}

	if err := k8s.ApplyIngressRoute(ctx, dynamicClient, *ingressRouteData, namespace, pathRuleSpecList); err != nil { // IngressRoute を apply する
		return fmt.Errorf("k8s IngressRoute apply: %w", err) // apply エラーを返す
	}

	return nil // 正常終了を返す
}
