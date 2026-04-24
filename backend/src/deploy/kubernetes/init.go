package kubernetes

import (
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var Clientset *kubernetes.Clientset
var DynamicClient dynamic.Interface

// Init は Kubernetes クライアントを初期化します
func Init() error {
	var config *rest.Config
	var err error

	// インクラスター設定を試行
	config, err = rest.InClusterConfig()
	if err != nil {
		// 失敗した場合はローカルの kubeconfig を試行
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}
	}

	Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}
