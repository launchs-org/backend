package k8s

import (
	"app/logger"
	"app/models"
	"app/repository"
	"context"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"gorm.io/datatypes"
)

// ApplyDeployment は k8s に Deployment を作成または更新する
func ApplyDeployment(ctx context.Context, client kubernetes.Interface, deploymentManifest *appsv1.Deployment) error {
	existing, err := client.AppsV1().Deployments(deploymentManifest.Namespace).Get(ctx, deploymentManifest.Name, metav1.GetOptions{}) // 既存の Deployment を取得する
	if err != nil {
		// 存在しない場合は新規作成する
		_, err = client.AppsV1().Deployments(deploymentManifest.Namespace).Create(ctx, deploymentManifest, metav1.CreateOptions{})
		return err
	}
	// 既存の ResourceVersion を設定して更新する（k8s の楽観的並行性制御のため）
	deploymentManifest.ResourceVersion = existing.ResourceVersion
	_, err = client.AppsV1().Deployments(deploymentManifest.Namespace).Update(ctx, deploymentManifest, metav1.UpdateOptions{})
	return err
}

// DeleteDeployment は k8s から Deployment を削除する
func DeleteDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{}) // Deployment を削除する
}

// WatchDeployments は全 Namespace の Deployment 変化を監視して DB を自動更新する
func WatchDeployments(ctx context.Context, k8sClient kubernetes.Interface, deploymentRepo repository.DeploymentRepository, envVarMountRepo repository.EnvVarMountRepository, volumeMountRepo repository.VolumeMountRepository, applyHistoryRepo repository.ApplyHistoryRepository, buildRepo repository.DeploymentBuildRepository) {
	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, err := k8sClient.AppsV1().Deployments("").Watch(ctx, metav1.ListOptions{
			LabelSelector: "launchs.org/deployment-id", // launchs.org/deployment-id ラベルを持つ Deployment のみ監視する
		}) // Watch を開始する
		if err != nil {
			logger.PrintErr("WatchDeployments: Watch 開始に失敗しました: " + err.Error()) // エラーをログ出力する
			continue                                                                     // 再試行する
		}

		logger.Println("WatchDeployments: 監視を開始しました") // 監視開始ログを出力する

		watchLoop(ctx, watcher, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, buildRepo) // イベントループを実行する

		logger.Println("WatchDeployments: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// watchLoop は Watch イベントチャネルを処理するループ
func watchLoop(ctx context.Context, watcher watch.Interface, deploymentRepo repository.DeploymentRepository, envVarMountRepo repository.EnvVarMountRepository, volumeMountRepo repository.VolumeMountRepository, applyHistoryRepo repository.ApplyHistoryRepository, buildRepo repository.DeploymentBuildRepository) {
	defer watcher.Stop() // 終了時に Watch を停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case event, ok := <-watcher.ResultChan(): // イベントを受信する
			if !ok { // チャネルが閉じられた場合はループを抜ける
				return
			}
			handleDeploymentEvent(ctx, event, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, buildRepo) // イベントを処理する
		}
	}
}

// handleDeploymentEvent は Deployment の Watch イベントを処理する
func handleDeploymentEvent(ctx context.Context, event watch.Event, deploymentRepo repository.DeploymentRepository, envVarMountRepo repository.EnvVarMountRepository, volumeMountRepo repository.VolumeMountRepository, applyHistoryRepo repository.ApplyHistoryRepository, buildRepo repository.DeploymentBuildRepository) {
	k8sDeployment, ok := event.Object.(*appsv1.Deployment) // イベントオブジェクトを Deployment にキャストする
	if !ok {                                                 // キャストに失敗した場合はスキップする
		return
	}

	deploymentID, exists := k8sDeployment.Labels["launchs.org/deployment-id"] // deployment-id ラベルを取得する
	if !exists || deploymentID == "" {                                          // ラベルが存在しない場合はスキップする
		return
	}

	switch event.Type {
	case watch.Deleted: // Deleted イベントの場合は DB の status を確認して処理を分岐する
		deploymentData, err := deploymentRepo.FindByID(ctx, deploymentID) // DB から deployment を取得して status を確認する
		if err != nil {
			logger.PrintErr("WatchDeployments: Deployment 取得に失敗しました: " + err.Error()) // エラーをログ出力する
			return
		}

		if deploymentData.Status == models.DeploymentStatusDeleting { // status が deleting の場合のみ連鎖削除する
			// EnvVarMount を全件削除する
			envVarMountList, envVarMountErr := envVarMountRepo.FindAllByDeploymentID(ctx, deploymentID) // EnvVarMount 一覧を取得する
			if envVarMountErr == nil {
				for _, mountData := range envVarMountList { // 各マウント設定を削除する
					if err := envVarMountRepo.Delete(ctx, nil, mountData); err != nil { // EnvVarMount を削除する
						logger.PrintErr("WatchDeployments: EnvVarMount 削除に失敗しました: " + err.Error()) // エラーをログ出力する
					}
				}
			}

			// VolumeMount を全件削除する
			volumeMountList, volumeMountErr := volumeMountRepo.FindAllByDeploymentID(ctx, deploymentID) // VolumeMount 一覧を取得する
			if volumeMountErr == nil {
				for _, mountData := range volumeMountList { // 各マウント設定を削除する
					if err := volumeMountRepo.Delete(ctx, nil, mountData); err != nil { // VolumeMount を削除する
						logger.PrintErr("WatchDeployments: VolumeMount 削除に失敗しました: " + err.Error()) // エラーをログ出力する
					}
				}
			}

			// ApplyHistory を全件削除する
			if err := applyHistoryRepo.DeleteAllByDeploymentID(ctx, deploymentID); err != nil { // ApplyHistory を削除する
				logger.PrintErr("WatchDeployments: ApplyHistory 削除に失敗しました: " + err.Error()) // エラーをログ出力する
			}

			// DeploymentBuild を全件削除する
			if err := buildRepo.DeleteAllByDeploymentID(ctx, deploymentID); err != nil { // DeploymentBuild を削除する
				logger.PrintErr("WatchDeployments: DeploymentBuild 削除に失敗しました: " + err.Error()) // エラーをログ出力する
			}

			// Deployment レコード本体を削除する
			if err := deploymentRepo.Delete(ctx, deploymentID); err != nil { // deployment を削除する
				logger.PrintErr("WatchDeployments: Deployment 削除に失敗しました: " + err.Error()) // エラーをログ出力する
			}
			logger.Println("WatchDeployments: Deployment を削除しました: " + deploymentID) // 削除ログを出力する
		} else { // status が deleting 以外（意図しない削除）の場合は k8s_status を deleted に更新する
			deletedStatusJSON := datatypes.JSON([]byte(`{"deleted":true}`))                            // deleted 状態を表す JSON を生成する
			if err := deploymentRepo.UpdateK8sStatus(ctx, deploymentID, deletedStatusJSON); err != nil { // k8s_status を更新する
				logger.PrintErr("WatchDeployments: k8s_status 更新に失敗しました: " + err.Error()) // エラーをログ出力する
			}
			logger.Println("WatchDeployments: k8s_status を deleted に更新しました: " + deploymentID) // 更新ログを出力する
		}

	case watch.Added, watch.Modified: // Added/Modified イベントの場合は app_status と k8s_status を更新する
		appStatus := calcAppStatus(k8sDeployment) // app_status を計算する

		if err := deploymentRepo.UpdateAppStatus(ctx, deploymentID, appStatus); err != nil { // app_status を更新する
			logger.PrintErr("WatchDeployments: app_status 更新に失敗しました: " + err.Error()) // エラーをログ出力する
			return
		}

		k8sStatusJSON, err := marshalDeploymentStatus(k8sDeployment.Status) // DeploymentStatus を JSON にシリアライズする
		if err != nil {
			logger.PrintErr("WatchDeployments: k8s_status のシリアライズに失敗しました: " + err.Error()) // エラーをログ出力する
			return
		}

		if err := deploymentRepo.UpdateK8sStatus(ctx, deploymentID, k8sStatusJSON); err != nil { // k8s_status を更新する
			logger.PrintErr("WatchDeployments: k8s_status 更新に失敗しました: " + err.Error()) // エラーをログ出力する
			return
		}
		logger.Println("WatchDeployments: app_status と k8s_status を更新しました: " + deploymentID) // 更新ログを出力する
	}
}

// marshalDeploymentStatus は appsv1.DeploymentStatus を datatypes.JSON にシリアライズする
func marshalDeploymentStatus(status appsv1.DeploymentStatus) (datatypes.JSON, error) {
	statusBytes, err := json.Marshal(status) // DeploymentStatus を JSON バイト列に変換する
	if err != nil {
		return nil, err // シリアライズエラーを返す
	}
	return datatypes.JSON(statusBytes), nil // datatypes.JSON に変換して返す
}

// calcAppStatus は k8s Deployment の状態から AppStatus を計算する
func calcAppStatus(k8sDeployment *appsv1.Deployment) models.AppStatus {
	desiredReplicas := k8sDeployment.Spec.Replicas // 希望レプリカ数を取得する
	if desiredReplicas == nil {                     // nil の場合はデフォルト 1 とみなす
		defaultReplicas := int32(1)
		desiredReplicas = &defaultReplicas
	}

	readyReplicas := k8sDeployment.Status.ReadyReplicas // Ready なレプリカ数を取得する

	if readyReplicas >= *desiredReplicas && *desiredReplicas > 0 { // 全レプリカが Ready の場合
		return models.AppStatusRunning // running を返す
	}
	return models.AppStatusDeploying // 未達の場合は deploying を返す
}
