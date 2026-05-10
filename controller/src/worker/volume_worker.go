package worker

import (
	"context"
	"fmt"
	"time"

	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.CreateVolumeJobArgs] = (*CreateVolumeWorker)(nil)
var _ river.Worker[jobs.DeleteVolumeJobArgs] = (*DeleteVolumeWorker)(nil)

type CreateVolumeWorker struct {
	river.WorkerDefaults[jobs.CreateVolumeJobArgs]
}

func (w *CreateVolumeWorker) Timeout(*river.Job[jobs.CreateVolumeJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *CreateVolumeWorker) Work(ctx context.Context, job *river.Job[jobs.CreateVolumeJobArgs]) error {
	payload := job.Args
	fmt.Printf("[create-volume-worker] processing job %d (volume_id: %s)\n", job.ID, payload.VolumeID)

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	pvcName := fmt.Sprintf("pvc-%s", payload.VolumeID)

	pvcSpec := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: payload.Namespace,
			Labels: map[string]string{
				"managed-by": "launchs",
				"volume-id":  payload.VolumeID,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dMi", payload.SizeMB)),
				},
			},
		},
	}

	if _, err := clientset.CoreV1().PersistentVolumeClaims(payload.Namespace).Create(ctx, pvcSpec, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create PVC %s: %w", pvcName, err)
	}

	vol, err := model.GetVolumeByID(payload.VolumeID)
	if err == nil {
		vol.Status = "Available"
		if updateErr := model.UpdateVolume(vol); updateErr != nil {
			fmt.Printf("[create-volume-worker] warning: failed to update volume status: %v\n", updateErr)
		}
	}

	fmt.Printf("[create-volume-worker] created PVC %s in %s\n", pvcName, payload.Namespace)
	return nil
}

type DeleteVolumeWorker struct {
	river.WorkerDefaults[jobs.DeleteVolumeJobArgs]
}

func (w *DeleteVolumeWorker) Timeout(*river.Job[jobs.DeleteVolumeJobArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *DeleteVolumeWorker) Work(ctx context.Context, job *river.Job[jobs.DeleteVolumeJobArgs]) error {
	payload := job.Args
	fmt.Printf("[delete-volume-worker] processing job %d (volume_id: %s)\n", job.ID, payload.VolumeID)

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	pvcName := fmt.Sprintf("pvc-%s", payload.VolumeID)

	err := clientset.CoreV1().PersistentVolumeClaims(payload.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PVC %s: %w", pvcName, err)
	}

	if err := model.DeleteVolume(payload.VolumeID); err != nil {
		return fmt.Errorf("failed to delete volume from DB: %w", err)
	}

	fmt.Printf("[delete-volume-worker] deleted PVC %s and volume record\n", pvcName)
	return nil
}
