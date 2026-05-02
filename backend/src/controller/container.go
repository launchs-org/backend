package controller

import (
	"backend/service"
	"net/http"

	"github.com/labstack/echo/v5"
)

// CreateContainerRequest はコンテナ作成のリクエストボディです
type CreateContainerRequest struct {
	Name          string `json:"name"`
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Directory     string `json:"directory"`
	EnvVars       string `json:"env_vars"`
	Replicas      int    `json:"replicas"`
	Resources     string `json:"resources"`
}

// CreateContainer はコンテナを作成し、ビルドを開始するハンドラーです
func CreateContainer(ctx *echo.Context) error {
	var req CreateContainerRequest
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

	res, err := service.CreateContainer((*ctx).Request().Context(), service.CreateContainerInput{
		ProjectID:     projectID,
		OwnerID:       userID,
		Name:          req.Name,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		Directory:     req.Directory,
		EnvVars:       req.EnvVars,
		Replicas:      req.Replicas,
		Resources:     req.Resources,
	})

	if err != nil {
		switch err {
		case service.ErrProjectNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "プロジェクトが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		case service.ErrContainerAlreadyExists:
			return (*ctx).JSON(http.StatusConflict, map[string]string{
				"code":    "CONFLICT",
				"message": "コンテナ名が重複しています",
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
