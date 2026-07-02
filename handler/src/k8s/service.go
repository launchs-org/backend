package k8s

import (
	"handler/logger"
	"handler/models"
	"handler/repository"
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"gorm.io/datatypes"
)

// ApplyService は k8s に Service を作成または更新する
func ApplyService(ctx context.Context, client kubernetes.Interface, serviceManifest *corev1.Service) error {
	existing, err := client.CoreV1().Services(serviceManifest.Namespace).Get(ctx, serviceManifest.Name, metav1.GetOptions{}) // 既存の Service を取得する
	if err != nil {
		// 存在しない場合は新規作成する
		_, err = client.CoreV1().Services(serviceManifest.Namespace).Create(ctx, serviceManifest, metav1.CreateOptions{})
		return err
	}
	// 既存の ResourceVersion と ClusterIP を引き継いで更新する（k8s の楽観的並行性制御および ClusterIP 不変制約のため）
	serviceManifest.ResourceVersion = existing.ResourceVersion
	serviceManifest.Spec.ClusterIP = existing.Spec.ClusterIP
	_, err = client.CoreV1().Services(serviceManifest.Namespace).Update(ctx, serviceManifest, metav1.UpdateOptions{})
	return err
}

// DeleteService は k8s から Service を削除する
func DeleteService(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}) // Service を削除する
}

// WatchServices は全 Namespace の k8s Service 変化を監視して DB を自動更新する
func WatchServices(ctx context.Context, k8sClient kubernetes.Interface, serviceRepo repository.ServiceRepository) {
	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, err := k8sClient.CoreV1().Services("").Watch(ctx, metav1.ListOptions{
			LabelSelector: "launchs.org/service-id", // launchs.org/service-id ラベルを持つ Service のみ監視する
		}) // Watch を開始する
		if err != nil {
			logger.PrintErr("WatchServices: Watch 開始に失敗しました: " + err.Error()) // エラーをログ出力する
			continue                                                                   // 再試行する
		}

		logger.Println("WatchServices: 監視を開始しました") // 監視開始ログを出力する

		serviceWatchLoop(ctx, watcher, serviceRepo) // イベントループを実行する

		logger.Println("WatchServices: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// serviceWatchLoop は k8s Service Watch イベントチャネルを処理するループ
func serviceWatchLoop(ctx context.Context, watcher watch.Interface, serviceRepo repository.ServiceRepository) {
	defer watcher.Stop() // 終了時に Watch を停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case event, ok := <-watcher.ResultChan(): // イベントを受信する
			if !ok { // チャネルが閉じられた場合はループを抜ける
				return
			}
			handleServiceEvent(ctx, event, serviceRepo) // イベントを処理する
		}
	}
}

// handleServiceEvent は k8s Service の Watch イベントを処理する
func handleServiceEvent(ctx context.Context, event watch.Event, serviceRepo repository.ServiceRepository) {
	k8sService, ok := event.Object.(*corev1.Service) // イベントオブジェクトを Service にキャストする
	if !ok {                                           // キャストに失敗した場合はスキップする
		return
	}

	serviceID, exists := k8sService.Labels["launchs.org/service-id"] // service-id ラベルを取得する
	if !exists || serviceID == "" {                                    // ラベルが存在しない場合はスキップする
		return
	}

	if event.Type != watch.Added && event.Type != watch.Modified { // Added/Modified 以外はスキップする
		return
	}

	k8sStatusJSON, err := marshalServiceStatus(k8sService.Status) // ServiceStatus を JSON にシリアライズする
	if err != nil {
		logger.PrintErr("WatchServices: k8s_status のシリアライズに失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	if err := serviceRepo.UpdateStatus(ctx, serviceID, models.ServiceStatusActive, k8sStatusJSON); err != nil { // status を active に更新する
		logger.PrintErr("WatchServices: status 更新に失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	if k8sService.Spec.ClusterIP != "" { // ClusterIP が割り当てられている場合のみ保存する
		if err := serviceRepo.UpdateClusterIP(ctx, serviceID, k8sService.Spec.ClusterIP); err != nil { // cluster_ip を DB に保存する
			logger.PrintErr("WatchServices: cluster_ip 更新に失敗しました: " + err.Error()) // エラーをログ出力する
		}
	}

	logger.Println("WatchServices: status を更新しました: " + serviceID) // 更新ログを出力する
}

// marshalServiceStatus は corev1.ServiceStatus を datatypes.JSON にシリアライズする
func marshalServiceStatus(status corev1.ServiceStatus) (datatypes.JSON, error) {
	statusBytes, err := json.Marshal(status) // ServiceStatus を JSON バイト列に変換する
	if err != nil {
		return nil, err // シリアライズエラーを返す
	}
	return datatypes.JSON(statusBytes), nil // datatypes.JSON に変換して返す
}
