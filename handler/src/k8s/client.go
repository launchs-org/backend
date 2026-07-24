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

// buildConfig は Pod 内で実行されていれば in-cluster 設定を使い、そうでなければ kubeconfig にフォールバックする
func buildConfig() (*rest.Config, error) {
	// KUBERNETES_SERVICE_HOST は Pod 内実行時に必ず設定されるため、これを in-cluster 判定に使う
	// （HOME 環境変数はコンテナランタイムが /etc/passwd から自動補完することがあり、kubeconfig の有無判定には使えない）
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return rest.InClusterConfig() // Pod 内の ServiceAccount から設定を構築する
	}
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config") // kubeconfig のパスを組み立てる
	return clientcmd.BuildConfigFromFlags("", kubeconfig)             // ローカル実行時は kubeconfig から設定を構築する
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
