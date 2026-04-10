package controllers

import (
	"net/http"

	"build/services"

	"github.com/labstack/echo/v4"
)

// BuildController ビルド関連のリクエストを処理するコントローラー
type BuildController struct {
	buildService services.BuildService
}

// NewBuildController BuildController の新しいインスタンスを作成する
func NewBuildController(service services.BuildService) *BuildController {
	return &BuildController{
		buildService: service,
	}
}

// TriggerBuild ビルドを開始するエンドポイント
func (b *BuildController) TriggerBuild(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusAccepted, echo.Map{"message": "trigger build logic here"})
}

// GetBuildStatus ビルドステータスを取得するエンドポイント
func (b *BuildController) GetBuildStatus(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get build status logic here"})
}

// GetBuildLogs ビルドログを取得するエンドポイント
func (b *BuildController) GetBuildLogs(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get build logs logic here"})
}
