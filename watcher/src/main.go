package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"launchs/shared/database"
	"watcher/leader"
	"watcher/watcher"
)

func main() {
	fmt.Println("[watcher] starting...")

	// 各種クライアント初期化
	database.Init()
	database.InitK8s()
	database.InitRedis()

	fmt.Println("[watcher] initialized database, k8s, redis")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = "watcher-1"
	}
	fmt.Printf("[watcher] pod name: %s\n", podName)

	// リーダー選出してから Watch を開始
	leader.RunWithLeaderElection(ctx, database.RedisClient, func(leaderCtx context.Context) {
		fmt.Println("[watcher] I am the leader, starting watchers")

		// Job Watcher 起動
		go watcher.WatchJobs(leaderCtx)

		// Deployment Watcher 起動
		go watcher.WatchDeployments(leaderCtx)

		// リーダーコンテキストが終わるまで待機
		<-leaderCtx.Done()
		fmt.Println("[watcher] leader context done")
	})

	fmt.Println("[watcher] shutting down")
}
