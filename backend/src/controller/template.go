package controller

import (
	"backend/service"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ListTemplates は利用可能なテンプレート一覧を返すハンドラーです
func ListTemplates(ctx *echo.Context) error {
	templates := service.ListTemplates()
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"items": templates,
			"total": len(templates),
		},
	})
}

type DeployFromTemplateRequest struct {
	Name          string            `json:"name"`
	TemplateName  string            `json:"template_name"`
	EnvVars       map[string]string `json:"env_vars"`
	CreateService *bool             `json:"create_service"`
	VolumeSizeMB  *int              `json:"volume_size_mb"`
	EnableIngress *bool             `json:"enable_ingress"`
	Command       string            `json:"command"`
	Args          string            `json:"args"`
}

// DeployFromTemplate はテンプレートからコンテナをデプロイするハンドラーです
func DeployFromTemplate(ctx *echo.Context) error {
	var req DeployFromTemplateRequest
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	projectID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// create_service はデフォルト true
	createService := true
	if req.CreateService != nil {
		createService = *req.CreateService
	}

	enableIngress := false
	if req.EnableIngress != nil {
		enableIngress = *req.EnableIngress
	}

	res, err := service.DeployFromTemplate((*ctx).Request().Context(), service.DeployFromTemplateInput{
		ProjectID:     projectID,
		OwnerID:       userID,
		Name:          req.Name,
		TemplateName:  req.TemplateName,
		EnvVars:       req.EnvVars,
		CreateService: createService,
		VolumeSizeMB:  req.VolumeSizeMB,
		EnableIngress: enableIngress,
		Command:       req.Command,
		Args:          req.Args,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTemplateNotFound):
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "テンプレートが見つかりません",
			})
		case errors.Is(err, service.ErrProjectNotFound):
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "プロジェクトが見つかりません",
			})
		case errors.Is(err, service.ErrForbidden):
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		case errors.Is(err, service.ErrContainerAlreadyExists):
			return (*ctx).JSON(http.StatusConflict, map[string]string{
				"code":    "CONFLICT",
				"message": "コンテナ名が重複しています",
			})
		case errors.Is(err, service.ErrMissingRequiredEnvVar):
			return (*ctx).JSON(http.StatusBadRequest, map[string]string{
				"code":    "BAD_REQUEST",
				"message": fmt.Sprintf("必須の環境変数が不足しています: %v", err),
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusCreated, res)
}
