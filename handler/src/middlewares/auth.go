package middlewares

import (
	"handler/logger"
	"handler/repository"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

var cliTokenRepo repository.CliTokenRepository // CLIトークンの有効性照合に使うリポジトリ

// SetCliTokenRepository は RequireAuth が使う CLIトークン照合用リポジトリを設定する
func SetCliTokenRepository(repo repository.CliTokenRepository) {
	cliTokenRepo = repo
}

// 認証ミドルウェア
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// ヘッダからトークンを取得
		token := ctx.Request().Header.Get("Authorization")
		if token == "" {
			return ctx.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
		}

		// kidがCLIトークンを示す場合はCLI用の検証パスを通す
		if IsCliTokenHeader(token) {
			return requireCliAuth(ctx, next, token)
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

// requireCliAuth はCLIトークンの署名検証とDBでの有効性照合を行う
func requireCliAuth(ctx echo.Context, next echo.HandlerFunc, token string) error {
	// CLIトークンの署名を検証する
	claim, err := ValidateCliToken(token)
	if err != nil {
		logger.PrintErr(err)
		return ctx.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	// DBでCLIトークンの存在（失効されていないか）・有効期限を照合する
	// 失効済みトークンはレコードごと削除されるため、FindByIDのエラーがそのまま失効判定になる
	cliTokenRecord, err := cliTokenRepo.FindByID(ctx.Request().Context(), claim.ID)
	if err != nil {
		logger.PrintErr(err)
		return ctx.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}
	if cliTokenRecord.ExpiresAt != nil && cliTokenRecord.ExpiresAt.Before(time.Now()) {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	// contextにトークンを格納
	ctx.Set("claim", AccessTokenClaim{UserID: claim.UserID})
	// トークンを格納
	ctx.Set("token", token)
	// ユーザーIDを格納
	ctx.Set("UserID", claim.UserID)
	// CLIトークン経由のリクエストであることを格納する（発行API等の鶏卵問題回避に使う）
	ctx.Set("isCliToken", true)

	return next(ctx)
}
