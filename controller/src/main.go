package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"controller/worker"
	"launchs/shared/database"
	"launchs/shared/job_queue"
	"launchs/shared/model"

	"github.com/riverqueue/river"
)

func main() {
	database.Init()
	database.InitRedis()
	database.InitK8s()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	database.InitTaskDB()

	workers := river.NewWorkers()
	river.AddWorker(workers, &worker.DeployWorker{})
	river.AddWorker(workers, &worker.DeleteContainerWorker{})
	river.AddWorker(workers, &worker.DeleteProjectWorker{})
	river.AddWorker(workers, &worker.CreateProjectWorker{})
	river.AddWorker(workers, &worker.UpdateServiceWorker{})
	river.AddWorker(workers, &worker.CreateIngressWorker{})
	river.AddWorker(workers, &worker.UpdateIngressWorker{})
	river.AddWorker(workers, &worker.DeleteIngressWorker{})
	river.AddWorker(workers, &worker.CreateVolumeWorker{})
	river.AddWorker(workers, &worker.DeleteVolumeWorker{})
	river.AddWorker(workers, &worker.RolloutRestartWorker{})
	river.AddWorker(workers, &worker.ScaleWorker{})

	if err := job_queue.UseRiver(ctx, database.TaskDB, workers, "controller"); err != nil {
		panic("failed to initialize job queue: " + err.Error())
	}

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
		panic("failed to migrate database: " + err.Error())
	}

	<-ctx.Done()
	os.Exit(0)
}
