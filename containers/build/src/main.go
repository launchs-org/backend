package main

import (
	"net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	echoServer := echo.New()
	echoServer.Use(middleware.Logger())
	echoServer.Use(middleware.Recover())

	echoServer.GET("/", func(context echo.Context) error {
		return context.String(http.StatusOK, "Build Service")
	})

	echoServer.Logger.Fatal(echoServer.Start(":8080"))
}
