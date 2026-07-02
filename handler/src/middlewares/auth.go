package middlewares

import (
	"handler/logger"
	"net/http"

	"github.com/labstack/echo/v4"
)

// 認証ミドルウェア
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// ヘッダからトークンを取得
		token := ctx.Request().Header.Get("Authorization")
		if token == "" {
			return ctx.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
		}

		// トークンを検証
		claim, err := ValidateToken(token)

		// エラー処理
		if err != nil {
			logger.PrintErr(err)
			return ctx.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
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