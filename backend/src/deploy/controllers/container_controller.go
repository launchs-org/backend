package controllers

import (
	"backend/deploy/dto"
	"backend/deploy/services"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ContainerController はコンテナ関連の HTTP リクエストを処理します
type ContainerController struct {
	containerService *services.ContainerService
}

// NewContainerController は ContainerController の新しいインスタンスを作成します
func NewContainerController(service *services.ContainerService) *ContainerController {
	return &ContainerController{
		containerService: service,
	}
}

// CreateContainer はプロジェクト内に新しいコンテナを作成します
// POST /v1/projects/:id/containers
func (controller *ContainerController) CreateContainer(ctx *echo.Context) error {
	projectID := (*ctx).Param("id")
	var req dto.ContainerCreateRequest
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": "リクエストボディが不正です"})
	}

	container, err := controller.containerService.CreateContainer(projectID, req)
	if err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"message": "コンテナの作成に失敗しました"})
	}

	return (*ctx).JSON(http.StatusCreated, container)
}

// UpdateContainer はコンテナ情報を更新します
// PATCH /v1/containers/:id
func (controller *ContainerController) UpdateContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	var updates map[string]interface{}
	if err := (*ctx).Bind(&updates); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": "リクエストボディが不正です"})
	}

	if err := controller.containerService.UpdateContainer(containerID, updates); err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"message": "コンテナの更新に失敗しました"})
	}

	return (*ctx).NoContent(http.StatusOK)
}

// DeleteContainer はコンテナを削除します
// DELETE /v1/containers/:id
func (controller *ContainerController) DeleteContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")

	if err := controller.containerService.DeleteContainer(containerID); err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"message": "コンテナの削除に失敗しました"})
	}

	return (*ctx).NoContent(http.StatusNoContent)
}

// GetContainer はコンテナ詳細を返します
// GET /v1/containers/:id
func (controller *ContainerController) GetContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	container, err := controller.containerService.GetContainerByID(containerID)
	if err != nil {
		return (*ctx).JSON(http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "コンテナが見つかりません"})
	}

	return (*ctx).JSON(http.StatusOK, container)
}
