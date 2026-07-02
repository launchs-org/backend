package k8s

import (
	"handler/logger"
	"handler/models"
	"handler/repository"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"gorm.io/datatypes"
)

// ApplyPVC は k8s に PersistentVolumeClaim を作成または更新する
// PVC の spec は bound 後に immutable なため、既存 PVC が存在する場合は作成をスキップする
func ApplyPVC(ctx context.Context, client kubernetes.Interface, pvcManifest *corev1.PersistentVolumeClaim) error {
	_, err := client.CoreV1().PersistentVolumeClaims(pvcManifest.Namespace).Get(ctx, pvcManifest.Name, metav1.GetOptions{}) // 既存の PVC を取得する
	if err == nil {
		return nil // 既存 PVC があれば何もしない（spec は immutable なため更新不可）
	}
	// 存在しない場合は新規作成する
	_, err = client.CoreV1().PersistentVolumeClaims(pvcManifest.Namespace).Create(ctx, pvcManifest, metav1.CreateOptions{})
	return err
}

// DeletePVC は k8s から PersistentVolumeClaim を削除する
// PVC が存在しない場合（既に削除済みまたは未作成）は正常終了とみなす
func DeletePVC(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	err := client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}) // PVC を削除する
	if err != nil && !errors.IsNotFound(err) {                                                          // Not Found 以外のエラーは返す
		return err // 削除エラーを返す
	}
	return nil // 正常終了
}

// BuildPVCManifest は Volume モデルの情報から PVC マニフェストを生成する
func BuildPVCManifest(namespace, name string, sizeMB int, storageClassName string) *corev1.PersistentVolumeClaim {
	storageRequest := resource.MustParse(fmt.Sprintf("%dMi", sizeMB)) // SizeMB を MiB 単位の Quantity に変換する
	pvcManifest := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany, // 複数からのアクセスを許可する
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageRequest, // ストレージ容量を設定する
				},
			},
		},
	}
	if storageClassName != "" {
		pvcManifest.Spec.StorageClassName = &storageClassName // StorageClass が指定されている場合に設定する
	}
	return pvcManifest
}

// WatchPVCs は全 Namespace の PVC 変化を監視して DB の Volume.status を自動更新する
func WatchPVCs(ctx context.Context, k8sClient kubernetes.Interface, volumeRepo repository.VolumeRepository) {
	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, err := k8sClient.CoreV1().PersistentVolumeClaims("").Watch(ctx, metav1.ListOptions{}) // 全 PVC を監視する
		if err != nil {
			logger.PrintErr("WatchPVCs: Watch 開始に失敗しました: " + err.Error()) // エラーをログ出力する
			continue                                                               // 再試行する
		}

		logger.Println("WatchPVCs: 監視を開始しました") // 監視開始ログを出力する

		pvcWatchLoop(ctx, watcher, volumeRepo) // イベントループを実行する

		logger.Println("WatchPVCs: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// pvcWatchLoop は PVC Watch イベントチャネルを処理するループ
func pvcWatchLoop(ctx context.Context, watcher watch.Interface, volumeRepo repository.VolumeRepository) {
	defer watcher.Stop() // 終了時に Watch を停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case event, ok := <-watcher.ResultChan(): // イベントを受信する
			if !ok { // チャネルが閉じられた場合はループを抜ける
				return
			}
			handlePVCEvent(ctx, event, volumeRepo) // イベントを処理する
		}
	}
}

// handlePVCEvent は PVC の Watch イベントを処理する
func handlePVCEvent(ctx context.Context, event watch.Event, volumeRepo repository.VolumeRepository) {
	pvc, ok := event.Object.(*corev1.PersistentVolumeClaim) // イベントオブジェクトを PVC にキャストする
	if !ok {                                                  // キャストに失敗した場合はスキップする
		return
	}

	if event.Type != watch.Added && event.Type != watch.Modified { // Added/Modified 以外はスキップする
		return
	}

	pvcName := pvc.Name // PVC 名を取得する
	if !strings.HasSuffix(pvcName, "-pvc") { // launchs が管理する PVC は "{volume_id}-pvc" の命名規則に従う
		return
	}

	volumeID := strings.TrimSuffix(pvcName, "-pvc") // PVC 名から volume_id を抽出する
	if volumeID == "" {                               // volume_id が空の場合はスキップする
		return
	}

	if pvc.Status.Phase != corev1.ClaimBound { // Bound 状態でない場合はスキップする
		return
	}

	k8sStatusJSON, err := marshalPVCStatus(pvc.Status) // PVCStatus を JSON にシリアライズする
	if err != nil {
		logger.PrintErr("WatchPVCs: k8s_status のシリアライズに失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	if err := volumeRepo.UpdateStatus(ctx, volumeID, models.VolumeStatusBound, k8sStatusJSON); err != nil { // status を bound に更新する
		logger.PrintErr("WatchPVCs: status 更新に失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	logger.Println("WatchPVCs: Volume を bound に更新しました: " + volumeID) // 更新ログを出力する
}

// marshalPVCStatus は corev1.PersistentVolumeClaimStatus を datatypes.JSON にシリアライズする
func marshalPVCStatus(status corev1.PersistentVolumeClaimStatus) (datatypes.JSON, error) {
	statusBytes, err := json.Marshal(status) // PVCStatus を JSON バイト列に変換する
	if err != nil {
		return nil, err // シリアライズエラーを返す
	}
	return datatypes.JSON(statusBytes), nil // datatypes.JSON に変換して返す
}
