package activity

import (
	"app/shared/repository"
	"context"
	"fmt"

	"controller/k8s"

	k8sclient "k8s.io/client-go/kubernetes"
)

// VolumeActivities は Volume 系 Workflow で使われる Activity 群を保持する構造体
type VolumeActivities struct {
	k8sClient        k8sclient.Interface                // k8s クライアント
	volumeRepo       repository.VolumeRepository        // volume リポジトリ
	projectRepo      repository.ProjectRepository       // project リポジトリ
	storageClassName string                             // StorageClass 名
}

// NewVolumeActivities は VolumeActivities を生成して返す
func NewVolumeActivities(
	k8sClient k8sclient.Interface,
	volumeRepo repository.VolumeRepository,
	projectRepo repository.ProjectRepository,
	storageClassName string,
) *VolumeActivities {
	return &VolumeActivities{ // 依存を注入して返す
		k8sClient:        k8sClient,
		volumeRepo:       volumeRepo,
		projectRepo:      projectRepo,
		storageClassName: storageClassName,
	}
}

// VolumeWorkflowInput は Volume 系 Workflow への共通入力
type VolumeWorkflowInput struct {
	VolumeID string // 対象ボリュームのID
}

// CreateK8sPVCActivity は k8s PVC を作成する Activity
func (activities *VolumeActivities) CreateK8sPVCActivity(ctx context.Context, input VolumeWorkflowInput) error {
	volumeData, err := activities.volumeRepo.FindByID(ctx, input.VolumeID) // volume を取得する
	if err != nil {
		return fmt.Errorf("volume not found: %w", err) // 取得エラーを返す
	}

	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, volumeData.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	pvcName := volumeData.ID + "-pvc"                                                                                  // PVC 名を VolumeID から生成する
	pvcManifest := k8s.BuildPVCManifest(projectData.Namespace, pvcName, volumeData.SizeMB, activities.storageClassName) // PVC マニフェストを生成する
	if err := k8s.ApplyPVC(ctx, activities.k8sClient, pvcManifest); err != nil {                                       // k8s に PVC を作成する
		return fmt.Errorf("k8s PVC 作成に失敗しました: %w", err) // 作成エラーを返す
	}
	return nil // 正常終了を返す
}

// DeleteK8sPVCActivity は k8s PVC を削除する Activity
func (activities *VolumeActivities) DeleteK8sPVCActivity(ctx context.Context, input VolumeWorkflowInput) error {
	volumeData, err := activities.volumeRepo.FindByID(ctx, input.VolumeID) // volume を取得する
	if err != nil {
		return fmt.Errorf("volume not found: %w", err) // 取得エラーを返す
	}

	projectData, err := activities.projectRepo.FindByIDNoTx(ctx, volumeData.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project not found: %w", err) // 取得エラーを返す
	}

	pvcName := volumeData.ID + "-pvc"                                                                // PVC 名を VolumeID から生成する
	if err := k8s.DeletePVC(ctx, activities.k8sClient, projectData.Namespace, pvcName); err != nil { // k8s から PVC を削除する
		return fmt.Errorf("k8s PVC 削除に失敗しました: %w", err) // 削除エラーを返す
	}
	return nil // 正常終了を返す
}

// DeleteVolumeRecordActivity は DB から volume レコードを削除する Activity
func (activities *VolumeActivities) DeleteVolumeRecordActivity(ctx context.Context, input VolumeWorkflowInput) error {
	volumeData, err := activities.volumeRepo.FindByID(ctx, input.VolumeID) // volume を取得する
	if err != nil {
		return fmt.Errorf("volume not found: %w", err) // 取得エラーを返す
	}

	if err := activities.volumeRepo.DeleteNoTx(ctx, volumeData); err != nil { // volume レコードを削除する
		return fmt.Errorf("volume record delete: %w", err) // 削除エラーを返す
	}
	return nil // 正常終了を返す
}
