package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CheckEndpointsReady は指定した Service に紐づく Endpoints に Ready なアドレスが1件以上あるかを返す
func CheckEndpointsReady(ctx context.Context, k8sClient kubernetes.Interface, namespace string, serviceName string) (bool, error) {
	endpointsData, err := k8sClient.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{}) // Service と同名の Endpoints を取得する
	if err != nil {
		if isNotFound(err) { // Endpoints が未作成の場合は未 Ready 扱いとする
			return false, nil
		}
		return false, err // その他のエラーは伝播する
	}

	for _, subset := range endpointsData.Subsets { // 各 Subset を確認する
		if len(subset.Addresses) > 0 { // Ready なアドレスが1件以上あれば Ready と判定する
			return true, nil
		}
	}
	return false, nil // Ready なアドレスが存在しない
}
