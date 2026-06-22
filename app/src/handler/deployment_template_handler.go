package handler

import (
	"app/logger"
	"app/service"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// DeploymentTemplateHandler はテンプレート CRUD と from-template エンドポイントを提供する
type DeploymentTemplateHandler struct {
	templateService service.DeploymentTemplateService // テンプレートサービスのインターフェース
}

// NewDeploymentTemplateHandler は DeploymentTemplateHandler を生成して返す
func NewDeploymentTemplateHandler(templateService service.DeploymentTemplateService) *DeploymentTemplateHandler {
	return &DeploymentTemplateHandler{
		templateService: templateService, // 依存を注入する
	}
}

// ListTemplates は GET /deployment-templates のハンドラー
func (templateHandler *DeploymentTemplateHandler) ListTemplates(echoCtx echo.Context) error {
	templateList, err := templateHandler.templateService.ListTemplates(echoCtx.Request().Context()) // サービスを呼び出して一覧を取得する
	if err != nil {                                                                                   // エラーが発生した場合
		logger.PrintHandlerError("DeploymentTemplateHandler", "ListTemplates", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, templateList) // テンプレート一覧を返す
}

// GetTemplate は GET /deployment-templates/:id のハンドラー
func (templateHandler *DeploymentTemplateHandler) GetTemplate(echoCtx echo.Context) error {
	templateID := echoCtx.Param("id") // パスパラメータからテンプレート ID を取得する

	templateData, err := templateHandler.templateService.GetTemplate(echoCtx.Request().Context(), templateID) // サービスを呼び出してテンプレートを取得する
	if err != nil {                                                                                             // エラーが発生した場合
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "テンプレートが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentTemplateHandler", "GetTemplate", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, templateData) // テンプレートを返す
}

// CreateTemplate は POST /deployment-templates のハンドラー（管理者専用）
func (templateHandler *DeploymentTemplateHandler) CreateTemplate(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	var requestBody service.CreateTemplateRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("DeploymentTemplateHandler", "CreateTemplate", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	templateData, err := templateHandler.templateService.CreateTemplate(echoCtx.Request().Context(), userID, requestBody) // サービスを呼び出してテンプレートを作成する
	if err != nil {                                                                                                         // エラーが発生した場合
		logger.PrintHandlerError("DeploymentTemplateHandler", "CreateTemplate", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, templateData) // 作成したテンプレートを返す
}

// UpdateTemplate は PUT /deployment-templates/:id のハンドラー（管理者専用）
func (templateHandler *DeploymentTemplateHandler) UpdateTemplate(echoCtx echo.Context) error {
	templateID := echoCtx.Param("id") // パスパラメータからテンプレート ID を取得する

	var requestBody service.UpdateTemplateRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("DeploymentTemplateHandler", "UpdateTemplate", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	templateData, err := templateHandler.templateService.UpdateTemplate(echoCtx.Request().Context(), templateID, requestBody) // サービスを呼び出してテンプレートを更新する
	if err != nil {                                                                                                             // エラーが発生した場合
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "テンプレートが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentTemplateHandler", "UpdateTemplate", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, templateData) // 更新したテンプレートを返す
}

// DeleteTemplate は DELETE /deployment-templates/:id のハンドラー（管理者専用）
func (templateHandler *DeploymentTemplateHandler) DeleteTemplate(echoCtx echo.Context) error {
	templateID := echoCtx.Param("id") // パスパラメータからテンプレート ID を取得する

	if err := templateHandler.templateService.DeleteTemplate(echoCtx.Request().Context(), templateID); err != nil { // サービスを呼び出してテンプレートを削除する
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "テンプレートが見つかりません",
			})
		}
		logger.PrintHandlerError("DeploymentTemplateHandler", "DeleteTemplate", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusNoContent, nil) // 削除成功を返す
}

// CreateDeploymentFromTemplate は POST /projects/:id/deployments/from-template のハンドラー
func (templateHandler *DeploymentTemplateHandler) CreateDeploymentFromTemplate(echoCtx echo.Context) error {
	projectID := echoCtx.Param("id")          // パスパラメータからプロジェクト ID を取得する
	userID := echoCtx.Get("UserID").(string)  // ミドルウェアがセットした UserID を取得する

	var requestBody service.CreateDeploymentFromTemplateRequest // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {          // リクエストをバインドする
		logger.PrintHandlerError("DeploymentTemplateHandler", "CreateDeploymentFromTemplate", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	requestBody.ProjectID = projectID // パスパラメータのプロジェクト ID をセットする

	deploymentData, err := templateHandler.templateService.CreateDeploymentFromTemplate(echoCtx.Request().Context(), userID, requestBody) // サービスを呼び出してデプロイメントを作成する
	if err != nil {                                                                                                                         // エラーが発生した場合
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "テンプレートまたはプロジェクトが見つかりません",
			})
		}
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("DeploymentTemplateHandler", "CreateDeploymentFromTemplate", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		var quotaErr *service.QuotaExceededError
		if errors.As(err, &quotaErr) { // quota 超過の場合は 403 と詳細情報を返す
			logger.PrintHandlerError("DeploymentTemplateHandler", "CreateDeploymentFromTemplate", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]interface{}{
				"error":    "quota_exceeded",
				"resource": quotaErr.Resource,
				"current":  quotaErr.Current,
				"limit":    quotaErr.Limit,
			})
		}
		logger.PrintHandlerError("DeploymentTemplateHandler", "CreateDeploymentFromTemplate", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, deploymentData) // 作成したデプロイメントを返す
}
