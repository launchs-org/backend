package activity

import (
	"app/shared/models"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// TestBuildVolumePVC_設定したStorageClassが使われる は、デプロイ時に作る PVC に設定値の StorageClass が入ることを確認する。
// 空文字だと k8s の既定の StorageClass(local-path など RWX 非対応のもの)になり、PVC が Pending のままデプロイが止まる。
func TestBuildVolumePVC_設定したStorageClassが使われる(t *testing.T) {
	activities := &ApplyActivities{storageClassName: "longhorn"}
	volumeData := &models.Volume{ID: "vol-1", SizeMB: 3072}

	pvc := activities.buildVolumePVC("project-ns", volumeData)

	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "longhorn" {
		t.Fatalf("StorageClass が longhorn であることを期待しましたが、実際: %v", pvc.Spec.StorageClassName)
	}
	if pvc.Name != "vol-1-pvc" || pvc.Namespace != "project-ns" {
		t.Errorf("名前と namespace が期待と異なります: %s/%s", pvc.Namespace, pvc.Name)
	}
	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Errorf("RWX であることを期待しましたが、実際: %v", pvc.Spec.AccessModes)
	}
}

// TestBuildVolumePVC_StorageClass未設定なら指定しない は、設定が空のときだけ StorageClass を指定しない(クラスタの既定に任せる)ことを確認する。
func TestBuildVolumePVC_StorageClass未設定なら指定しない(t *testing.T) {
	activities := &ApplyActivities{}

	pvc := activities.buildVolumePVC("project-ns", &models.Volume{ID: "vol-2", SizeMB: 1024})

	if pvc.Spec.StorageClassName != nil {
		t.Errorf("StorageClass が未指定(nil)であることを期待しましたが、実際: %v", *pvc.Spec.StorageClassName)
	}
}
