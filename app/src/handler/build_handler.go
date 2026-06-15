package handler

import (
	"app/service"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// BuildHandler はビルドトリガーの HTTP ハンドラーを提供する
type BuildHandler struct {
	buildService service.BuildService // ビルドサービスのインターフェース
}

// NewBuildHandler は BuildHandler を生成して返す
func NewBuildHandler(buildService service.BuildService) *BuildHandler {
	return &BuildHandler{
		buildService: buildService, // 依存を注入する
	}
}

// TriggerBuild は POST /api/v1/deployments/:id/build のハンドラー
func (buildHandler *BuildHandler) TriggerBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")     // パスパラメータから deployment ID を取得する

	buildData, err := buildHandler.buildService.TriggerBuild(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出してビルドをトリガーする
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrBuildConflict) { // ビルド中の場合は 409 を返す
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "ビルドが既に進行中です",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, buildData) // 作成したビルドレコードを返す
}
