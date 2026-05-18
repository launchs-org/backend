package watcher

import (
	"context"
	"fmt"
	"os"
	"time"

	"launchs/shared/database"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WatchJobs は K8s Job を監視するメインループです。
func WatchJobs(ctx context.Context) {
	namespace := os.Getenv("BUILD_NAMESPACE")
	if namespace == "" {
		namespace = "buildkit"
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	fmt.Println("[job-watcher] starting job watcher in namespace:", namespace)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Println("[job-watcher] starting job watch...")

		if err := runJobWatch(ctx, clientset, namespace); err != nil {
			fmt.Printf("[job-watcher] watch error: %v, restarting...\n", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// runJobWatch は K8s Job の Watch を開始し、イベントを受け取り続けます。
func runJobWatch(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	watcher, err := clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "launchs-managed=true",
	})
	if err != nil {
		return fmt.Errorf("failed to start job watch: %w", err)
	}
	defer watcher.Stop()

	streamedJobs := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if err := handleJobEvent(ctx, clientset, namespace, event, streamedJobs); err != nil {
				fmt.Printf("[job-watcher] handle event error: %v\n", err)
			}
		}
	}
}
