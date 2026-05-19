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

// CreateTemplateContainerRequest はテンプレートコンテナ作成のリクエストボディです
// ListTemplates はYAMLで定義されたテンプレート一覧を返すハンドラーです
func ListTemplates(ctx *echo.Context) error {
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": service.ListTemplates(),
	})
}

type CreateTemplateContainerRequest struct {
	Name          string `json:"name"`
	ContainerType string `json:"container_type"`
	EnvVars       string `json:"env_vars"`
}

// CreateTemplateContainer はMySQL/PostgreSQL/Redisなどのテンプレートからコンテナを作成するハンドラーです
func CreateTemplateContainer(ctx *echo.Context) error {
	var req CreateTemplateContainerRequest
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

	res, err := service.CreateTemplateContainer((*ctx).Request().Context(), service.CreateTemplateContainerInput{
		ProjectID:     projectID,
		OwnerID:       userID,
		Name:          req.Name,
		ContainerType: req.ContainerType,
		EnvVars:       req.EnvVars,
	})

	if err != nil {
		switch err {
		case service.ErrInvalidContainerType:
			return (*ctx).JSON(http.StatusBadRequest, map[string]string{
				"code":    "BAD_REQUEST",
				"message": "サポートされていないコンテナタイプです",
			})
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

// ListBuildJobs はコンテナのビルド履歴一覧を取得するハンドラーです
func ListBuildJobs(ctx *echo.Context) error {
	containerID := (*ctx).Param("id") // note: router specifies /containers/:id/build-jobs
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	res, err := service.ListBuildJobs((*ctx).Request().Context(), service.ListBuildJobsInput{
		ContainerID: containerID,
		OwnerID:     userID,
	})

	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}

// GetContainer はコンテナの詳細を取得するハンドラーです
func GetContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	res, err := service.GetContainer((*ctx).Request().Context(), containerID, userID)

	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}

// DeleteContainer はコンテナを削除するハンドラーです
func DeleteContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	err := service.DeleteContainer((*ctx).Request().Context(), containerID, userID)

	if err != nil {

		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, map[string]string{"message": "コンテナが削除されました"})
}

// UpdateContainerRequest はコンテナ更新のリクエストボディです
type UpdateContainerRequest struct {
	RepositoryURL *string `json:"repository_url"`
	Branch        *string `json:"branch"`
	Directory     *string `json:"directory"`
	EnvVars       *string `json:"env_vars"`
	Replicas      *int    `json:"replicas"`
	Resources     *string `json:"resources"`
}

// UpdateContainer はコンテナの設定を更新し、再ビルドを開始するハンドラーです
func UpdateContainer(ctx *echo.Context) error {
	var req UpdateContainerRequest
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	res, err := service.UpdateContainer((*ctx).Request().Context(), service.UpdateContainerInput{
		ContainerID:   containerID,
		OwnerID:       userID,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		Directory:     req.Directory,
		EnvVars:       req.EnvVars,
		Replicas:      req.Replicas,
		Resources:     req.Resources,
	})

	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}

// ScaleContainer はコンテナのレプリカ数を変更するハンドラーです (1〜5)
func ScaleContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	res, err := service.ScaleContainer((*ctx).Request().Context(), containerID, userID, req.Replicas)
	if err != nil {
		switch err {
		case service.ErrInvalidReplicas:
			return (*ctx).JSON(http.StatusBadRequest, map[string]string{
				"code":    "BAD_REQUEST",
				"message": "レプリカ数は1〜5の範囲で指定してください",
			})
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}

// UpdateContainerEnvVars は再ビルドせずに環境変数のみ更新するハンドラーです
func UpdateContainerEnvVars(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	var req struct {
		EnvVars string `json:"env_vars"`
	}
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	res, err := service.UpdateContainerEnvVars((*ctx).Request().Context(), containerID, userID, req.EnvVars)
	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "コンテナが見つかりません"})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{"code": "FORBIDDEN", "message": "アクセス権限がありません"})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{"code": "INTERNAL_ERROR", "message": err.Error()})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}

// RebuildContainer はコンテナを再ビルドするハンドラーです
func RebuildContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	res, err := service.RebuildContainer((*ctx).Request().Context(), containerID, userID)

	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}

// RedeployContainer はコンテナを再デプロイするハンドラーです
func RedeployContainer(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	res, err := service.RedeployContainer((*ctx).Request().Context(), containerID, userID)

	if err != nil {
		switch err {
		case service.ErrContainerNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	return (*ctx).JSON(http.StatusOK, res)
}
