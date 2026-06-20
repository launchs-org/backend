package service

import (
	"app/k8s"
	"app/k8s/manifest"
	"app/models"
	"app/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"
)

// ErrDuplicateEnvKey は apply 時に環境変数キーが重複している場合のエラー
var ErrDuplicateEnvKey = errors.New("duplicate env key: same key exists in env_var_mounts")

// ErrAlreadyApplying は apply 中の deployment に再 apply しようとした場合のエラー
var ErrAlreadyApplying = errors.New("already applying")

// ApplyServiceInterface は apply サービスのインターフェース
type ApplyServiceInterface interface {
	Apply(ctx context.Context, userID string, deploymentID string) (*ApplyResult, error)                          // apply を実行する
	ListApplyHistories(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error)   // apply 履歴一覧を取得する
}

// ApplyService は apply のコアロジックを実装するサービス
type ApplyService struct {
	DB                 *gorm.DB                            // データベース接続（トランザクション管理用）
	K8s                k8sclient.Interface                 // k8s クライアント
	DynamicClient      dynamic.Interface                   // dynamic クライアント（Traefik CRD 用）
	DeploymentRepo     repository.DeploymentRepository     // deployment リポジトリ
	ApplyHistoryRepo   repository.ApplyHistoryRepository   // apply_history リポジトリ
	ProjectRepository  repository.ProjectRepository        // project リポジトリ
	ServiceRepo        repository.ServiceRepository        // service リポジトリ
	IngressRouteRepo   repository.IngressRouteRepository   // ingress_route リポジトリ
	PathRuleRepo       repository.PathRuleRepository       // path_rule リポジトリ
	EnvVarRepo         repository.EnvVarRepository         // env_var リポジトリ
	EnvVarMountRepo    repository.EnvVarMountRepository    // env_var_mount リポジトリ
	VolumeRepo         repository.VolumeRepository         // volume リポジトリ
	VolumeMountRepo    repository.VolumeMountRepository    // volume_mount リポジトリ
	UserQuotaRepo      repository.UserQuotaRepository      // user_quota リポジトリ（Quotaチェック用）
}

// ApplyResult は Apply 処理の結果を表す構造体
type ApplyResult struct {
	ApplyHistoryID string // apply_history の ID
	Status         string // apply の結果ステータス
	BuildID        string // ビルドが必要な場合に設定（Phase8 で使用）
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
	envVarRepo repository.EnvVarRepository,
	envVarMountRepo repository.EnvVarMountRepository,
	volumeRepo repository.VolumeRepository,
	volumeMountRepo repository.VolumeMountRepository,
	userQuotaRepo repository.UserQuotaRepository,
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
		PathRuleRepo:      pathRuleRepo,    // path_rule リポジトリを注入する
		EnvVarRepo:        envVarRepo,
		EnvVarMountRepo:   envVarMountRepo,
		VolumeRepo:        volumeRepo,
		VolumeMountRepo:   volumeMountRepo,
		UserQuotaRepo:     userQuotaRepo,   // user_quota リポジトリを注入する
	}
}

// Apply は deployment に対して apply を実行する
func (applyService *ApplyService) Apply(ctx context.Context, userID string, deploymentID string) (*ApplyResult, error) {
	var applyResult *ApplyResult // 結果を格納する変数を定義する

	err := applyService.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		// 1. SELECT FOR UPDATE でロックを取得する
		deploymentData, err := applyService.DeploymentRepo.FindByIDForUpdate(ctx, tx, deploymentID) // FOR UPDATE ロック付きで deployment を取得する
		if err != nil {
			return fmt.Errorf("deployment not found: %w", err) // 取得エラーを返す
		}

		// 所有権を確認する（トランザクション外で取得）
		ownerProjectData, ownerErr := applyService.ProjectRepository.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
		if ownerErr != nil {
			return fmt.Errorf("project not found: %w", ownerErr) // 取得エラーを返す
		}
		if ownerProjectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
			return ErrForbidden
		}

		// apply 中の deployment への二重 apply を防ぐ
		if deploymentData.AppStatus == models.AppStatusDeploying { // 既に apply 中の場合は競合エラーを返す
			return ErrAlreadyApplying
		}

		// 削除中の deployment への apply を防ぐ
		if deploymentData.Status == models.DeploymentStatusDeleting { // 削除中の場合は競合エラーを返す
			return ErrAlreadyApplying
		}

		// 2. Project を取得する（namespace 解決のため）
		projectData, err := applyService.ProjectRepository.FindByID(ctx, tx, deploymentData.ProjectID) // ProjectRepository 経由で project を取得する
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

		if err := CheckReplicasQuota(ctx, applyService.UserQuotaRepo, userID, replicas); err != nil { // レプリカ数のQuotaチェックを行う
			return err // Quota超過エラーを返す
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
		var instanceSizeData models.InstanceSize                                                         // instance_size を格納する変数を定義する
		if err := tx.WithContext(ctx).First(&instanceSizeData, "size = ?", instanceSize).Error; err != nil { // instance_size マスターを取得する
			return fmt.Errorf("instance_size '%s' が instance_sizes テーブルに存在しません: %w", instanceSize, err) // レコードが見つからない場合はエラーを返す
		}

		deploymentForManifest := *deploymentData          // manifest 生成用にコピーする
		deploymentForManifest.InstanceSize = instanceSize // 実効 instance_size を設定する
		deploymentForManifest.Replicas = replicas         // 実効 replicas を設定する
		deploymentForManifest.Command = command           // 実効 command を設定する
		deploymentForManifest.Args = args                 // 実効 args を設定する

		// 5. EnvVarMount 一覧を取得して ConfigMap/Secret データを構築する
		envVarMountList, err := applyService.EnvVarMountRepo.FindAllByDeploymentID(ctx, deploymentID) // deployment に紐づくマウント設定一覧を取得する
		if err != nil {
			return fmt.Errorf("env_var_mount list: %w", err) // 取得エラーを返す
		}

		configMapData := map[string]string{}  // ConfigMap 用の非シークレット環境変数を格納するマップ
		secretData := map[string][]byte{}     // Secret 用のシークレット環境変数を格納するマップ
		keySet := map[string]bool{}           // キー名重複チェック用のセット
		var duplicateKeyErr error             // 重複キーエラーを一時保存する変数

		for _, mountItem := range envVarMountList { // マウント設定ごとに環境変数を解決する
			envVarData, envVarErr := applyService.EnvVarRepo.FindByID(ctx, mountItem.EnvVarID) // env_var を取得する
			if envVarErr != nil {
				return fmt.Errorf("env_var not found (id=%s): %w", mountItem.EnvVarID, envVarErr) // 取得エラーを返す
			}

			effectiveKey := envVarData.Key   // 実効キー名を決定する（デフォルトは元のキー）
			if mountItem.OverrideKey != "" { // override_key が設定されている場合はそちらを使う
				effectiveKey = mountItem.OverrideKey
			}

			if keySet[effectiveKey] { // キー名が重複している場合は後で failed にするためエラーを保存する
				duplicateKeyErr = fmt.Errorf("%w: key=%s", ErrDuplicateEnvKey, effectiveKey) // 重複エラーを保存する
				break                                                                         // ループを抜ける
			}
			keySet[effectiveKey] = true // キーをセットに追加する

			if envVarData.IsSecret { // is_secret が true の場合は Secret に分類する
				secretData[effectiveKey] = []byte(envVarData.Value) // Secret データに追加する
			} else {
				configMapData[effectiveKey] = envVarData.Value // ConfigMap データに追加する
			}
		}

		// 5-2. VolumeMount 一覧を取得して PVC マニフェストを準備する
		volumeMountList, volumeMountErr := applyService.VolumeMountRepo.FindAllByDeploymentID(ctx, deploymentID) // deployment に紐づく VolumeMount 一覧を取得する
		if volumeMountErr != nil {
			return fmt.Errorf("volume_mount list: %w", volumeMountErr) // 取得エラーを返す
		}

		volumeMountValues := make([]models.VolumeMount, len(volumeMountList)) // ポインタスライスを値スライスに変換する
		for mountIndex, mountPtr := range volumeMountList {                   // VolumeMount を値スライスに変換する
			volumeMountValues[mountIndex] = *mountPtr
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
		manifestJSON, _ := json.Marshal(deploymentManifest) // マニフェストを JSON にシリアライズする
		applyHistoryRecord := &models.ApplyHistory{         // apply_history レコードを生成する
			DeploymentID: deploymentID,
			Manifests:    manifestJSON,
			Status:       models.ApplyStatusApplied, // 初期ステータスは applied とする
			AppliedAt:    time.Now(),
		}
		if err := applyService.ApplyHistoryRepo.Create(ctx, tx, applyHistoryRecord); err != nil { // apply_history を作成する
			return fmt.Errorf("apply_history create: %w", err) // 作成エラーを返す
		}

		// 6-2. 重複キーが存在した場合は apply_history を failed にしてエラーを返す
		if duplicateKeyErr != nil { // 重複キーエラーが保存されている場合は処理する
			applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // ステータスを failed に変更する
			applyHistoryRecord.ErrorMessage = duplicateKeyErr.Error()                                                                             // エラーメッセージを記録する
			if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
				return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
			}
			return duplicateKeyErr // 重複キーエラーを返す
		}

		// 6-3. k8s に PVC を apply する（VolumeMount が存在する場合のみ）
		for _, volumeMountItem := range volumeMountList { // VolumeMount ごとに PVC を apply する
			volumeData, volumeErr := applyService.VolumeRepo.FindByID(ctx, volumeMountItem.VolumeID) // Volume を取得する
			if volumeErr != nil {
				applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // Volume 取得失敗時はステータスを failed に変更する
				applyHistoryRecord.ErrorMessage = volumeErr.Error()                                                                                   // エラーメッセージを記録する
				if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
					return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
				}
				return fmt.Errorf("volume not found (id=%s): %w", volumeMountItem.VolumeID, volumeErr) // Volume 取得エラーを返す
			}
			pvcName := volumeData.ID + "-pvc"                                                                                              // PVC 名を VolumeID から生成する（generator.go の命名規則と一致させる）
			pvcManifest := k8s.BuildPVCManifest(projectData.Namespace, pvcName, volumeData.SizeMB, "")                                    // PVC マニフェストを生成する
			if pvcErr := k8s.ApplyPVC(ctx, applyService.K8s, pvcManifest); pvcErr != nil {                                                // k8s に PVC を apply する
				applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // PVC apply 失敗時はステータスを failed に変更する
				applyHistoryRecord.ErrorMessage = pvcErr.Error()                                                                                      // エラーメッセージを記録する
				if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
					return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
				}
				return fmt.Errorf("k8s pvc apply (volume_id=%s): %w", volumeData.ID, pvcErr) // k8s PVC apply エラーを返す
			}
		}

		// 7-0. k8s に ConfigMap を apply する（非シークレット環境変数が存在する場合のみ）
		if len(configMapData) > 0 { // ConfigMap データが存在する場合のみ apply する
			if err := k8s.ApplyConfigMap(ctx, applyService.K8s, projectData.Namespace, deploymentData.Name, configMapData); err != nil { // k8s に ConfigMap を apply する
				applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // ConfigMap apply 失敗時はステータスを failed に変更する
				applyHistoryRecord.ErrorMessage = err.Error()                                                                                         // エラーメッセージを記録する
				if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
					return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
				}
				return fmt.Errorf("k8s configmap apply: %w", err) // k8s ConfigMap apply エラーを返す
			}
		}

		// 7-0-2. k8s に Secret を apply する（シークレット環境変数が存在する場合のみ）
		if len(secretData) > 0 { // Secret データが存在する場合のみ apply する
			if err := k8s.ApplySecret(ctx, applyService.K8s, projectData.Namespace, deploymentData.Name, secretData); err != nil { // k8s に Secret を apply する
				applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // Secret apply 失敗時はステータスを failed に変更する
				applyHistoryRecord.ErrorMessage = err.Error()                                                                                         // エラーメッセージを記録する
				if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
					return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
				}
				return fmt.Errorf("k8s secret apply: %w", err) // k8s Secret apply エラーを返す
			}
		}

		// 7. k8s に Deployment を apply する
		if err := k8s.ApplyDeployment(ctx, applyService.K8s, deploymentManifest); err != nil { // k8s に Deployment を apply する
			applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // k8s apply 失敗時はステータスを failed に変更する
			applyHistoryRecord.ErrorMessage = err.Error()                                                                                         // エラーメッセージを記録する
			if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
				return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
			}
			return fmt.Errorf("k8s deployment apply: %w", err) // k8s Deployment apply エラーを返す
		}

		// 7-2. k8s に Service を apply する（pending_port=0 の場合は k8s から削除する）
		var serviceData *models.Service                                                            // Service レコードを格納する変数を宣言する
		serviceData, _ = applyService.ServiceRepo.FindByDeploymentID(ctx, deploymentID)           // Service レコードを取得する（存在しない場合は nil）
		if serviceData != nil {
			pendingPort := serviceData.PendingPort // pending_port を取得する
			if pendingPort == 0 && serviceData.Port != 0 {
				// pending_port=0 かつ現在値が設定済みの場合は k8s から Service を削除する（無効化）
				k8sServiceName := deploymentData.Name + "-svc"
				if delErr := k8s.DeleteService(ctx, applyService.K8s, projectData.Namespace, k8sServiceName); delErr != nil { // k8s Service を削除する
					if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil {
						return fmt.Errorf("apply_history update: %w", updateErr)
					}
					return fmt.Errorf("k8s service delete: %w", delErr)
				}
			} else if pendingPort != 0 {
				// pending_port が設定されている場合は k8s に apply する
				serviceManifest := manifestGenerator.GenerateService(*serviceData, deploymentData.Name, projectData.Namespace) // Service マニフェストを生成する
				if err := k8s.ApplyService(ctx, applyService.K8s, serviceManifest); err != nil {                              // k8s に Service を apply する
					applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // k8s Service apply 失敗時はステータスを failed に変更する
					applyHistoryRecord.ErrorMessage = err.Error()                                                                                         // エラーメッセージを記録する
					if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
						return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
					}
					return fmt.Errorf("k8s service apply: %w", err) // k8s Service apply エラーを返す
				}
			}
		}

		// 7-3. k8s に IngressRoute を apply する（PathRule を集約して Traefik IngressRoute を再構築する）
		var ingressRouteData *models.IngressRoute                                                                       // IngressRoute レコードを格納する変数を宣言する
		ingressRouteData, _ = applyService.IngressRouteRepo.FindByProjectID(ctx, deploymentData.ProjectID)             // Project に紐づく IngressRoute を取得する（存在しない場合は nil）
		if ingressRouteData != nil {
			pathRuleList, pathRuleErr := applyService.PathRuleRepo.FindActiveAndPendingByIngressRouteID(ctx, ingressRouteData.ID) // active/pending の PathRule 一覧を取得する
			if pathRuleErr != nil {
				return fmt.Errorf("path_rule 一覧の取得に失敗しました: %w", pathRuleErr) // 取得エラーを返す
			}

			if len(pathRuleList) == 0 {
				// PathRule が 0 件の場合は k8s から IngressRoute を削除する
				if delErr := k8s.DeleteIngressRoute(ctx, applyService.DynamicClient, projectData.Namespace, ingressRouteData.ID); delErr != nil { // k8s IngressRoute を削除する
					if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil {
						return fmt.Errorf("apply_history update: %w", updateErr)
					}
					return fmt.Errorf("k8s ingress_route delete: %w", delErr)
				}
			} else {
				// PathRule を集約して Service 情報を解決する
				pathRuleSpecList := make([]k8s.PathRuleSpec, 0, len(pathRuleList)) // PathRuleSpec 一覧を初期化する
				for _, pathRuleItem := range pathRuleList {
					pathRuleService, serviceErr := applyService.ServiceRepo.FindByServiceID(ctx, pathRuleItem.ServiceID) // PathRule が指す Service を取得する
					if serviceErr != nil {
						return fmt.Errorf("path_rule の Service 取得に失敗しました (service_id=%s): %w", pathRuleItem.ServiceID, serviceErr) // 取得エラーを返す
					}
					pathRuleServicePort := pathRuleService.PendingPort // pending_port を使う
					if pathRuleServicePort == 0 {                      // pending が 0 の場合は current 値を使う
						pathRuleServicePort = pathRuleService.Port
					}
					pathRuleSpecList = append(pathRuleSpecList, k8s.PathRuleSpec{ // PathRuleSpec を追加する
						PathPrefix:  pathRuleItem.PathPrefix,                 // パスプレフィックスを設定する
						ServiceName: pathRuleService.DeploymentID + "-svc",  // Service 名を生成する（deployment名-svc の命名規則）
						ServicePort: pathRuleServicePort,                     // サービスポートを設定する
					})
				}

				if err := k8s.ApplyIngressRoute(ctx, applyService.DynamicClient, *ingressRouteData, projectData.Namespace, pathRuleSpecList); err != nil { // k8s に IngressRoute を apply する
					applyHistoryRecord.Status = models.ApplyStatusFailed                                                                                  // k8s IngressRoute apply 失敗時はステータスを failed に変更する
					applyHistoryRecord.ErrorMessage = err.Error()                                                                                         // エラーメッセージを記録する
					if updateErr := applyService.ApplyHistoryRepo.UpdateStatus(ctx, tx, applyHistoryRecord, models.ApplyStatusFailed); updateErr != nil { // ステータスを更新する
						return fmt.Errorf("apply_history update: %w", updateErr) // 更新エラーを返す
					}
					return fmt.Errorf("k8s ingress_route apply: %w", err) // k8s IngressRoute apply エラーを返す
				}
			}
		}

		// 8. pending_*** を空にして current 値に昇格させる
		appliedAt := time.Now() // apply 完了時刻を記録する
		updates := map[string]interface{}{
			"image_url":                     imageURL,                                  // image_url を昇格する
			"pending_image_url":             "",                                        // pending_image_url をクリアする
			"instance_size":                 instanceSize,                              // instance_size を昇格する
			"pending_instance_size":         "",                                        // pending_instance_size をクリアする
			"replicas":                      replicas,                                  // replicas を昇格する
			"pending_replicas":              0,                                         // pending_replicas をクリアする
			"github_repo_url":               deploymentData.PendingGithubRepoURL,       // github_repo_url を昇格する
			"pending_github_repo_url":       "",                                        // pending_github_repo_url をクリアする
			"github_branch":                 deploymentData.PendingGithubBranch,        // github_branch を昇格する
			"pending_github_branch":         "",                                        // pending_github_branch をクリアする
			"github_commit_sha":             deploymentData.PendingGithubCommitSHA,     // github_commit_sha を昇格する
			"pending_github_commit_sha":     "",                                        // pending_github_commit_sha をクリアする
			"github_repo_directory":         deploymentData.PendingGithubRepoDirectory, // github_repo_directory を昇格する
			"pending_github_repo_directory": "",                                        // pending_github_repo_directory をクリアする
			"dockerfile_path":               deploymentData.PendingDockerfilePath,      // dockerfile_path を昇格する
			"pending_dockerfile_path":       "",                                        // pending_dockerfile_path をクリアする
			"command":                       command,                                   // command を昇格する
			"pending_command":               nil,                                       // pending_command をクリアする
			"args":                          args,                                      // args を昇格する
			"pending_args":                  nil,                                       // pending_args をクリアする
			"status":                        models.DeploymentStatusRunning,            // status を running に更新する
			"app_status":                    models.AppStatusDeploying,                 // app_status を deploying に更新する
			"applied_at":                    &appliedAt,                                // applied_at を更新する
		}
		if err := applyService.DeploymentRepo.Updates(ctx, tx, deploymentData, updates); err != nil { // deployment を更新する
			return fmt.Errorf("deployment updates: %w", err) // 更新エラーを返す
		}

		// 9. Service の pending_*** を昇格させる
		if serviceData != nil { // Service レコードが存在する場合のみ昇格する
			serviceData.Port = serviceData.PendingPort             // pending_port を昇格する（0=無効化も含む）
			serviceData.PendingPort = 0                            // pending_port をクリアする
			serviceData.TargetPort = serviceData.PendingTargetPort // pending_target_port を昇格する
			serviceData.PendingTargetPort = 0                      // pending_target_port をクリアする
			if serviceData.Port == 0 {
				serviceData.Status = models.ServiceStatusPending // port=0 は未設定（pending）に戻す
			} else {
				serviceData.Status = models.ServiceStatusActive // port が設定されている場合は active にする
			}
			if err := applyService.ServiceRepo.Update(ctx, serviceData); err != nil { // Service を更新する
				return fmt.Errorf("service update: %w", err) // 更新エラーを返す
			}
		}

		// 10. PathRule の pending→active 昇格・deleting→物理削除を行う
		if ingressRouteData != nil { // IngressRoute レコードが存在する場合のみ処理する
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
		}

		// 11. VolumeMount の status を mounted に更新する
		for _, volumeMountItem := range volumeMountList { // VolumeMount ごとに status を更新する
			if updateErr := applyService.VolumeMountRepo.UpdateStatus(ctx, tx, volumeMountItem, models.VolumeMountStatusMounted); updateErr != nil { // status を mounted に変更する
				return fmt.Errorf("volume_mount update status: %w", updateErr) // 更新エラーを返す
			}
		}

		applyResult = &ApplyResult{ // 結果を設定する
			ApplyHistoryID: applyHistoryRecord.ID,
			Status:         "applied",
		}
		return nil // トランザクションをコミットする
	})

	return applyResult, err // 結果とエラーを返す
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
