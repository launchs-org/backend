package k8s

import (
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// NewClient は ~/.kube/config から通常の k8s クライアントを生成する
func NewClient() (*kubernetes.Clientset, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config") // kubeconfig のパスを組み立てる
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)     // kubeconfig からクライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return kubernetes.NewForConfig(config) // クライアントセットを生成して返す
}

// NewDynamicClient は ~/.kube/config から dynamic クライアントを生成する（Traefik CRD 等に使用）
func NewDynamicClient() (dynamic.Interface, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config") // kubeconfig のパスを組み立てる
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)     // kubeconfig からクライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return dynamic.NewForConfig(config) // dynamic クライアントを生成して返す
}

// NewMetricsClient は ~/.kube/config から Metrics Server 用クライアントを生成する
func NewMetricsClient() (metricsv1beta1.Interface, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config") // kubeconfig のパスを組み立てる
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)     // kubeconfig からクライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return metricsv1beta1.NewForConfig(config) // Metrics Server クライアントを生成して返す
}
