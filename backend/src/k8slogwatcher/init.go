package k8slogwatcher

import (
	"launchs/shared/database"
	"fmt"

	"k8s.io/client-go/kubernetes"
)

var (
	// GlobalWatcher は Deployment 監視用のグローバルインスタンスです
	GlobalWatcher *Watcher
	// GlobalJobWatcher は Job 監視用のグローバルインスタンスです
	GlobalJobWatcher *JobWatcher
)

// Init は Watcher 類を初期化します
func Init() {
	var err error

	// database.K8sClientset を *kubernetes.Clientset にキャスト
	clientset, ok := database.K8sClientset.(*kubernetes.Clientset)
	if !ok {
		panic("database.K8sClientset is not *kubernetes.Clientset")
	}

	// Watcher の初期化
	GlobalWatcher, err = NewWatcher(clientset, database.RedisClient)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize GlobalWatcher: %v", err))
	}

	// JobWatcher の初期化
	GlobalJobWatcher, err = NewJobWatcher(clientset, database.RedisClient)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize GlobalJobWatcher: %v", err))
	}
}
