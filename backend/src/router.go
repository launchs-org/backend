package main

import (
	"backend/controller"
	"backend/middlewares"
	"net/http"

	"github.com/labstack/echo/v5"
)

func InitRouter(router *echo.Echo) {
	v1Group := router.Group("/v1")
	v1Group.Use(middlewares.RequireAuth)

	{
		projectsGroup := v1Group.Group("/projects")
		{
			projectsGroup.GET("", controller.ListProjects)
			projectsGroup.POST("", controller.CreateProject)
			projectsGroup.GET("/:id", controller.GetProject)
			projectsGroup.DELETE("/:id", controller.DeleteProject)
			projectsGroup.POST("/:id/containers", controller.CreateContainer)

			projectsGroup.GET("/:id/histories", func(c *echo.Context) error {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"items": []map[string]interface{}{},
						"total": 0,
					},
				})
			})
			projectsGroup.POST("/:id/rollback/:phid", func(c *echo.Context) error {
				return c.String(http.StatusOK, "not implemented")
			})
		}

		containersGroup := v1Group.Group("/containers")
		{
			containersGroup.GET("/:id", controller.GetContainer)
			containersGroup.PATCH("/:id", controller.UpdateContainer)
			containersGroup.PATCH("/:id/env-vars", controller.UpdateContainerEnvVars)
			containersGroup.POST("/:id/rebuild", controller.RebuildContainer)
			containersGroup.POST("/:id/redeploy", controller.RedeployContainer)
			containersGroup.POST("/:id/scale", controller.ScaleContainer)
			containersGroup.DELETE("/:id", controller.DeleteContainer)
			containersGroup.GET("/:id/build-jobs", controller.ListBuildJobs)
			containersGroup.GET("/:id/logs", controller.GetContainerLogs)
			containersGroup.PATCH("/:id/service", controller.UpdateService)
			containersGroup.POST("/:id/ingress", controller.CreateIngress)
			containersGroup.PATCH("/:id/ingress", controller.UpdateIngress)
			containersGroup.DELETE("/:id/ingress", controller.DeleteIngress)
			containersGroup.POST("/:id/volumes", controller.CreateContainerVolume)
			containersGroup.GET("/:id/volumes", controller.ListContainerVolumes)
		}

		buildJobsGroup := v1Group.Group("/build-jobs")
		{
			buildJobsGroup.POST("/:id/cancel", func(c *echo.Context) error {
				return c.String(http.StatusOK, "not implemented")
			})
			buildJobsGroup.GET("/:id/logs", controller.GetBuildJobLogs)
		}

		volumesGroup := v1Group.Group("/volumes")
		{
			volumesGroup.DELETE("/:id", controller.DeleteVolume)
		}

		streamGroup := v1Group.Group("/stream")
		{
			streamGroup.GET("/build-jobs/:id", controller.GetBuildJobLogs)
		}
	}
}
