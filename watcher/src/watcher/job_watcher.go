package watcher

import (
	"context"
	"fmt"
	"os"
	"time"

	"launchs/shared/database"

	"github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WatchJobs は K8s Job を監視するメインループです。
// エラーが発生した場合は 3 秒待機して自動で再起動します。
func WatchJobs(ctx context.Context) {
	namespace := os.Getenv("BUILD_NAMESPACE")
	if namespace == "" {
		namespace = "buildkit"
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)
	redisClient := database.RedisClient

	fmt.Println("[job-watcher] starting job watcher in namespace:", namespace)

	for {
		// コンテキストがキャンセルされたら終了
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Println("[job-watcher] starting job watch...")

		if err := runJobWatch(ctx, clientset, redisClient, namespace); err != nil {
			fmt.Printf("[job-watcher] watch error: %v, restarting...\n", err)
		}

		// 再起動前に少し待機（K8s API サーバーへの負荷軽減）
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// runJobWatch は K8s Job の Watch を開始し、イベントを受け取り続けます。
// チャンネルが閉じられた場合はエラーを返してループ側で再接続します。
func runJobWatch(ctx context.Context, clientset *kubernetes.Clientset, redisClient *redis.Client, namespace string) error {
	// "launchs-managed=true" ラベルが付いた Job だけを対象に Watch する。
	// このラベルは builder/railpack/job.go の createJob() で付与される。
	watcher, err := clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		return fmt.Errorf("failed to start job watch: %w", err)
	}
	defer watcher.Stop()

	// ログストリームを開始済みの Job 名を追跡して二重起動を防ぐ
	streamedJobs := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			// チャンネルが閉じられた場合は上位ループで再接続する
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if err := handleJobEvent(ctx, clientset, redisClient, namespace, event, streamedJobs); err != nil {
				fmt.Printf("[job-watcher] handle event error: %v\n", err)
			}
		}
	}
}
