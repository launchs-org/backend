package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNamespace は新しい Namespace を作成します
func CreateNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"managed-by": "launchs-org",
			},
		},
	}
	_, err := Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// DeleteNamespace は Namespace を削除します
func DeleteNamespace(ctx context.Context, name string) error {
	return Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}
