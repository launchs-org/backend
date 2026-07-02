package middlewares

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RequireAdmin は管理者ラベルを持つユーザーのみ通過を許可するミドルウェア
func RequireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(echoCtx echo.Context) error {
		userClaim := echoCtx.Get("claim").(AccessTokenClaim) // コンテキストからクレームを取得する（値型）
		for _, label := range userClaim.Labels {             // ラベル一覧を確認する
			if label == "admin" {                            // admin ラベルがある場合は通過する
				return next(echoCtx)
			}
		}
		return echoCtx.JSON(http.StatusForbidden, map[string]string{
			"error": "管理者権限が必要です", // admin ラベルがない場合は 403 を返す
		})
	}
}
