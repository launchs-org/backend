package handler

import (
	"app/logger"
	"app/middlewares"
	"app/service"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EnvVarHandler は環境変数 CRUD の HTTP ハンドラーを提供する
type EnvVarHandler struct {
	envVarService      service.EnvVarService      // env_var サービスのインターフェース
	envVarMountService service.EnvVarMountService // env_var_mount サービスのインターフェース
}

// NewEnvVarHandler は EnvVarHandler を生成して返す
func NewEnvVarHandler(envVarService service.EnvVarService, envVarMountService service.EnvVarMountService) *EnvVarHandler {
	return &EnvVarHandler{
		envVarService:      envVarService,      // env_var サービスを注入する
		envVarMountService: envVarMountService, // env_var_mount サービスを注入する
	}
}

// maskedValue は is_secret=true のときに返すマスク文字列
const maskedValue = "***"

// ListEnvVars は GET /api/v1/projects/:id/env-vars のハンドラー
func (envVarHandler *EnvVarHandler) ListEnvVars(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	envVarList, err := envVarHandler.envVarService.ListEnvVars(echoCtx.Request().Context(), userClaim.UserID, projectID) // サービスを呼び出して一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "ListEnvVars", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "ListEnvVars", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "ListEnvVars", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}

	responseList := make([]map[string]interface{}, 0, len(envVarList)) // レスポンス一覧を生成する
	for _, envVar := range envVarList {
		value := envVar.Value // 値を取得する
		if envVar.IsSecret {
			value = maskedValue // シークレットの場合はマスクする
		}
		responseList = append(responseList, map[string]interface{}{
			"id":         envVar.ID,        // ID を設定する
			"project_id": envVar.ProjectID, // プロジェクト ID を設定する
			"key":        envVar.Key,       // キーを設定する
			"value":      value,            // マスク済みの値を設定する
			"is_secret":  envVar.IsSecret,  // シークレットフラグを設定する
		})
	}
	return echoCtx.JSON(http.StatusOK, responseList) // env_var 一覧を返す
}

// CreateEnvVar は POST /api/v1/projects/:id/env-vars のハンドラー
func (envVarHandler *EnvVarHandler) CreateEnvVar(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	var requestBody service.CreateEnvVarRequest               // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("EnvVarHandler", "CreateEnvVar", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	if requestBody.Key == "" { // 必須フィールドのバリデーションを行う
		logger.PrintHandlerError("EnvVarHandler", "CreateEnvVar", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("key は必須です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "key は必須です",
		})
	}

	envVarData, err := envVarHandler.envVarService.CreateEnvVar(echoCtx.Request().Context(), userClaim.UserID, projectID, requestBody) // サービスを呼び出して env_var を作成する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "CreateEnvVar", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "CreateEnvVar", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "CreateEnvVar", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}

	value := envVarData.Value // 値を取得する
	if envVarData.IsSecret {
		value = maskedValue // シークレットの場合はマスクする
	}
	return echoCtx.JSON(http.StatusCreated, map[string]interface{}{
		"id":         envVarData.ID,        // ID を設定する
		"project_id": envVarData.ProjectID, // プロジェクト ID を設定する
		"key":        envVarData.Key,       // キーを設定する
		"value":      value,                // マスク済みの値を設定する
		"is_secret":  envVarData.IsSecret,  // シークレットフラグを設定する
	})
}

// UpdateEnvVar は PUT /api/v1/env-vars/:id のハンドラー
func (envVarHandler *EnvVarHandler) UpdateEnvVar(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	envVarID := echoCtx.Param("id")                                   // パスパラメータから env_var ID を取得する

	var requestBody service.UpdateEnvVarRequest               // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("EnvVarHandler", "UpdateEnvVar", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}

	envVarData, err := envVarHandler.envVarService.UpdateEnvVar(echoCtx.Request().Context(), userClaim.UserID, envVarID, requestBody) // サービスを呼び出して env_var を更新する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "UpdateEnvVar", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "UpdateEnvVar", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "UpdateEnvVar", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}

	value := envVarData.Value // 値を取得する
	if envVarData.IsSecret {
		value = maskedValue // シークレットの場合はマスクする
	}
	return echoCtx.JSON(http.StatusOK, map[string]interface{}{
		"id":         envVarData.ID,        // ID を設定する
		"project_id": envVarData.ProjectID, // プロジェクト ID を設定する
		"key":        envVarData.Key,       // キーを設定する
		"value":      value,                // マスク済みの値を設定する
		"is_secret":  envVarData.IsSecret,  // シークレットフラグを設定する
	})
}

// DeleteEnvVar は DELETE /api/v1/env-vars/:id のハンドラー
func (envVarHandler *EnvVarHandler) DeleteEnvVar(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	envVarID := echoCtx.Param("id")                                   // パスパラメータから env_var ID を取得する

	err := envVarHandler.envVarService.DeleteEnvVar(echoCtx.Request().Context(), userClaim.UserID, envVarID) // サービスを呼び出して env_var を削除する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "DeleteEnvVar", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "DeleteEnvVar", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "DeleteEnvVar", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}

// ListEnvVarMounts は GET /api/v1/deployments/:id/env-var-mounts のハンドラー
func (envVarHandler *EnvVarHandler) ListEnvVarMounts(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	deploymentID := echoCtx.Param("id")                               // パスパラメータから deployment ID を取得する

	mountList, err := envVarHandler.envVarMountService.ListEnvVarMounts(echoCtx.Request().Context(), userClaim.UserID, deploymentID) // サービスを呼び出して一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "ListEnvVarMounts", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "ListEnvVarMounts", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "ListEnvVarMounts", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, mountList) // マウント設定一覧を返す
}

// CreateEnvVarMount は POST /api/v1/deployments/:id/env-var-mounts のハンドラー
func (envVarHandler *EnvVarHandler) CreateEnvVarMount(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	deploymentID := echoCtx.Param("id")                               // パスパラメータから deployment ID を取得する

	var requestBody service.CreateEnvVarMountRequest          // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("EnvVarHandler", "CreateEnvVarMount", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	if requestBody.EnvVarID == "" { // 必須フィールドのバリデーションを行う
		logger.PrintHandlerError("EnvVarHandler", "CreateEnvVarMount", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("env_var_id は必須です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "env_var_id は必須です",
		})
	}

	mountData, err := envVarHandler.envVarMountService.CreateEnvVarMount(echoCtx.Request().Context(), userClaim.UserID, deploymentID, requestBody) // サービスを呼び出してマウント設定を作成する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "CreateEnvVarMount", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, service.ErrDuplicateMount) { // 重複マウントの場合は 409 を返す
			logger.PrintHandlerError("EnvVarHandler", "CreateEnvVarMount", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "この環境変数は既にマウントされています",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "CreateEnvVarMount", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "CreateEnvVarMount", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, mountData) // 作成結果を返す
}

// DeleteEnvVarMount は DELETE /api/v1/env-var-mounts/:id のハンドラー
func (envVarHandler *EnvVarHandler) DeleteEnvVarMount(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	mountID := echoCtx.Param("id")                                    // パスパラメータからマウント ID を取得する

	err := envVarHandler.envVarMountService.DeleteEnvVarMount(echoCtx.Request().Context(), userClaim.UserID, mountID) // サービスを呼び出してマウント設定を削除する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("EnvVarHandler", "DeleteEnvVarMount", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("EnvVarHandler", "DeleteEnvVarMount", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("EnvVarHandler", "DeleteEnvVarMount", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}
