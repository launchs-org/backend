package middlewares

import (
	"backend/logger"
	"net/http"

	"github.com/labstack/echo/v5"
)

// 認証ミドルウェア
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx *echo.Context) error {
		// ヘッダからトークンを取得、なければ Sec-WebSocket-Protocol から取得、なければクエリパラメータから取得
		token := ctx.Request().Header.Get("Authorization")
		if token == "" {
			token = ctx.Request().Header.Get("Sec-WebSocket-Protocol")
		}
		if token == "" {
			token = ctx.QueryParam("token")
		}
		if token == "" {
			return ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		// トークンを検証
		claim, err := ValidateToken(token)

		// エラー処理
		if err != nil {
			logger.PrintErr(err)
			return ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		// contextにトークンを格納
		ctx.Set("claim", claim)
		// トークンを格納
		ctx.Set("token", token)
		// ユーザーIDを格納
		ctx.Set("UserID", claim.UserID)

		// 認証処理
		return next(ctx)
	}
}
