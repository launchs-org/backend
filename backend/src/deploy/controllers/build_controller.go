package controllers

import (
	"backend/deploy/services"
	"net/http"

	"github.com/labstack/echo/v5"
)

// BuildController はビルドジョブ関連の HTTP リクエストを処理します
type BuildController struct {
	containerService *services.ContainerService
}

// NewBuildController は BuildController の新しいインスタンスを作成します
func NewBuildController(service *services.ContainerService) *BuildController {
	return &BuildController{
		containerService: service,
	}
}

// CancelBuild は実行中のビルドジョブをキャンセルします
// POST /v1/build-jobs/:id/cancel
func (controller *BuildController) CancelBuild(ctx *echo.Context) error {
	buildJobID := (*ctx).Param("id")
	// TODO: ビルドキャンセルロジックの実装
	_ = buildJobID

	return (*ctx).JSON(http.StatusOK, map[string]string{"message": "ビルドがキャンセルされました"})
}

// StreamBuildLogs はビルドログを SSE で配信します
// GET /v1/stream/build-jobs/:id
func (controller *BuildController) StreamBuildLogs(ctx *echo.Context) error {
	// buildJobID := (*ctx).Param("id")
	
	(*ctx).Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	(*ctx).Response().WriteHeader(http.StatusOK)

	// TODO: SSE ストリーミングの実装
	
	return nil
}
