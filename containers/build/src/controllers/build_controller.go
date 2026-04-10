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

// TriggerBuild ビルド実行リクエストを受け取り、非同期にビルドプロセスを開始します。
func (controller *BuildController) TriggerBuild(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusAccepted, echo.Map{"message": "trigger build logic here"})
}

// GetBuildStatus ビルドが現在どのフェーズにあるか、最新の状態を返します。
func (controller *BuildController) GetBuildStatus(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get build status logic here"})
}

// GetBuildLogs ビルド実行時に出力された標準出力/標準エラー出力を返します。
func (controller *BuildController) GetBuildLogs(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get build logs logic here"})
}
