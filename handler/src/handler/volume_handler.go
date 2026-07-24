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

// VolumeHandler はボリューム CRUD の HTTP ハンドラーを提供する
type VolumeHandler struct {
	volumeService service.VolumeService // volume サービスのインターフェース
}

// NewVolumeHandler は VolumeHandler を生成して返す
func NewVolumeHandler(volumeService service.VolumeService) *VolumeHandler {
	return &VolumeHandler{
		volumeService: volumeService, // volume サービスを注入する
	}
}

// ListVolumes は GET /api/v1/projects/:id/volumes のハンドラー
func (volumeHandler *VolumeHandler) ListVolumes(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")         // パスパラメータから project ID を取得する

	volumeList, err := volumeHandler.volumeService.ListVolumes(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出して一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("VolumeHandler", "ListVolumes", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("VolumeHandler", "ListVolumes", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("VolumeHandler", "ListVolumes", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, volumeList) // volume 一覧を返す
}

// CreateVolume は POST /api/v1/projects/:id/volumes のハンドラー
func (volumeHandler *VolumeHandler) CreateVolume(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")         // パスパラメータから project ID を取得する

	var requestBody service.CreateVolumeRequest               // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	if requestBody.Name == "" { // 必須フィールドのバリデーションを行う
		logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("name は必須です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "name は必須です",
		})
	}
	if requestBody.SizeMB <= 0 { // size_mb のバリデーションを行う
		logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("size_mb は 1 以上の値が必要です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "size_mb は 1 以上の値が必要です",
		})
	}

	volumeData, err := volumeHandler.volumeService.CreateVolume(echoCtx.Request().Context(), userID, projectID, requestBody) // サービスを呼び出して volume を作成する
	if err != nil {
		var quotaErr *service.QuotaExceededError
		if errors.As(err, &quotaErr) { // quota 超過の場合は 403 と詳細情報を返す
			logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]interface{}{
				"error":    "quota_exceeded",
				"resource": quotaErr.Resource,
				"current":  quotaErr.Current,
				"limit":    quotaErr.Limit,
			})
		}
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("VolumeHandler", "CreateVolume", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, volumeData) // 作成結果を返す
}

// DeleteVolume は DELETE /api/v1/volumes/:id のハンドラー
func (volumeHandler *VolumeHandler) DeleteVolume(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	volumeID := echoCtx.Param("id")          // パスパラメータから volume ID を取得する

	err := volumeHandler.volumeService.DeleteVolume(echoCtx.Request().Context(), userID, volumeID) // サービスを呼び出して volume を削除する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("VolumeHandler", "DeleteVolume", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("VolumeHandler", "DeleteVolume", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		if errors.Is(err, service.ErrVolumeMountedCannotDelete) { // マウント中の場合は 400 を返す
			logger.PrintHandlerError("VolumeHandler", "DeleteVolume", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{
				"error": "ボリュームがマウントされています。先にアンマウントしてから削除してください",
			})
		}
		logger.PrintHandlerError("VolumeHandler", "DeleteVolume", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}

// ListVolumeMounts は GET /api/v1/deployments/:id/volume-mounts のハンドラー
func (volumeHandler *VolumeHandler) ListVolumeMounts(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)  // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")       // パスパラメータから deployment ID を取得する

	mountList, err := volumeHandler.volumeService.ListVolumeMounts(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出してマウント一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("VolumeHandler", "ListVolumeMounts", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("VolumeHandler", "ListVolumeMounts", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("VolumeHandler", "ListVolumeMounts", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, mountList) // マウント一覧を返す
}

// CreateVolumeMount は POST /api/v1/deployments/:id/volume-mounts のハンドラー
func (volumeHandler *VolumeHandler) CreateVolumeMount(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)  // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")       // パスパラメータから deployment ID を取得する

	var requestBody service.CreateVolumeMountRequest              // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {           // リクエストをバインドする
		logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "リクエストが不正です",
		})
	}
	if requestBody.VolumeID == "" { // 必須フィールドのバリデーションを行う
		logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("volume_id は必須です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "volume_id は必須です",
		})
	}
	if requestBody.MountPath == "" { // マウントパスのバリデーションを行う
		logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusBadRequest, fmt.Errorf("mount_path は必須です")) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{
			"error": "mount_path は必須です",
		})
	}

	mountData, err := volumeHandler.volumeService.CreateVolumeMount(echoCtx.Request().Context(), userID, deploymentID, requestBody) // サービスを呼び出してマウント設定を作成する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, service.ErrDuplicateVolumeMount) { // 重複エラーの場合は 409 を返す
			logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "同一マウントパスが既に存在します",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("VolumeHandler", "CreateVolumeMount", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, mountData) // 作成結果を返す
}

// DeleteVolumeMount は DELETE /api/v1/volume-mounts/:id のハンドラー
func (volumeHandler *VolumeHandler) DeleteVolumeMount(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	mountID := echoCtx.Param("id")           // パスパラメータからマウント ID を取得する

	err := volumeHandler.volumeService.DeleteVolumeMount(echoCtx.Request().Context(), userID, mountID) // サービスを呼び出してマウント設定を削除する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("VolumeHandler", "DeleteVolumeMount", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが拒否されました",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("VolumeHandler", "DeleteVolumeMount", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("VolumeHandler", "DeleteVolumeMount", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}
