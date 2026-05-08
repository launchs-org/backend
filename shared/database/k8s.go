package database

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var K8sClientset kubernetes.Interface
var K8sDynamicClient dynamic.Interface

func InitK8s() {
	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
			kubeconfig = envKubeconfig
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(fmt.Sprintf("Kubernetes 設定の読み込みに失敗しました: %v", err))
		}
	}

	K8sClientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("Kubernetes クライアントの作成に失敗しました: %v", err))
	}

	K8sDynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("Kubernetes ダイナミッククライアントの作成に失敗しました: %v", err))
	}
}
