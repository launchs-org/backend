package main

import (
	"context"
	"net/http"
	"os"

	"backend/middlewares"
	tmpl "backend/template"
	"launchs/shared/database"
	"launchs/shared/job_queue"
	"launchs/shared/model"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func runMigrate() {
	database.InitTaskDB()
	if err := job_queue.UseRiver(context.Background(), database.TaskDB, nil); err != nil {
		panic("failed to migrate river: " + err.Error())
	}

	database.Init()
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
		panic("failed to migrate database: " + err.Error())
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate()
		return
	}

	database.Init()
	database.InitTaskDB()

	if err := job_queue.UseRiver(context.Background(), database.TaskDB, nil); err != nil {
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
		&model.PodStatus{},
	); err != nil {
		panic("failed to migrate database: " + err.Error())
	}

	middlewares.Init()

	if err := tmpl.LoadAll(); err != nil {
		panic("failed to load templates: " + err.Error())
	}

	router := echo.New()
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	InitRouter(router)

	router.GET("/", func(ctx *echo.Context) error {
		return (*ctx).String(http.StatusOK, "backend ok")
	})

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}
	if err := router.Start(":" + port); err != nil {
		router.Logger.Error("server error", "error", err)
	}
}
