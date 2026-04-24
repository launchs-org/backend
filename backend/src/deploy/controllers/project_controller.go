package controllers

import (
	"backend/deploy/dto"
	"backend/deploy/services"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ProjectController はプロジェクト関連の HTTP リクエストを処理します
type ProjectController struct {
	projectService *services.ProjectService
}

// NewProjectController は ProjectController の新しいインスタンスを作成します
func NewProjectController(service *services.ProjectService) *ProjectController {
	return &ProjectController{
		projectService: service,
	}
}

// GetAllProjects はログインユーザーのプロジェクト一覧を返します
// GET /v1/projects
func (controller *ProjectController) GetAllProjects(ctx *echo.Context) error {
	userID := (*ctx).Get("UserID").(string)
	projects, err := controller.projectService.GetAllProjects(userID)
	if err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"message": "プロジェクト一覧の取得に失敗しました"})
	}
	return (*ctx).JSON(http.StatusOK, projects)
}

// CreateProject は新しいプロジェクトを作成します
// POST /v1/projects
func (controller *ProjectController) CreateProject(ctx *echo.Context) error {
	userID := (*ctx).Get("UserID").(string)
	var req dto.ProjectCreateRequest
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": "リクエストボディが不正です"})
	}

	project, err := controller.projectService.CreateProject(req, userID)
	if err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"message": "プロジェクトの作成に失敗しました"})
	}

	return (*ctx).JSON(http.StatusCreated, project)
}

// GetProject は指定されたプロジェクトの詳細を返します
// GET /v1/projects/:id
func (controller *ProjectController) GetProject(ctx *echo.Context) error {
	userID := (*ctx).Get("UserID").(string)
	projectID := (*ctx).Param("id")

	project, err := controller.projectService.GetProjectByID(projectID, userID)
	if err != nil {
		return (*ctx).JSON(http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "プロジェクトが見つかりません"})
	}

	return (*ctx).JSON(http.StatusOK, project)
}

// DeleteProject はプロジェクトを削除します
// DELETE /v1/projects/:id
func (controller *ProjectController) DeleteProject(ctx *echo.Context) error {
	userID := (*ctx).Get("UserID").(string)
	projectID := (*ctx).Param("id")

	if err := controller.projectService.DeleteProject(projectID, userID); err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"message": "プロジェクトの削除に失敗しました"})
	}

	return (*ctx).NoContent(http.StatusNoContent)
}
