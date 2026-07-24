package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// buildConfig は kubeconfig が存在すればそれを使い、存在しなければ in-cluster 設定にフォールバックする
func buildConfig() (*rest.Config, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config") // kubeconfig のパスを組み立てる
	if _, statErr := os.Stat(kubeconfig); statErr == nil {
		return clientcmd.BuildConfigFromFlags("", kubeconfig) // kubeconfig が存在する場合はそこから設定を構築する
	}
	return rest.InClusterConfig() // kubeconfig が存在しない場合は Pod 内の ServiceAccount から設定を構築する
}

// NewClient は kubeconfig または in-cluster 設定から通常の k8s クライアントを生成する
func NewClient() (*kubernetes.Clientset, error) {
	config, err := buildConfig() // クライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return kubernetes.NewForConfig(config) // クライアントセットを生成して返す
}

// NewDynamicClient は kubeconfig または in-cluster 設定から dynamic クライアントを生成する（Traefik CRD 等に使用）
func NewDynamicClient() (dynamic.Interface, error) {
	config, err := buildConfig() // クライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return dynamic.NewForConfig(config) // dynamic クライアントを生成して返す
}

// NewMetricsClient は kubeconfig または in-cluster 設定から Metrics Server 用クライアントを生成する
func NewMetricsClient() (metricsv1beta1.Interface, error) {
	config, err := buildConfig() // クライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return metricsv1beta1.NewForConfig(config) // Metrics Server クライアントを生成して返す
}
