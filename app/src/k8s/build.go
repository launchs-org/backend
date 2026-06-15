package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateBuildJob は dockerfile ビルダーによる k8s Job を作成する
// dockerfile ビルダーは設計検討中のため現時点では未実装
func CreateBuildJob(
	ctx context.Context,
	client kubernetes.Interface,
	buildID string,
	namespace string,
) (string, error) {
	return "", fmt.Errorf("dockerfile ビルダーは未実装です（ISSUE-051 で実装予定）") // dockerfile ビルダーは未実装のためエラーを返す
}

// DeleteBuildJob は jobName に対応する k8s Job を削除する
func DeleteBuildJob(ctx context.Context, client kubernetes.Interface, namespace, jobName string) error {
	propagationPolicy := metav1.DeletePropagationForeground                                                                    // Pod も連動して削除する
	return client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}) // Job を削除する
}
