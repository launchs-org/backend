package handler

import (
	"app/logger"
	"app/service"
	"net/http"

	"github.com/labstack/echo/v4"
)

// UserQuotaHandler は quota 取得・更新のHTTPハンドラーを提供する
type UserQuotaHandler struct {
	quotaService service.QuotaService // quota サービスのインターフェース
}

// NewUserQuotaHandler は UserQuotaHandler を生成して返す
func NewUserQuotaHandler(quotaService service.QuotaService) *UserQuotaHandler {
	return &UserQuotaHandler{
		quotaService: quotaService, // 依存を注入する
	}
}

// GetQuota は GET /api/v1/users/:user_id/quota のハンドラー
func (handler *UserQuotaHandler) GetQuota(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	quotaResponse, err := handler.quotaService.GetQuota(echoCtx.Request().Context(), userID) // サービスを呼び出して quota を取得する
	if err != nil {
		logger.PrintHandlerError("UserQuotaHandler", "GetQuota", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー", // 500 エラーメッセージ
		})
	}

	return echoCtx.JSON(http.StatusOK, quotaResponse) // quota レスポンスを返す
}

// UpdateQuota は PUT /api/v1/users/:user_id/quota のハンドラー
func (handler *UserQuotaHandler) UpdateQuota(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	var requestBody service.UpdateQuotaRequest                      // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {             // リクエストをバインドする
		logger.PrintHandlerError("UserQuotaHandler", "UpdateQuota", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です", // バインドエラーメッセージ
		})
	}

	quotaResponse, err := handler.quotaService.UpdateQuota(echoCtx.Request().Context(), userID, requestBody) // サービスを呼び出して quota を更新する
	if err != nil {
		logger.PrintHandlerError("UserQuotaHandler", "UpdateQuota", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー", // 500 エラーメッセージ
		})
	}

	return echoCtx.JSON(http.StatusOK, quotaResponse) // 更新後の quota レスポンスを返す
}
