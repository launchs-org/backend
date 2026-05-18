package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"launchs/shared/database"
	"launchs/shared/job_queue"
	"launchs/shared/model"
	"watcher/watcher"
)

func main() {
	fmt.Println("[watcher] starting...")

	database.Init()
	database.InitK8s()
	database.InitTaskDB()

	if err := database.DB.AutoMigrate(
		&model.Project{},
		&model.Container{},
		&model.BuildJob{},
		&model.Image{},
		&model.Service{},
		&model.Ingress{},
		&model.Volume{},
		&model.HarborCredential{},
		&model.PodStatus{},
	); err != nil {
		fmt.Printf("[watcher] migration error: %v\n", err)
	}

	if err := job_queue.UseRiver(context.Background(), database.TaskDB, nil); err != nil {
		panic("[watcher] failed to init job queue: " + err.Error())
	}

	fmt.Println("[watcher] initialized database, k8s")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go watcher.WatchJobs(ctx)
	go watcher.WatchDeployments(ctx)

	<-ctx.Done()
	fmt.Println("[watcher] shutting down")
}
