package handler

import (
	"app/logger"
	"app/service"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)


// DeploymentHandler は Deployment CRUD の HTTP ハンドラーを提供する
type DeploymentHandler struct {
	deploymentService service.DeploymentService        // deployment サービスのインターフェース
	applyService      service.ApplyServiceInterface    // apply サービスのインターフェース
}

// NewDeploymentHandler は DeploymentHandler を生成して返す
func NewDeploymentHandler(deploymentService service.DeploymentService, applyService service.ApplyServiceInterface) *DeploymentHandler {
	return &DeploymentHandler{
		deploymentService: deploymentService, // 依存を注入する
		applyService:      applyService,      // apply サービスを注入する
	}
}

// ListDeployments は GET /projects/:id/deployments のハンドラー
func (deploymentHandler *DeploymentHandler) ListDeployments(echoCtx echo.Context) error {
	projectID := echoCtx.Param("id") // パスパラメータから project ID を取得する

	deploymentList, err := deploymentHandler.deploymentService.ListDeployments(echoCtx.Request().Context(), projectID) // サービスを呼び出して一覧を取得する
	if err != nil {                                                                                                      // エラーが発生した場合
		logger.PrintHandlerError("DeploymentHandler", "ListDeployments", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, deploymentList) // deployment 一覧を返す
}

// CreateDeployment は POST /projects/:id/deployments のハンドラー
func (deploymentHandler *DeploymentHandler) CreateDeployment(echoCtx echo.Context) error {
	projectID := echoCtx.Param("id") // パスパラメータから project ID を取得する

	var requestBody service.CreateDeploymentRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {         // リクエストをバインドする
		logger.PrintHandlerError("DeploymentHandler", "CreateDeployment", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	requestBody.ProjectID = projectID // パスパラメータの project ID をセットする

	deploymentData, err := deploymentHandler.deploymentService.CreateDeployment(echoCtx.Request().Context(), requestBody) // サービスを呼び出して deployment を作成する
	if err != nil {                                                                                                         // エラーが発生した場合
		var quotaErr *service.QuotaExceededError
		if errors.As(err, &quotaErr) { // quota 超過の場合は 403 と詳細情報を返す
			logger.PrintHandlerError("DeploymentHandler", "CreateDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]interface{}{
				"error":    "quota_exceeded",
				"resource": quotaErr.Resource,
				"current":  quotaErr.Current,
				"limit":    quotaErr.Limit,
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "CreateDeployment", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, deploymentData) // 作成した deployment を返す
}

// GetDeployment は GET /deployments/:id のハンドラー
func (deploymentHandler *DeploymentHandler) GetDeployment(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	deploymentData, err := deploymentHandler.deploymentService.GetDeployment(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出して deployment を取得する
	if err != nil {                                                                                                               // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "GetDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "GetDeployment", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "GetDeployment", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, deploymentData) // deployment を返す
}

// DiscardPending は POST /deployments/:id/discard-pending のハンドラー
func (deploymentHandler *DeploymentHandler) DiscardPending(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	deploymentData, err := deploymentHandler.deploymentService.DiscardPending(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出して pending をクリアする
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // deployment が見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "デプロイメントが見つかりません",
			})
		}
		if errors.Is(err, service.ErrForbidden) { // 所有権がない場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "権限がありません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "DiscardPending", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, deploymentData) // 更新後の deployment を返す
}

// UpdateDeployment は PUT /deployments/:id のハンドラー
func (deploymentHandler *DeploymentHandler) UpdateDeployment(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	var requestBody service.UpdateDeploymentRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {         // リクエストをバインドする
		logger.PrintHandlerError("DeploymentHandler", "UpdateDeployment", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	deploymentData, err := deploymentHandler.deploymentService.UpdateDeployment(echoCtx.Request().Context(), userID, deploymentID, requestBody) // サービスを呼び出して deployment を更新する
	if err != nil {                                                                                                                               // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "UpdateDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		var quotaErrUpdate *service.QuotaExceededError
		if errors.As(err, &quotaErrUpdate) { // quota 超過の場合は 403 と詳細情報を返す
			logger.PrintHandlerError("DeploymentHandler", "UpdateDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]interface{}{
				"error":    "quota_exceeded",
				"resource": quotaErrUpdate.Resource,
				"current":  quotaErrUpdate.Current,
				"limit":    quotaErrUpdate.Limit,
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "UpdateDeployment", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "UpdateDeployment", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, deploymentData) // 更新後の deployment を返す
}

// DeleteDeployment は DELETE /deployments/:id のハンドラー
func (deploymentHandler *DeploymentHandler) DeleteDeployment(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	deploymentData, err := deploymentHandler.deploymentService.DeleteDeployment(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出して deployment を削除する
	if err != nil {                                                                                                                   // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "DeleteDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "DeleteDeployment", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "DeleteDeployment", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, deploymentData) // 更新後の deployment を返す
}

// ListApplyHistories は GET /deployments/:id/apply-histories のハンドラー
func (deploymentHandler *DeploymentHandler) ListApplyHistories(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	historyList, err := deploymentHandler.applyService.ListApplyHistories(echoCtx.Request().Context(), userID, deploymentID) // apply 履歴一覧を取得する
	if err != nil {                                                                                                           // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "ListApplyHistories", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // deployment が存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "ListApplyHistories", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "ListApplyHistories", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, historyList) // 履歴一覧を返す
}

// CreateService は POST /deployments/:id/service のハンドラー
func (deploymentHandler *DeploymentHandler) CreateService(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	var requestBody service.CreateServiceRequest        // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil { // リクエストをバインドする
		logger.PrintHandlerError("DeploymentHandler", "CreateService", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	serviceData, err := deploymentHandler.deploymentService.CreateService(echoCtx.Request().Context(), userID, deploymentID, requestBody) // サービスを呼び出して service を作成する
	if err != nil {                                                                                                                         // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "CreateService", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "CreateService", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "CreateService", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, serviceData) // 作成した service を返す
}

// GetService は GET /deployments/:id/service のハンドラー
func (deploymentHandler *DeploymentHandler) GetService(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	serviceData, err := deploymentHandler.deploymentService.GetService(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出して service 設定を取得する
	if err != nil {                                                                                                         // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "GetService", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "GetService", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "GetService", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, serviceData) // service 設定を返す
}

// UpdateService は PUT /deployments/:id/service のハンドラー
func (deploymentHandler *DeploymentHandler) UpdateService(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	var requestBody service.UpdateServiceRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {      // リクエストをバインドする
		logger.PrintHandlerError("DeploymentHandler", "UpdateService", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	serviceData, err := deploymentHandler.deploymentService.UpdateService(echoCtx.Request().Context(), userID, deploymentID, requestBody) // サービスを呼び出して service を更新する
	if err != nil {                                                                                                                         // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "UpdateService", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "UpdateService", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "UpdateService", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, serviceData) // 更新後の service を返す
}

// DeleteService は DELETE /deployments/:id/service のハンドラー
func (deploymentHandler *DeploymentHandler) DeleteService(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	if err := deploymentHandler.deploymentService.DeleteService(echoCtx.Request().Context(), userID, deploymentID); err != nil { // サービスを呼び出して service を削除する
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "DeleteService", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "DeleteService", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		logger.PrintHandlerError("DeploymentHandler", "DeleteService", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 204 No Content を返す
}

// ApplyDeployment は POST /deployments/:id/apply のハンドラー
func (deploymentHandler *DeploymentHandler) ApplyDeployment(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	applyResult, err := deploymentHandler.applyService.Apply(echoCtx.Request().Context(), userID, deploymentID) // apply サービスを呼び出す
	if err != nil {                                                                                               // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentHandler", "ApplyDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが禁止されています",
			})
		}
		if errors.Is(err, service.ErrAlreadyApplying) { // apply 中の場合は 409 を返す
			logger.PrintHandlerError("DeploymentHandler", "ApplyDeployment", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "apply が実行中です",
			})
		}
		var quotaErrApply *service.QuotaExceededError
		if errors.As(err, &quotaErrApply) { // quota 超過の場合は 403 と詳細情報を返す
			logger.PrintHandlerError("DeploymentHandler", "ApplyDeployment", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]interface{}{
				"error":    "quota_exceeded",
				"resource": quotaErrApply.Resource,
				"current":  quotaErrApply.Current,
				"limit":    quotaErrApply.Limit,
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // deployment が存在しない場合は 404 を返す
			logger.PrintHandlerError("DeploymentHandler", "ApplyDeployment", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentHandler", "ApplyDeployment", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return echoCtx.JSON(http.StatusOK, applyResult) // apply 結果を返す
}
