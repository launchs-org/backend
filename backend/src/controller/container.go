package controller

import (
	"backend/service"
	"backend/k8slogwatcher"
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

// StreamContainerLogsWS はコンテナの実行ログをWebSocketでストリーミングするハンドラーです
func StreamContainerLogsWS(ctx *echo.Context) error {
	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")
	// JWTからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サブプロトコルからトークンを取得している場合、ハンドシェイク時にそれを返す必要がある
	responseHeader := http.Header{}
	if protocol := (*ctx).Request().Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		responseHeader.Set("Sec-WebSocket-Protocol", protocol)
	}

	ws, err := upgrader.Upgrade((*ctx).Response(), (*ctx).Request(), responseHeader)
	if err != nil {
		return err
	}
	// ハンドラー終了時にWebSocketを閉じる
	defer ws.Close()

	// ログのストリーミングを開始
	err = service.StreamContainerLogs(
		(*ctx).Request().Context(),
		containerID,
		userID,
		func(entry k8slogwatcher.LogEntry) {
			// ログエントリをクライアントに送信
			_ = ws.WriteJSON(map[string]interface{}{
				"event":     "log",
				"log":       entry.Message,
				"pod":       entry.PodName,
				"container": entry.Container,
				"timestamp": entry.Timestamp,
			})
		},
	)

	if err != nil {
		// エラーが発生した場合はクライアントに通知
		_ = ws.WriteJSON(map[string]string{"error": err.Error()})
	}

	return nil
}
