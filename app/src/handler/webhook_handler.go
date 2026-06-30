package handler

import (
	"app/logger"
	"app/service"
	"errors"
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
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	var requestBody service.CreateWebhookRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {      // リクエストをバインドする
		logger.PrintHandlerError("WebhookHandler", "CreateWebhook", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	webhookData, err := webhookHandler.webhookService.CreateWebhook( // サービスを呼び出して webhook を作成する
		echoCtx.Request().Context(),
		userID,
		deploymentID,
		requestBody,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			logger.PrintHandlerError("WebhookHandler", "CreateWebhook", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			logger.PrintHandlerError("WebhookHandler", "CreateWebhook", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("WebhookHandler", "CreateWebhook", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, webhookData) // 作成した webhook を返す
}

// GetWebhook は GET /api/v1/deployments/:id/webhooks のハンドラー
func (webhookHandler *WebhookHandler) GetWebhook(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	webhookData, err := webhookHandler.webhookService.GetWebhook( // サービスを呼び出して webhook を取得する
		echoCtx.Request().Context(),
		userID,
		deploymentID,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			logger.PrintHandlerError("WebhookHandler", "GetWebhook", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			logger.PrintHandlerError("WebhookHandler", "GetWebhook", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("WebhookHandler", "GetWebhook", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, webhookData) // webhook を返す
}

// DeleteWebhook は DELETE /api/v1/webhooks/:id のハンドラー
func (webhookHandler *WebhookHandler) DeleteWebhook(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	webhookID := echoCtx.Param("id")         // パスパラメータから webhook ID を取得する

	err := webhookHandler.webhookService.DeleteWebhook( // サービスを呼び出して webhook を削除する
		echoCtx.Request().Context(),
		userID,
		webhookID,
	)
	if err != nil { // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 認可エラーの場合
			logger.PrintHandlerError("WebhookHandler", "DeleteWebhook", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
			logger.PrintHandlerError("WebhookHandler", "DeleteWebhook", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("WebhookHandler", "DeleteWebhook", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{
		"message": "削除しました",
	}) // 削除成功を返す
}

// WebhookTriggerBuildRequest は POST /webhooks/:deployment_id/build のリクエストボディ
type WebhookTriggerBuildRequest struct {
	CommitMessage string `json:"commit_message"` // コミットメッセージ（オプション）
	Author        string `json:"author"`         // コミット著者（オプション）
}

// webhookAuthError は X-Webhook-Secret 認証エラーを統一して返す
func webhookAuthError(echoCtx echo.Context, methodName string, err error) error {
	if errors.Is(err, service.ErrInvalidSignature) || errors.Is(err, service.ErrWebhookInactive) { // シークレット不正または Webhook 無効の場合
		logger.PrintHandlerError("WebhookHandler", methodName, echoCtx.Request().URL.Path, http.StatusUnauthorized, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusUnauthorized, map[string]string{
			"error": "シークレットが不正です",
		})
	}
	if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合
		logger.PrintHandlerError("WebhookHandler", methodName, echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusNotFound, map[string]string{
			"error": "リソースが見つかりません",
		})
	}
	if errors.Is(err, service.ErrBuildConflict) { // ビルド中の場合
		logger.PrintHandlerError("WebhookHandler", methodName, echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusConflict, map[string]string{
			"error": "ビルドが既に進行中です",
		})
	}
	if errors.Is(err, service.ErrNotInitialized) { // not_init 状態の場合
		logger.PrintHandlerError("WebhookHandler", methodName, echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusConflict, map[string]string{
			"error": "初回ビルドが未完了のため操作できません",
		})
	}
	if errors.Is(err, service.ErrAlreadyApplying) { // Apply 中の場合
		logger.PrintHandlerError("WebhookHandler", methodName, echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusConflict, map[string]string{
			"error": "Apply が既に進行中です",
		})
	}
	logger.PrintHandlerError("WebhookHandler", methodName, echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
	return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
		"error": "内部サーバーエラー",
	})
}

// TriggerBuildByWebhook は POST /webhooks/:deployment_id/build のハンドラー
func (webhookHandler *WebhookHandler) TriggerBuildByWebhook(echoCtx echo.Context) error {
	deploymentID := echoCtx.Param("deployment_id")                    // パスパラメータから deployment ID を取得する
	secret := echoCtx.Request().Header.Get("X-Webhook-Secret")        // シークレットヘッダーを取得する
	if secret == "" {                                                   // シークレットが未指定の場合は認証エラーを返す
		return echoCtx.JSON(http.StatusUnauthorized, map[string]string{
			"error": "X-Webhook-Secret ヘッダーが必要です",
		})
	}

	var requestBody WebhookTriggerBuildRequest                    // リクエストボディの構造体を定義する
	_ = echoCtx.Bind(&requestBody)                               // バインド失敗は無視してオプション扱いにする

	buildData, err := webhookHandler.webhookService.TriggerBuildByWebhook( // サービスを呼び出してビルドをトリガーする
		echoCtx.Request().Context(),
		deploymentID,
		secret,
		requestBody.CommitMessage,
		requestBody.Author,
	)
	if err != nil { // エラーが発生した場合
		return webhookAuthError(echoCtx, "TriggerBuildByWebhook", err) // エラーレスポンスを返す
	}
	return echoCtx.JSON(http.StatusCreated, buildData) // 作成したビルドレコードを返す
}

// GetBuildByWebhook は GET /webhooks/:deployment_id/builds/:build_id のハンドラー
func (webhookHandler *WebhookHandler) GetBuildByWebhook(echoCtx echo.Context) error {
	deploymentID := echoCtx.Param("deployment_id")             // パスパラメータから deployment ID を取得する
	buildID := echoCtx.Param("build_id")                       // パスパラメータからビルド ID を取得する
	secret := echoCtx.Request().Header.Get("X-Webhook-Secret") // シークレットヘッダーを取得する
	if secret == "" {                                            // シークレットが未指定の場合は認証エラーを返す
		return echoCtx.JSON(http.StatusUnauthorized, map[string]string{
			"error": "X-Webhook-Secret ヘッダーが必要です",
		})
	}

	buildData, err := webhookHandler.webhookService.GetBuildByWebhook( // サービスを呼び出してビルド状態を取得する
		echoCtx.Request().Context(),
		deploymentID,
		secret,
		buildID,
	)
	if err != nil { // エラーが発生した場合
		return webhookAuthError(echoCtx, "GetBuildByWebhook", err) // エラーレスポンスを返す
	}
	return echoCtx.JSON(http.StatusOK, buildData) // ビルドレコードを返す
}

// ApplyByWebhook は POST /webhooks/:deployment_id/apply のハンドラー
func (webhookHandler *WebhookHandler) ApplyByWebhook(echoCtx echo.Context) error {
	deploymentID := echoCtx.Param("deployment_id")             // パスパラメータから deployment ID を取得する
	secret := echoCtx.Request().Header.Get("X-Webhook-Secret") // シークレットヘッダーを取得する
	if secret == "" {                                            // シークレットが未指定の場合は認証エラーを返す
		return echoCtx.JSON(http.StatusUnauthorized, map[string]string{
			"error": "X-Webhook-Secret ヘッダーが必要です",
		})
	}

	applyResult, err := webhookHandler.webhookService.ApplyByWebhook( // サービスを呼び出して Apply を実行する
		echoCtx.Request().Context(),
		deploymentID,
		secret,
	)
	if err != nil { // エラーが発生した場合
		return webhookAuthError(echoCtx, "ApplyByWebhook", err) // エラーレスポンスを返す
	}
	return echoCtx.JSON(http.StatusOK, applyResult) // Apply 結果を返す
}

// WebhookUpdateImageRequest は POST /webhooks/:deployment_id/update-image のリクエストボディ
type WebhookUpdateImageRequest struct {
	ImageURL string `json:"image_url"` // 新しいイメージ URL
}

// UpdateImageAndApplyByWebhook は POST /webhooks/:deployment_id/update-image のハンドラー
func (webhookHandler *WebhookHandler) UpdateImageAndApplyByWebhook(echoCtx echo.Context) error {
	deploymentID := echoCtx.Param("deployment_id")             // パスパラメータから deployment ID を取得する
	secret := echoCtx.Request().Header.Get("X-Webhook-Secret") // シークレットヘッダーを取得する
	if secret == "" {                                            // シークレットが未指定の場合は認証エラーを返す
		return echoCtx.JSON(http.StatusUnauthorized, map[string]string{
			"error": "X-Webhook-Secret ヘッダーが必要です",
		})
	}

	var requestBody WebhookUpdateImageRequest                      // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	if requestBody.ImageURL == "" { // image_url が未指定の場合はエラーを返す
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "image_url は必須です",
		})
	}

	applyResult, err := webhookHandler.webhookService.UpdateImageAndApplyByWebhook( // サービスを呼び出して image_url を更新して Apply を実行する
		echoCtx.Request().Context(),
		deploymentID,
		secret,
		requestBody.ImageURL,
	)
	if err != nil { // エラーが発生した場合
		return webhookAuthError(echoCtx, "UpdateImageAndApplyByWebhook", err) // エラーレスポンスを返す
	}
	return echoCtx.JSON(http.StatusOK, applyResult) // Apply 結果を返す
}
