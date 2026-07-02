package k8s

import (
	"app/shared/logger"
	"app/shared/models"
	"app/shared/repository"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// ExistsDeployment は namespace/name に対応する k8s Deployment が存在するかどうかを返す
func ExistsDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string) (bool, error) {
	_, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{}) // k8s から Deployment を取得する
	if err != nil {
		if isNotFound(err) { // 404 の場合は存在しないと判定する
			return false, nil
		}
		return false, err // その他のエラーは伝播する
	}
	return true, nil // 存在する場合は true を返す
}

// isNotFound は k8s の 404 エラーかどうかを判定する
func isNotFound(err error) bool {
	if err == nil { // nil の場合は false を返す
		return false
	}
	return strings.Contains(err.Error(), "not found") // エラーメッセージで判定する
}

// pollDeployments は 10 秒ごとに全 Deployment を List して app_status を同期する
// Watch 接続切断中のイベント取りこぼしを補完するためのポーリングループ
func pollDeployments(ctx context.Context, k8sClient kubernetes.Interface, deploymentRepo repository.DeploymentRepository, podLogChunkRepo repository.PodLogChunkRepository, projectRepo repository.ProjectRepository, streamCancelMap map[string]podStreamState, streamCancelMu *sync.Mutex) {
	ticker := time.NewTicker(10 * time.Second) // 10 秒ごとにポーリングするタイマーを生成する
	defer ticker.Stop()                        // 終了時にタイマーを停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case <-ticker.C: // 10 秒ごとにポーリングを実行する
			logger.Println("pollDeployments: ポーリング開始") // ポーリング開始ログを出力する

			deploymentList, err := k8sClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
				LabelSelector: "launchs.org/deployment-id", // launchs.org/deployment-id ラベルを持つ Deployment のみ取得する
			}) // Deployment 一覧を取得する
			if err != nil {
				logger.PrintErr("pollDeployments: Deployment 一覧取得に失敗しました: " + err.Error()) // エラーをログ出力する
				continue                                                                        // 次のポーリングまで待機する
			}

			logger.Println("pollDeployments: " + strconv.Itoa(len(deploymentList.Items)) + " 件の Deployment を確認します") // 件数ログを出力する

			for pollIndex := range deploymentList.Items { // 各 Deployment を確認する
				k8sDeployment := &deploymentList.Items[pollIndex]                        // ポインタを取得する
				deploymentID := k8sDeployment.Labels["launchs.org/deployment-id"]       // deployment-id ラベルを取得する
				if deploymentID == "" {                                                  // ラベルが存在しない場合はスキップする
					continue
				}

				newAppStatus := calcAppStatus(k8sDeployment) // 現在の k8s 状態から app_status を計算する

				currentDeployment, err := deploymentRepo.FindByID(ctx, deploymentID) // DB の現在値を取得する
				if err != nil {
					logger.PrintErr("pollDeployments: Deployment 取得に失敗しました（deploymentID=" + deploymentID + "）: " + err.Error()) // エラーをログ出力する
					continue                                                                                                          // 次の Deployment に進む
				}

				if currentDeployment.AppStatus != newAppStatus { // app_status に変化がある場合のみ更新する
					logger.Println("pollDeployments: app_status 変化検出 deploymentID=" + deploymentID + " " + string(currentDeployment.AppStatus) + " → " + string(newAppStatus)) // 変化ログを出力する

					if err := deploymentRepo.UpdateAppStatus(ctx, deploymentID, newAppStatus); err != nil { // app_status を更新する
						logger.PrintErr("pollDeployments: app_status 更新に失敗しました（deploymentID=" + deploymentID + "）: " + err.Error()) // エラーをログ出力する
						continue                                                                                                            // 次の Deployment に進む
					}

					k8sStatusJSON, marshalErr := marshalDeploymentStatus(k8sDeployment.Status) // k8s_status をシリアライズする
					if marshalErr != nil {
						logger.PrintErr("pollDeployments: k8s_status シリアライズに失敗しました（deploymentID=" + deploymentID + "）: " + marshalErr.Error()) // エラーをログ出力する
					} else if err := deploymentRepo.UpdateK8sStatus(ctx, deploymentID, k8sStatusJSON); err != nil { // k8s_status を更新する
						logger.PrintErr("pollDeployments: k8s_status 更新に失敗しました（deploymentID=" + deploymentID + "）: " + err.Error()) // エラーをログ出力する
					}
				} else {
					logger.Println("pollDeployments: 変化なし deploymentID=" + deploymentID + " app_status=" + string(newAppStatus)) // 変化なしログを出力する
				}

				if newAppStatus == models.AppStatusRunning { // app_status が running の場合はログストリームを開始する
					currentReadyReplicas := k8sDeployment.Status.ReadyReplicas // 現在の ReadyReplicas を取得する
					streamCancelMu.Lock()                                       // マップへの排他アクセスを開始する
					existingState, streamRunning := streamCancelMap[deploymentID] // ストリームが既に実行中か確認する
					replicasChanged := streamRunning && existingState.readyReplicas != currentReadyReplicas // レプリカ数が変化したか確認する
					streamCancelMu.Unlock()                                     // マップへの排他アクセスを終了する

					if !streamRunning || replicasChanged { // ストリームが未起動またはレプリカ数変化時に開始する
						if replicasChanged { // レプリカ数変化の場合は既存ストリームをキャンセルする
							streamCancelMu.Lock()                                    // マップへの排他アクセスを開始する
							existingState.cancel()                                   // 既存ストリームをキャンセルする
							delete(streamCancelMap, deploymentID)                    // マップから削除する
							streamCancelMu.Unlock()                                  // マップへの排他アクセスを終了する
							logger.Println("pollDeployments: レプリカ数変化によりPodログストリームを再起動します: " + deploymentID) // 再起動ログを出力する
						}
						projectData, projectErr := projectRepo.FindByIDNoTx(ctx, currentDeployment.ProjectID) // project を取得して namespace を解決する
						if projectErr != nil {
							logger.PrintErr("pollDeployments: Project 取得に失敗しました（deploymentID=" + deploymentID + "）: " + projectErr.Error()) // エラーをログ出力する
							continue                                                                                                                // 次の Deployment に進む
						}
						streamCtx, streamCancel := context.WithCancel(ctx)      // ストリームのコンテキストを生成する
						streamCancelMu.Lock()                                    // マップへの排他アクセスを開始する
						streamCancelMap[deploymentID] = podStreamState{cancel: streamCancel, readyReplicas: currentReadyReplicas} // 状態をマップに登録する
						streamCancelMu.Unlock()                                  // マップへの排他アクセスを終了する
						go streamAndSavePodLogs(streamCtx, k8sClient, podLogChunkRepo, deploymentID, projectData.Namespace, k8sDeployment.Name) // ログストリームを goroutine で開始する
						logger.Println("pollDeployments: Podログストリームを開始しました: " + deploymentID)                                               // 開始ログを出力する
					}
				} else { // running 以外の場合は実行中のログストリームをキャンセルする
					streamCancelMu.Lock()                                               // マップへの排他アクセスを開始する
					if existingState, streamExists := streamCancelMap[deploymentID]; streamExists { // ストリームが実行中の場合
						existingState.cancel()                // ストリームをキャンセルする
						delete(streamCancelMap, deploymentID) // マップから削除する
						logger.Println("pollDeployments: Podログストリームを停止しました: " + deploymentID) // 停止ログを出力する
					}
					streamCancelMu.Unlock() // マップへの排他アクセスを終了する
				}
			}

			logger.Println("pollDeployments: ポーリング完了") // ポーリング完了ログを出力する
		}
	}
}

// podStreamState はログストリームのキャンセル関数と起動時のレプリカ数を保持する
type podStreamState struct {
	cancel        context.CancelFunc // ストリームキャンセル関数
	readyReplicas int32              // ストリーム起動時の ReadyReplicas 数
}

// WatchDeployments は全 Namespace の Deployment 変化を監視して DB を自動更新する
func WatchDeployments(ctx context.Context, k8sClient kubernetes.Interface, deploymentRepo repository.DeploymentRepository, envVarMountRepo repository.EnvVarMountRepository, volumeMountRepo repository.VolumeMountRepository, applyHistoryRepo repository.ApplyHistoryRepository, podLogChunkRepo repository.PodLogChunkRepository, projectRepo repository.ProjectRepository) {
	// 実行中のログストリームを管理するマップ（deploymentID → podStreamState）
	streamCancelMap := make(map[string]podStreamState) // ストリーム状態を管理するマップ
	var streamCancelMu sync.Mutex                      // マップへの排他アクセスのためのミューテックス

	// 起動時リカバリ: running 状態の Deployment に対してログストリームを再開する
	runningList, recoveryErr := deploymentRepo.FindAllRunning(ctx) // running 状態の deployment を全件取得する
	if recoveryErr != nil {
		logger.PrintErr("WatchDeployments: running Deployment の取得に失敗しました: " + recoveryErr.Error()) // エラーをログ出力する
	} else {
		for deploymentIndex := range runningList { // 各 Deployment に対してログストリームを再開する
			deploymentData := &runningList[deploymentIndex]                                            // ポインタを取得する
			projectData, projectErr := projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID)        // プロジェクトを取得して namespace を解決する
			if projectErr != nil {
				logger.PrintErr("WatchDeployments: namespace 解決に失敗しました（deploymentID=" + deploymentData.ID + "）: " + projectErr.Error()) // エラーをログ出力する
				continue                                                                                                                        // 次の Deployment に進む
			}
			streamCtx, streamCancel := context.WithCancel(ctx)      // ストリームのコンテキストを生成する
			streamCancelMu.Lock()                                    // マップへの排他アクセスを開始する
			streamCancelMap[deploymentData.ID] = podStreamState{cancel: streamCancel, readyReplicas: 0} // 状態をマップに登録する（リカバリ時は ReadyReplicas 不明なので 0）
			streamCancelMu.Unlock()                                  // マップへの排他アクセスを終了する
			go streamAndSavePodLogs(streamCtx, k8sClient, podLogChunkRepo, deploymentData.ID, projectData.Namespace, deploymentData.Name) // ログストリームを goroutine で再開する
			logger.Println("WatchDeployments: 起動時リカバリでログストリームを再開しました: " + deploymentData.ID)                                            // リカバリログを出力する
		}
	}

	go pollDeployments(ctx, k8sClient, deploymentRepo, podLogChunkRepo, projectRepo, streamCancelMap, &streamCancelMu) // 定期ポーリングを goroutine で起動する
	go watchPodDeletions(ctx, k8sClient, podLogChunkRepo)                                                              // Pod 削除イベントを監視する goroutine を起動する

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

		watchLoop(ctx, watcher, k8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo, streamCancelMap, &streamCancelMu) // イベントループを実行する

		logger.Println("WatchDeployments: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// watchLoop は Watch イベントチャネルを処理するループ
func watchLoop(ctx context.Context, watcher watch.Interface, k8sClient kubernetes.Interface, deploymentRepo repository.DeploymentRepository, envVarMountRepo repository.EnvVarMountRepository, volumeMountRepo repository.VolumeMountRepository, applyHistoryRepo repository.ApplyHistoryRepository, podLogChunkRepo repository.PodLogChunkRepository, projectRepo repository.ProjectRepository, streamCancelMap map[string]podStreamState, streamCancelMu *sync.Mutex) {
	defer watcher.Stop() // 終了時に Watch を停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case event, ok := <-watcher.ResultChan(): // イベントを受信する
			if !ok { // チャネルが閉じられた場合はループを抜ける
				return
			}
			handleDeploymentEvent(ctx, event, k8sClient, deploymentRepo, envVarMountRepo, volumeMountRepo, applyHistoryRepo, podLogChunkRepo, projectRepo, streamCancelMap, streamCancelMu) // イベントを処理する
		}
	}
}

// handleDeploymentEvent は Deployment の Watch イベントを処理する
func handleDeploymentEvent(ctx context.Context, event watch.Event, k8sClient kubernetes.Interface, deploymentRepo repository.DeploymentRepository, envVarMountRepo repository.EnvVarMountRepository, volumeMountRepo repository.VolumeMountRepository, applyHistoryRepo repository.ApplyHistoryRepository, podLogChunkRepo repository.PodLogChunkRepository, projectRepo repository.ProjectRepository, streamCancelMap map[string]podStreamState, streamCancelMu *sync.Mutex) {
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
		if k8sDeployment.DeletionTimestamp != nil { // DeletionTimestamp が残っている場合はまだ Terminating 中なのでスキップする
			return
		}

		// 実行中のログストリームをキャンセルする
		streamCancelMu.Lock()                              // マップへの排他アクセスを開始する
		if existingState, streamExists := streamCancelMap[deploymentID]; streamExists { // ストリームが実行中の場合
			existingState.cancel()                // ストリームをキャンセルする
			delete(streamCancelMap, deploymentID) // マップから削除する
		}
		streamCancelMu.Unlock() // マップへの排他アクセスを終了する

		deploymentData, err := deploymentRepo.FindByID(ctx, deploymentID) // DB から deployment を取得して status を確認する
		if err != nil {
			logger.PrintErr("WatchDeployments: Deployment 取得に失敗しました: " + err.Error()) // エラーをログ出力する
			return
		}

		if deploymentData.Status == models.DeploymentStatusDeleting { // status が deleting の場合のみ連鎖削除する
			// EnvVarMount を全件削除する
			_ = deploymentRepo.UpdateDeleteProgress(ctx, deploymentID, "環境変数マウントを削除中")         // 進捗を記録する
			envVarMountList, envVarMountErr := envVarMountRepo.FindAllByDeploymentID(ctx, deploymentID) // EnvVarMount 一覧を取得する
			if envVarMountErr == nil {
				for _, mountData := range envVarMountList { // 各マウント設定を削除する
					if err := envVarMountRepo.Delete(ctx, nil, mountData); err != nil { // EnvVarMount を削除する
						logger.PrintErr("WatchDeployments: EnvVarMount 削除に失敗しました: " + err.Error()) // エラーをログ出力する
					}
				}
			}

			// VolumeMount を全件削除する
			_ = deploymentRepo.UpdateDeleteProgress(ctx, deploymentID, "ボリュームマウントを削除中")          // 進捗を記録する
			volumeMountList, volumeMountErr := volumeMountRepo.FindAllByDeploymentID(ctx, deploymentID) // VolumeMount 一覧を取得する
			if volumeMountErr == nil {
				for _, mountData := range volumeMountList { // 各マウント設定を削除する
					if err := volumeMountRepo.Delete(ctx, nil, mountData); err != nil { // VolumeMount を削除する
						logger.PrintErr("WatchDeployments: VolumeMount 削除に失敗しました: " + err.Error()) // エラーをログ出力する
					}
				}
			}

			// ApplyHistory を全件削除する
			_ = deploymentRepo.UpdateDeleteProgress(ctx, deploymentID, "Apply履歴を削除中") // 進捗を記録する
			if err := applyHistoryRepo.DeleteAllByDeploymentID(ctx, deploymentID); err != nil { // ApplyHistory を削除する
				logger.PrintErr("WatchDeployments: ApplyHistory 削除に失敗しました: " + err.Error()) // エラーをログ出力する
			}

			// Deployment レコード本体を削除する
			_ = deploymentRepo.UpdateDeleteProgress(ctx, deploymentID, "レコードを削除中") // 進捗を記録する
			if err := deploymentRepo.Delete(ctx, deploymentID); err != nil {            // deployment を削除する
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

		if appStatus == models.AppStatusRunning { // app_status が running の場合はログストリームを開始または継続する
			currentReadyReplicas := k8sDeployment.Status.ReadyReplicas // 現在の ReadyReplicas を取得する
			streamCancelMu.Lock()                                       // マップへの排他アクセスを開始する
			existingState, streamRunning := streamCancelMap[deploymentID] // ストリームが既に実行中か確認する
			replicasChanged := streamRunning && existingState.readyReplicas != currentReadyReplicas // レプリカ数が変化したか確認する
			streamCancelMu.Unlock()                                     // マップへの排他アクセスを終了する

			if !streamRunning || replicasChanged { // ストリームが未起動またはレプリカ数変化時に開始する
				if replicasChanged { // レプリカ数変化の場合は既存ストリームをキャンセルする
					streamCancelMu.Lock()                                    // マップへの排他アクセスを開始する
					existingState.cancel()                                   // 既存ストリームをキャンセルする
					delete(streamCancelMap, deploymentID)                    // マップから削除する
					streamCancelMu.Unlock()                                  // マップへの排他アクセスを終了する
					logger.Println("WatchDeployments: レプリカ数変化によりPodログストリームを再起動します: " + deploymentID) // 再起動ログを出力する
				}
				deploymentData, deploymentErr := deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得して projectID を解決する
				if deploymentErr != nil {
					logger.PrintErr("WatchDeployments: Deployment 取得に失敗しました（deploymentID=" + deploymentID + "）: " + deploymentErr.Error()) // エラーをログ出力する
					return
				}
				projectData, projectErr := projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得して namespace を解決する
				if projectErr != nil {
					logger.PrintErr("WatchDeployments: Project 取得に失敗しました（deploymentID=" + deploymentID + "）: " + projectErr.Error()) // エラーをログ出力する
					return
				}
				streamCtx, streamCancel := context.WithCancel(ctx)      // ストリームのコンテキストを生成する
				streamCancelMu.Lock()                                    // マップへの排他アクセスを開始する
				streamCancelMap[deploymentID] = podStreamState{cancel: streamCancel, readyReplicas: currentReadyReplicas} // 状態をマップに登録する
				streamCancelMu.Unlock()                                  // マップへの排他アクセスを終了する
				go streamAndSavePodLogs(streamCtx, k8sClient, podLogChunkRepo, deploymentID, projectData.Namespace, k8sDeployment.Name) // ログストリームを goroutine で開始する
				logger.Println("WatchDeployments: Podログストリームを開始しました: " + deploymentID)                                               // 開始ログを出力する
			}
		} else { // running 以外の場合は実行中のログストリームをキャンセルする
			streamCancelMu.Lock()                                               // マップへの排他アクセスを開始する
			if existingState, streamExists := streamCancelMap[deploymentID]; streamExists { // ストリームが実行中の場合
				existingState.cancel()                // ストリームをキャンセルする
				delete(streamCancelMap, deploymentID) // マップから削除する
				logger.Println("WatchDeployments: Podログストリームを停止しました: " + deploymentID) // 停止ログを出力する
			}
			streamCancelMu.Unlock() // マップへの排他アクセスを終了する
		}
	}
}

// streamAndSavePodLogs は app=deploymentName ラベルの Pod を取得し、Pod ごとに goroutine でログを収集・保存する
func streamAndSavePodLogs(ctx context.Context, k8sClient kubernetes.Interface, podLogChunkRepo repository.PodLogChunkRepository, deploymentID, namespace, deploymentName string) {
	podList, err := waitForDeploymentPod(ctx, k8sClient, namespace, deploymentName) // Pod が起動するまで待機する
	if err != nil {
		if ctx.Err() == nil { // コンテキストキャンセル以外のエラーをログ出力する
			logger.PrintErr("WatchDeployments: Pod 取得に失敗しました（deploymentName=" + deploymentName + "）: " + err.Error()) // エラーをログ出力する
		}
		return
	}

	// 現在存在する Pod 名一覧を収集する
	activePodNames := make([]string, 0, len(podList)) // アクティブな Pod 名一覧を生成する
	for podIndex := range podList {                    // 各 Pod 名を収集する
		activePodNames = append(activePodNames, podList[podIndex].Name) // Pod 名を追加する
	}

	// スケールダウンで消えた Pod のチャンクを DB から削除する
	if len(activePodNames) > 0 { // Pod が1件以上ある場合のみ削除する（全削除を防ぐ）
		if deleteErr := podLogChunkRepo.DeleteByDeploymentIDAndPodNameNotIn(ctx, deploymentID, activePodNames); deleteErr != nil { // 不要チャンクを削除する
			logger.PrintErr("WatchDeployments: 不要 Pod ログチャンク削除に失敗しました（deploymentID=" + deploymentID + "）: " + deleteErr.Error()) // エラーをログ出力する
		}
	}

	for podIndex := range podList { // 各 Pod ごとにログ収集 goroutine を起動する
		pod := &podList[podIndex]
		go streamAndSaveSinglePodLogs(ctx, k8sClient, podLogChunkRepo, deploymentID, namespace, pod.Name) // Pod 単体のログ収集を goroutine で開始する
	}
}

// streamAndSaveSinglePodLogs は指定した1つの Pod の全コンテナのログをリアルタイムで取得し、chunk 単位で DB に保存する
func streamAndSaveSinglePodLogs(ctx context.Context, k8sClient kubernetes.Interface, podLogChunkRepo repository.PodLogChunkRepository, deploymentID, namespace, podName string) {
	logCh := collectSinglePodLogs(ctx, k8sClient, namespace, podName) // 1 Pod のログをチャンネルで取得する

	ticker := time.NewTicker(3 * time.Second) // 3秒ごとにバッファをフラッシュするタイマーを生成する
	defer ticker.Stop()                       // 終了時にタイマーを停止する

	var buf strings.Builder // ログバッファを生成する

	flush := func() { // バッファをフラッシュして DB に保存する関数
		if buf.Len() == 0 { // バッファが空の場合はスキップする
			return
		}
		chunk := &models.PodLogChunk{ // ログチャンクレコードを生成する
			DeploymentID: deploymentID, // デプロイメントIDを設定する
			PodName:      podName,      // Pod 名を設定する
			Content:      buf.String(), // バッファの内容を設定する
		}
		if err := podLogChunkRepo.Create(ctx, chunk); err != nil { // DB に保存する
			logger.PrintErr("WatchDeployments: Podログチャンク保存に失敗しました（deploymentID=" + deploymentID + ", pod=" + podName + "）: " + err.Error()) // エラーをログ出力する
		}
		buf.Reset() // バッファをリセットする
	}

	for {
		select {
		case line, ok := <-logCh: // ログ行を受信する
			if !ok { // チャネルが閉じられた場合はフラッシュして終了する
				flush()
				return
			}
			buf.WriteString(line + "\n") // ログ行をバッファに追記する
			if buf.Len() > 4096 {        // バッファが 4096 バイトを超えたら即時フラッシュする
				flush()
			}
		case <-ticker.C: // 3秒タイマーが発火したらフラッシュする
			flush()
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		}
	}
}

// collectSinglePodLogs は指定した Pod の全コンテナのログをチャンネルで返す
func collectSinglePodLogs(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName string) <-chan string {
	logCh := make(chan string, 100) // ログ行を送るチャンネルを生成する

	go func() {
		defer close(logCh) // 終了時にチャンネルをクローズする

		podData, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{}) // Pod の詳細を取得する
		if err != nil {
			if ctx.Err() == nil { // コンテキストキャンセル以外のエラーをログ出力する
				logger.PrintErr("WatchDeployments: Pod 詳細取得に失敗しました（pod=" + podName + "）: " + err.Error()) // エラーをログ出力する
			}
			return
		}

		containerNames := make([]string, 0) // コンテナ名一覧を初期化する
		for containerIndex := range podData.Spec.Containers {
			containerNames = append(containerNames, podData.Spec.Containers[containerIndex].Name) // container 名を追加する
		}

		for _, containerName := range containerNames { // 各コンテナのログをストリームする
			restartCount := 0 // 再起動回数カウンタを初期化する
			for {             // コンテナが再起動するたびにループする
				if restartCount > 0 { // 2回目以降はセパレーター行を挿入する
					separator := fmt.Sprintf("──── コンテナ再起動 #%d (%s) ────", restartCount, containerName) // 再起動セパレーター行を生成する
					select {
					case logCh <- separator: // セパレーターをチャンネルへ送信する
					case <-ctx.Done():       // コンテキストキャンセル時は終了する
						return
					}
				}

				if err := streamPodContainerLog(ctx, k8sClient, namespace, podName, containerName, logCh); err != nil { // コンテナのログをストリームする
					if ctx.Err() != nil { // コンテキストキャンセルの場合はループを抜ける
						return
					}
					logger.PrintErr("WatchDeployments: コンテナログ取得に失敗しました（pod=" + podName + ", container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
				}

				if ctx.Err() != nil { // コンテキストキャンセルの場合はループを抜ける
					return
				}

				// ストリームが終了した = コンテナが再起動した可能性があるので少し待ってから再接続する
				select {
				case <-ctx.Done(): // コンテキストキャンセル時は終了する
					return
				case <-time.After(2 * time.Second): // 2秒待機してから再接続する
				}

				// 現在のコンテナの再起動回数を確認して実際に再起動したかどうかを判定する
				podData, podErr := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{}) // Pod の詳細を再取得する
				if podErr != nil {
					if ctx.Err() == nil { // コンテキストキャンセル以外はログ出力する
						logger.PrintErr("WatchDeployments: Pod 再取得に失敗しました（pod=" + podName + "）: " + podErr.Error()) // エラーをログ出力する
					}
					return
				}

				currentRestartCount := int32(0) // 現在の再起動回数を初期化する
				for _, containerStatus := range podData.Status.ContainerStatuses { // コンテナステータスを確認する
					if containerStatus.Name == containerName { // 対象コンテナを見つける
						currentRestartCount = containerStatus.RestartCount // 再起動回数を取得する
						break
					}
				}

				if int(currentRestartCount) <= restartCount { // 再起動回数が増えていない場合はループを終了する
					break
				}
				restartCount = int(currentRestartCount) // 再起動回数を更新する
			}
		}
	}()

	return logCh // チャンネルを返す
}

// waitForDeploymentPod は app=deploymentName ラベルの Pod が少なくとも1つ起動するまで最大 60 秒待機して返す
func waitForDeploymentPod(ctx context.Context, k8sClient kubernetes.Interface, namespace, deploymentName string) ([]corev1.Pod, error) {
	labelSelector := "app=" + deploymentName // app ラベルでフィルタするセレクタを生成する
	for retryIndex := range make([]struct{}, 60) { // 最大 60 回リトライする
		_ = retryIndex
		podList, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector, // app ラベルで Pod を絞り込む
		}) // Pod 一覧を取得する
		if err == nil && len(podList.Items) > 0 { // Pod が1つ以上存在する場合は返す
			return podList.Items, nil
		}
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return nil, ctx.Err()
		case <-time.After(time.Second): // 1秒待機してリトライする
		}
	}
	return nil, nil // タイムアウトしても nil を返して呼び出し元に任せる（Pod なしは正常系として扱う）
}

// streamPodContainerLog は指定した Pod/コンテナのログを Follow で読み取り、logCh へ送信する
func streamPodContainerLog(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName, containerName string, logCh chan<- string) error {
	logOptions := &corev1.PodLogOptions{
		Container: containerName, // 対象コンテナ名を設定する
		Follow:    true,          // ログをリアルタイムで追従する
	}
	req := k8sClient.CoreV1().Pods(namespace).GetLogs(podName, logOptions) // ログ取得リクエストを生成する
	stream, err := req.Stream(ctx)                                          // ログストリームを開始する
	if err != nil {
		return err // ストリーム開始エラーを返す
	}
	defer stream.Close() // 終了時にストリームをクローズする

	scanner := bufio.NewScanner(stream) // スキャナを生成する
	for scanner.Scan() {                // ログを1行ずつ読み取る
		line := scanner.Text() // ログ行を取得する
		select {
		case logCh <- line: // ログ行をチャンネルへ送信する
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return ctx.Err()
		}
	}
	return nil // 正常終了を返す
}

// marshalDeploymentStatus は appsv1.DeploymentStatus を datatypes.JSON にシリアライズする
func marshalDeploymentStatus(status appsv1.DeploymentStatus) (datatypes.JSON, error) {
	statusBytes, err := json.Marshal(status) // DeploymentStatus を JSON バイト列に変換する
	if err != nil {
		return nil, err // シリアライズエラーを返す
	}
	return datatypes.JSON(statusBytes), nil // datatypes.JSON に変換して返す
}

// watchPodDeletions は全 Namespace の launchs.org/deployment-id ラベルを持つ Pod の Deleted イベントを監視し
// 削除された Pod のログチャンクを DB から削除する
func watchPodDeletions(ctx context.Context, k8sClient kubernetes.Interface, podLogChunkRepo repository.PodLogChunkRepository) {
	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, err := k8sClient.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
			LabelSelector: "launchs.org/deployment-id", // launchs.org/deployment-id ラベルを持つ Pod のみ監視する
		}) // Pod Watch を開始する
		if err != nil {
			logger.PrintErr("watchPodDeletions: Watch 開始に失敗しました: " + err.Error()) // エラーをログ出力する
			select {
			case <-ctx.Done(): // コンテキストキャンセル時は終了する
				return
			case <-time.After(5 * time.Second): // 5秒待機してリトライする
				continue
			}
		}

		for event := range watcher.ResultChan() { // イベントを受信するループ
			if event.Type != watch.Deleted { // Deleted 以外は無視する
				continue
			}
			pod, ok := event.Object.(*corev1.Pod) // イベントオブジェクトを Pod にキャストする
			if !ok {                              // キャストに失敗した場合はスキップする
				continue
			}
			deploymentID := pod.Labels["launchs.org/deployment-id"] // deployment-id ラベルを取得する
			if deploymentID == "" {                                  // ラベルが存在しない場合はスキップする
				continue
			}
			podName := pod.Name // 削除された Pod 名を取得する
			logger.Println("watchPodDeletions: Pod 削除を検知しました（pod=" + podName + ", deploymentID=" + deploymentID + "）") // 検知ログを出力する

			if err := podLogChunkRepo.DeleteByPodName(ctx, deploymentID, podName); err != nil { // 削除された Pod のチャンクを DB から削除する
				logger.PrintErr("watchPodDeletions: Podログチャンク削除に失敗しました（pod=" + podName + "）: " + err.Error()) // エラーをログ出力する
			}
		}

		if ctx.Err() != nil { // コンテキストキャンセルの場合は終了する
			return
		}
		logger.Println("watchPodDeletions: Watch チャンネルが閉じられました。再接続します") // 再接続ログを出力する
	}
}

// calcAppStatus は k8s Deployment の DeploymentAvailable 条件から AppStatus を計算する
// kubectl rollout status と同じ判定でローリングアップデート中の誤検知を防ぐ
func calcAppStatus(k8sDeployment *appsv1.Deployment) models.AppStatus {
	for _, condition := range k8sDeployment.Status.Conditions { // Deployment の条件一覧を確認する
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue { // Available 条件が True の場合
			return models.AppStatusRunning // running を返す
		}
	}
	return models.AppStatusDeploying // Available 条件が満たされていない場合は deploying を返す
}
