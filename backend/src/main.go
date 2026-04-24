package main

import (
	"backend/middlewares"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	router := echo.New()
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	// ミドルウェア初期化
	middlewares.Init()

	router.GET("/", func(ctx *echo.Context) error {
		return ctx.String(http.StatusOK, "Hello, World!")
	})

	router.GET("/authed", middlewares.RequireAuth(func(ctx *echo.Context) error {
		return ctx.JSON(http.StatusOK,map[string]string{"message": "authed"})
	}))

	if err := router.Start("0.0.0.0:8090"); err != nil {
		router.Logger.Error("failed to start server", "error", err)
	}
}
