package handler

import (
	"handler/logger"
	"handler/service"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ProjectHandler は Project CRUD の HTTP ハンドラーを提供する
type ProjectHandler struct {
	projectService service.ProjectService // project サービスのインターフェース
}

// NewProjectHandler は ProjectHandler を生成して返す
func NewProjectHandler(projectService service.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService, // 依存を注入する
	}
}

// ListProjects は GET /api/v1/projects のハンドラー
func (handler *ProjectHandler) ListProjects(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	projectList, err := handler.projectService.ListProjects(echoCtx.Request().Context(), userID) // サービスを呼び出して一覧を取得する
	if err != nil {
		logger.PrintHandlerError("ProjectHandler", "ListProjects", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, projectList) // project 一覧を返す
}

// CreateProject は POST /api/v1/projects のハンドラー
func (handler *ProjectHandler) CreateProject(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する

	var requestBody service.CreateProjectRequest                   // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		logger.PrintHandlerError("ProjectHandler", "CreateProject", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	if requestBody.Name == "" { // 必須フィールドのバリデーションを行う
		logger.PrintHandlerError("ProjectHandler", "CreateProject", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("バリデーションエラー: name は必須です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "name は必須です",
		})
	}

	projectData, err := handler.projectService.CreateProject(echoCtx.Request().Context(), userID, requestBody) // サービスを呼び出して project を作成する
	if err != nil {
		var quotaErr *service.QuotaExceededError
		if errors.As(err, &quotaErr) { // quota 超過の場合は 403 と詳細情報を返す
			logger.PrintHandlerError("ProjectHandler", "CreateProject", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]interface{}{
				"error":    "quota_exceeded",
				"resource": quotaErr.Resource,
				"current":  quotaErr.Current,
				"limit":    quotaErr.Limit,
			})
		}
		logger.PrintHandlerError("ProjectHandler", "CreateProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, projectData) // 作成した project を返す
}

// GetProject は GET /api/v1/projects/:id のハンドラー
func (handler *ProjectHandler) GetProject(echoCtx echo.Context) error {
	projectID := echoCtx.Param("id") // パスパラメータから project ID を取得する

	projectData, err := handler.projectService.GetProject(echoCtx.Request().Context(), projectID) // サービスを呼び出して project を取得する
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("ProjectHandler", "GetProject", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ProjectHandler", "GetProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, projectData) // project を返す
}

// UpdateProject は PUT /api/v1/projects/:id のハンドラー
func (handler *ProjectHandler) UpdateProject(echoCtx echo.Context) error {
	projectID := echoCtx.Param("id") // パスパラメータから project ID を取得する

	var requestBody service.UpdateProjectRequest                   // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		logger.PrintHandlerError("ProjectHandler", "UpdateProject", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	projectData, err := handler.projectService.UpdateProject(echoCtx.Request().Context(), projectID, requestBody) // サービスを呼び出して project を更新する
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("ProjectHandler", "UpdateProject", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ProjectHandler", "UpdateProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, projectData) // 更新後の project を返す
}

// DeleteProject は DELETE /api/v1/projects/:id のハンドラー
func (handler *ProjectHandler) DeleteProject(echoCtx echo.Context) error {
	projectID := echoCtx.Param("id") // パスパラメータから project ID を取得する

	err := handler.projectService.DeleteProject(echoCtx.Request().Context(), projectID) // サービスを呼び出して project を削除する
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("ProjectHandler", "DeleteProject", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ProjectHandler", "DeleteProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}

// GetProjectQuota は GET /api/v1/projects/:id/quota のハンドラー
func (handler *ProjectHandler) GetProjectQuota(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")        // パスパラメータから project ID を取得する

	quotaData, err := handler.projectService.GetProjectQuota(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出してクォータを取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("ProjectHandler", "GetProjectQuota", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("ProjectHandler", "GetProjectQuota", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ProjectHandler", "GetProjectQuota", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, quotaData) // クォータ情報を返す
}
