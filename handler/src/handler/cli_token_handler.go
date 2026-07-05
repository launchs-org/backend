package handler

import (
	"errors"
	"handler/logger"
	"handler/service"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CliTokenHandler は CLIトークンの発行・一覧・失効の HTTP ハンドラーを提供する
type CliTokenHandler struct {
	cliTokenService service.CliTokenService // CLIトークンサービスのインターフェース
}

// NewCliTokenHandler は CliTokenHandler を生成して返す
func NewCliTokenHandler(cliTokenService service.CliTokenService) *CliTokenHandler {
	return &CliTokenHandler{
		cliTokenService: cliTokenService, // 依存を注入する
	}
}

// CreateCliTokenRequest は POST /api/v1/cli-tokens のリクエストボディ
type CreateCliTokenRequest struct {
	Name          string `json:"name"`            // トークンの用途ラベル
	ExpiresInDays int    `json:"expires_in_days"` // 有効期限（日数）。0または未指定の場合は無期限
}

// CreateCliTokenResponse は POST /api/v1/cli-tokens のレスポンス
type CreateCliTokenResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"` // 平文トークン。発行時のみ返却する
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateCliToken は POST /api/v1/cli-tokens のハンドラー
func (cliTokenHandler *CliTokenHandler) CreateCliToken(echoCtx echo.Context) error {
	// CLIトークン自身でのCLIトークン発行は鶏卵問題になるため拒否する
	if isCliToken, ok := echoCtx.Get("isCliToken").(bool); ok && isCliToken {
		logger.PrintHandlerError("CliTokenHandler", "CreateCliToken", echoCtx.Request().URL.Path, http.StatusForbidden, errors.New("CLIトークンによるCLIトークン発行は許可されていません"))
		return echoCtx.JSON(http.StatusForbidden, map[string]string{
			"error": "CLIトークンを使ってCLIトークンを発行することはできません",
		})
	}

	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	var requestBody CreateCliTokenRequest              // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil { // リクエストをバインドする
		logger.PrintHandlerError("CliTokenHandler", "CreateCliToken", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	var expiresAt *time.Time
	if requestBody.ExpiresInDays > 0 { // 日数指定がある場合のみ有効期限を設定する（0は無期限）
		expiry := time.Now().AddDate(0, 0, requestBody.ExpiresInDays)
		expiresAt = &expiry
	}

	cliTokenData, plainToken, err := cliTokenHandler.cliTokenService.IssueToken( // サービスを呼び出してCLIトークンを発行する
		echoCtx.Request().Context(),
		userID,
		requestBody.Name,
		expiresAt,
	)
	if err != nil { // エラーが発生した場合
		logger.PrintHandlerError("CliTokenHandler", "CreateCliToken", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}

	return echoCtx.JSON(http.StatusCreated, CreateCliTokenResponse{
		ID:        cliTokenData.ID,
		Name:      cliTokenData.Name,
		Token:     plainToken,
		ExpiresAt: cliTokenData.ExpiresAt,
		CreatedAt: cliTokenData.CreatedAt,
	})
}

// ListCliTokens は GET /api/v1/cli-tokens のハンドラー
func (cliTokenHandler *CliTokenHandler) ListCliTokens(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	cliTokenList, err := cliTokenHandler.cliTokenService.ListTokens( // サービスを呼び出して一覧を取得する
		echoCtx.Request().Context(),
		userID,
	)
	if err != nil { // エラーが発生した場合
		logger.PrintHandlerError("CliTokenHandler", "ListCliTokens", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, cliTokenList) // 一覧を返す（平文トークンは含まれない）
}

// DeleteCliToken は DELETE /api/v1/cli-tokens/:id のハンドラー（削除により失効とする）
func (cliTokenHandler *CliTokenHandler) DeleteCliToken(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	cliTokenID := echoCtx.Param("id")        // パスパラメータから CLIトークン ID を取得する

	err := cliTokenHandler.cliTokenService.DeleteToken( // サービスを呼び出して削除する
		echoCtx.Request().Context(),
		userID,
		cliTokenID,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			logger.PrintHandlerError("CliTokenHandler", "DeleteCliToken", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			logger.PrintHandlerError("CliTokenHandler", "DeleteCliToken", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("CliTokenHandler", "DeleteCliToken", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{
		"message": "削除しました",
	})
}
