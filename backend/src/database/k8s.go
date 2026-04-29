package database

import (
	"fmt"      // フォーマット
	"os"       // OS機能
	"path/filepath" // パス操作

	"k8s.io/client-go/kubernetes" // K8s クライアント
	"k8s.io/client-go/rest"      // K8s REST
	"k8s.io/client-go/tools/clientcmd" // K8s 設定
	"k8s.io/client-go/util/homedir"    // ホームディレクトリ
)

// K8sClientset は Kubernetes クライアントセットを保持するグローバル変数です
var K8sClientset *kubernetes.Clientset

// InitK8s は Kubernetes クライアントを初期化します
func InitK8s() {
	// K8s設定用の変数を宣言
	var config *rest.Config
	// エラー用の変数を宣言
	var err error

	// 最初にインクラスター（Pod内）設定を試みる
	config, err = rest.InClusterConfig()
	// インクラスター設定に失敗した場合
	if err != nil {
		// ローカルの kubeconfig パスを取得 (デフォルトは ~/.kube/config)
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		// 環境変数 KUBECONFIG が設定されている場合はそちらを優先する
		if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
			// パスを上書き
			kubeconfig = envKubeconfig
		}
		
		// 指定されたパスから設定を読み込む
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		// 読み込みに失敗した場合
		if err != nil {
			// パニックで終了
			panic(fmt.Sprintf("Kubernetes 設定の読み込みに失敗しました: %v", err))
		}
	}

	// クライアントセットを新規作成
	K8sClientset, err = kubernetes.NewForConfig(config)
	// 作成に失敗した場合
	if err != nil {
		// パニックで終了
		panic(fmt.Sprintf("Kubernetes クライアントの作成に失敗しました: %v", err))
	}
}
