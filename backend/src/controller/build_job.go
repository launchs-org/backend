package controller

import (
	"backend/service"
	"net/http"

	"github.com/labstack/echo/v5"
)

// GetBuildJobLogs はビルドジョブのログを取得するハンドラーです
func GetBuildJobLogs(ctx *echo.Context) error {
	// パスパラメータからビルドジョブIDを取得
	buildJobID := (*ctx).Param("id")
	// JWTからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サービスを呼び出してログを取得
	logs, err := service.GetBuildJobLogs(buildJobID, userID)

	if err != nil {
		switch err {
		case service.ErrBuildJobNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "ビルドジョブが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	// レスポンスを返す
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"log": logs,
		},
	})
}
