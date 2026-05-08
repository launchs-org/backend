package service

import (
	"context"         // コンテキスト
	"errors"          // エラー
	"fmt"             // 文字列フォーマット
	"time"            // 時間

	"launchs/shared/database" // データベースパッケージ
	"launchs/shared/model"   // モデルパッケージ

	"github.com/google/uuid"        // UUID生成
	corev1 "k8s.io/api/core/v1"    // K8s API
	"k8s.io/apimachinery/pkg/api/resource" // K8s リソース単位
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // K8s Meta API
	"k8s.io/client-go/kubernetes" // K8s クライアント
)

var (
	// ボリュームが見つからないエラー
	ErrVolumeNotFound = errors.New("volume not found")
	// ボリュームサイズが上限を超えているエラー (5GB = 5120MB)
	ErrVolumeSizeExceeded = errors.New("volume size exceeded (max 5GB)")
)

// CreateVolumeInput はボリューム作成の入力データです
type CreateVolumeInput struct {
	ProjectID   string // プロジェクトID
	OwnerID     string // 所有者ID
	ContainerID string // コンテナID (オプション)
	Name        string // ボリューム名
	SizeMB      int    // サイズ (MB)
	MountPath   string // マウントパス
}

// CreateVolume はボリュームを作成し、K8s PVC を発行します
func CreateVolume(ctx context.Context, input CreateVolumeInput) (*model.Volume, error) {
	// プロジェクトを取得
	project, err := model.GetProjectByID(input.ProjectID)
	if err != nil {
		// 見つからない場合はプロジェクトサービスのエラーを流用
		return nil, ErrProjectNotFound
	}

	// 権限チェック
	if project.OwnerID != input.OwnerID {
		return nil, ErrForbidden
	}

	// サイズバリデーション (5GBまで)
	if input.SizeMB > 5120 {
		return nil, ErrVolumeSizeExceeded
	}

	// IDを生成
	volumeID := "vol-" + uuid.New().String()

	// ボリュームモデルを作成
	volume := &model.Volume{
		ID:          volumeID,
		ProjectID:   input.ProjectID,
		ContainerID: input.ContainerID,
		Name:        input.Name,
		SizeMB:      input.SizeMB,
		MountPath:   input.MountPath,
		Status:      "Available",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Kubernetes PVC の定義を作成
	pvcSpec := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pvc-%s", volumeID), // PVC名は一意にする
			Namespace: project.Namespace,
			Labels: map[string]string{
				"managed-by": "launchs",
				"project-id": input.ProjectID,
				"volume-id":  volumeID,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany, //複数ノードから書き込みを許可
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dMi", input.SizeMB)), // 指定サイズを要求
				},
			},
		},
	}

	// K8s API で PVC を作成
	clientset := database.K8sClientset.(*kubernetes.Clientset)
	_, err = clientset.CoreV1().PersistentVolumeClaims(project.Namespace).Create(ctx, pvcSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s PVC: %w", err)
	}

	// DBに保存
	if err := model.CreateVolume(volume); err != nil {
		// 失敗した場合はK8s側を削除すべきだが、ここでは簡略化のためそのままエラーを返す
		return nil, fmt.Errorf("failed to save volume to DB: %w", err)
	}

	return volume, nil
}

// ListVolumes はコンテナIDに紐づくボリューム一覧を取得します
func ListVolumes(ctx context.Context, containerID string, ownerID string) ([]model.Volume, error) {
	// コンテナを取得
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, ErrContainerNotFound
	}

	// プロジェクトを取得
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, ErrProjectNotFound
	}

	// 権限チェック
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	// DBから取得
	return model.GetVolumesByContainerID(containerID)
}

// DeleteVolume はボリュームのステータスを削除中にし、K8s PVC を削除します
func DeleteVolume(ctx context.Context, volumeID string, ownerID string) error {
	// ボリュームを取得
	volume, err := model.GetVolumeByID(volumeID)
	if err != nil {
		return ErrVolumeNotFound
	}

	// プロジェクトを取得
	project, err := model.GetProjectByID(volume.ProjectID)
	if err != nil {
		return ErrProjectNotFound
	}

	// 権限チェック
	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	// ステータスを削除中に更新
	volume.Status = "Deleting"
	if err := model.UpdateVolume(volume); err != nil {
		return fmt.Errorf("failed to update volume status: %w", err)
	}

	// K8s PVC を削除
	clientset := database.K8sClientset.(*kubernetes.Clientset)
	err = clientset.CoreV1().PersistentVolumeClaims(project.Namespace).Delete(ctx, fmt.Sprintf("pvc-%s", volumeID), metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("failed to delete K8s PVC: %v\n", err)
		// PVC が既になくても同期処理でDBから消えるため続行
	}

	return nil
}

// StartVolumeSync は定期的に削除中のボリュームの状態を確認し、PVCが消えていればDBからも削除します
func StartVolumeSync() {
	go func() {
		ticker := time.NewTicker(10 * time.Second) // 10秒ごとに確認
		defer ticker.Stop()

		for range ticker.C {
			// 削除中のボリュームを全件取得
			var volumes []model.Volume
			if err := database.DB.Where("status = ?", "Deleting").Find(&volumes).Error; err != nil {
				fmt.Printf("failed to list deleting volumes: %v\n", err)
				continue
			}

			for _, vol := range volumes {
				project, err := model.GetProjectByID(vol.ProjectID)
				if err != nil {
					// プロジェクトがない場合は異常系としてDBから削除
					model.DeleteVolume(vol.ID)
					continue
				}

				clientset := database.K8sClientset.(*kubernetes.Clientset)
				pvcName := fmt.Sprintf("pvc-%s", vol.ID)
				_, err = clientset.CoreV1().PersistentVolumeClaims(project.Namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
				
				// K8s上でPVCが見つからない場合（＝削除が完了した場合）
				if err != nil {
					fmt.Printf("PVC %s is gone, deleting Volume %s from DB\n", pvcName, vol.ID)
					model.DeleteVolume(vol.ID)
				}
			}
		}
	}()
}
