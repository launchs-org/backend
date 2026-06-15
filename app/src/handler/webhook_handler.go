package handler

import (
	"app/middlewares"
	"app/service"
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// WebhookHandler は Webhook CRUD の HTTP ハンドラーを提供する
type WebhookHandler struct {
	webhookService service.WebhookService // webhook サービスのインターフェース
}

// NewWebhookHandler は WebhookHandler を生成して返す
func NewWebhookHandler(webhookService service.WebhookService) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService, // 依存を注入する
	}
}

// CreateWebhook は POST /api/v1/deployments/:id/webhooks のハンドラー
func (webhookHandler *WebhookHandler) CreateWebhook(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	deploymentID := echoCtx.Param("id")                               // パスパラメータから deployment ID を取得する

	var requestBody service.CreateWebhookRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {      // リクエストをバインドする
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	webhookData, err := webhookHandler.webhookService.CreateWebhook( // サービスを呼び出して webhook を作成する
		echoCtx.Request().Context(),
		userClaim.UserID,
		deploymentID,
		requestBody,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, webhookData) // 作成した webhook を返す
}

// GetWebhook は GET /api/v1/deployments/:id/webhooks のハンドラー
func (webhookHandler *WebhookHandler) GetWebhook(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	deploymentID := echoCtx.Param("id")                               // パスパラメータから deployment ID を取得する

	webhookData, err := webhookHandler.webhookService.GetWebhook( // サービスを呼び出して webhook を取得する
		echoCtx.Request().Context(),
		userClaim.UserID,
		deploymentID,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, webhookData) // webhook を返す
}

// DeleteWebhook は DELETE /api/v1/webhooks/:id のハンドラー
func (webhookHandler *WebhookHandler) DeleteWebhook(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	webhookID := echoCtx.Param("id")                                   // パスパラメータから webhook ID を取得する

	err := webhookHandler.webhookService.DeleteWebhook( // サービスを呼び出して webhook を削除する
		echoCtx.Request().Context(),
		userClaim.UserID,
		webhookID,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{
		"message": "削除しました",
	}) // 削除成功を返す
}

// ReceiveGithubWebhook は POST /webhooks/:deployment_id/github のハンドラー
func (webhookHandler *WebhookHandler) ReceiveGithubWebhook(echoCtx echo.Context) error {
	deploymentID := echoCtx.Param("deployment_id")             // パスパラメータから deployment ID を取得する
	signature := echoCtx.Request().Header.Get("X-Hub-Signature-256") // HMAC 署名ヘッダーを取得する

	body, err := io.ReadAll(echoCtx.Request().Body) // リクエストボディを読み込む
	if err != nil {
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストボディの読み込みに失敗しました",
		})
	}

	if err := webhookHandler.webhookService.ReceiveGithubWebhook( // サービスを呼び出して Webhook を処理する
		echoCtx.Request().Context(),
		deploymentID,
		signature,
		body,
	); err != nil {
		if errors.Is(err, service.ErrInvalidSignature) { // 署名不正の場合
			return echoCtx.JSON(http.StatusUnauthorized, map[string]string{
				"error": "署名が不正です",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{
		"message": "Webhook を受信しました",
	}) // 受信成功を返す
}
