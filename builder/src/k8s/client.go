package k8s

import (
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// NewClient は ~/.kube/config から k8s クライアントを生成する
func NewClient() (*kubernetes.Clientset, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config") // kubeconfig のパスを組み立てる
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)     // kubeconfig からクライアント設定を構築する
	if err != nil {
		return nil, err // 設定構築に失敗した場合はエラーを返す
	}
	return kubernetes.NewForConfig(config) // クライアントセットを生成して返す
}
