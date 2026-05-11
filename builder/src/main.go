package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"builder/worker"
	"launchs/shared/database"
	"launchs/shared/job_queue"
	"launchs/shared/model"

	"github.com/riverqueue/river"
)

func main() {
	fmt.Println("[builder] starting...")

	database.Init()
	database.InitK8s()
	database.InitRedis()
	database.InitTaskDB()

	// DB マイグレーション実行
	if err := database.DB.AutoMigrate(
		&model.Project{},
		&model.Container{},
		&model.BuildJob{},
		&model.Image{},
		&model.Service{},
		&model.Ingress{},
		&model.Volume{},
		&model.HarborCredential{},
	); err != nil {
		fmt.Printf("[builder] migration error: %v\n", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// River ワーカーを登録して起動
	workers := river.NewWorkers()
	river.AddWorker(workers, &worker.BuildWorker{})
	river.AddWorker(workers, &worker.DeleteImageWorker{})

	if err := job_queue.UseRiver(ctx, database.TaskDB, workers, "builder"); err != nil {
		panic("[builder] failed to start job queue: " + err.Error())
	}

	fmt.Println("[builder] job queue started, waiting for jobs...")
	<-ctx.Done()
	fmt.Println("[builder] shutting down")
}
