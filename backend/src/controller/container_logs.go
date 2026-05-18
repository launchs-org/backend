package controller

import (
	"backend/service"
	"net/http"

	"github.com/labstack/echo/v5"
)

// GetContainerLogs は DB に蓄積されたコンテナの実行ログをポーリング用に返します。
func GetContainerLogs(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	resp, err := service.GetContainerLogs(containerID, userID)
	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセスが拒否されました",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "ログの取得に失敗しました",
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": resp,
	})
}
