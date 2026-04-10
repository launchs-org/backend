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

// CreateProject プロジェクト作成リクエストを受け取り、作成されたプロジェクト情報を返します。
func (controller *ProjectController) CreateProject(echoContext echo.Context) error {
	// リクエストのパースとサービスの呼び出し
	return echoContext.JSON(http.StatusCreated, echo.Map{"message": "project creation logic here"})
}

// GetProject プロジェクト ID に基づいて、プロジェクトの詳細情報を返します。
func (controller *ProjectController) GetProject(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "get project logic here"})
}

// ListProjects オーナーに紐付いた全プロジェクトの一覧を返します。
func (controller *ProjectController) ListProjects(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "list projects logic here"})
}

// DeleteProject プロジェクトを削除し、関連するリソースの破棄プロセスを開始します。
func (controller *ProjectController) DeleteProject(echoContext echo.Context) error {
	return echoContext.NoContent(http.StatusAccepted)
}
