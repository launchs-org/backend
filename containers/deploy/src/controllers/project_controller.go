package controllers

import (
	"net/http"

	"deploy/services"

	"github.com/labstack/echo/v4"
)

// ProjectController プロジェクト関連のリクエストを処理するコントローラー
type ProjectController struct {
	projectService services.ProjectService
}

// NewProjectController ProjectController の新しいインスタンスを作成する
func NewProjectController(service services.ProjectService) *ProjectController {
	return &ProjectController{
		projectService: service,
	}
}

// CreateProject プロジェクトを作成するエンドポイント
func (controller *ProjectController) CreateProject(echoContext echo.Context) error {
	// リクエストのパースとサービスの呼び出し
	return echoContext.JSON(http.StatusCreated, echo.Map{"message": "project creation logic here"})
}

// GetProject プロジェクトを取得するエンドポイント
func (controller *ProjectController) GetProject(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get project logic here"})
}

// ListProjects プロジェクト一覧を取得するエンドポイント
func (controller *ProjectController) ListProjects(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "list projects logic here"})
}

// DeleteProject プロジェクトを削除するエンドポイント
func (controller *ProjectController) DeleteProject(echoContext echo.Context) error {
	return echoContext.NoContent(http.StatusAccepted)
}
