package controller

import (
	"backend/model"   // モデルパッケージ
	"backend/service" // サービスパッケージ
	"net/http"        // HTTPステータス
	"github.com/labstack/echo/v5" // Echoフレームワーク
)

// CreateVolumeRequest はボリューム作成のリクエストボディです
type CreateVolumeRequest struct {
	Name      string `json:"name"`       // ボリューム名
	SizeMB    int    `json:"size_mb"`    // サイズ (MB)
	MountPath string `json:"mount_path"` // マウントパス
}

// CreateContainerVolume はコンテナに紐づくボリュームを作成するエンドポイントです
func CreateContainerVolume(ctx *echo.Context) error {
	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")

	// リクエストボディをバインド
	var req CreateVolumeRequest
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	// コンテキストからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// まずコンテナを取得して ProjectID を特定する必要がある
	// (service.CreateVolume で内部的にプロジェクト取得と権限チェックを行っているが、
	//  コンテナIDから ProjectID を引く必要がある)
	containerMap, err := service.GetContainer((*ctx).Request().Context(), containerID, userID)
	if err != nil {
		return (*ctx).JSON(http.StatusNotFound, map[string]string{
			"code":    "NOT_FOUND",
			"message": "コンテナが見つかりません",
		})
	}
	
	// data フィールドから Container 構造体を取得 (service.GetContainer の戻り値に合わせて)
	container, ok := containerMap["data"].(*model.Container)
	if !ok {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": "コンテナデータの取得に失敗しました",
		})
	}

	// ボリューム作成サービスを呼び出す
	// project_id はコンテナから取得
	vol, err := service.CreateVolume((*ctx).Request().Context(), service.CreateVolumeInput{
		ProjectID:   container.ProjectID,
		OwnerID:     userID,
		ContainerID: containerID,
		Name:        req.Name,
		SizeMB:      req.SizeMB,
		MountPath:   req.MountPath,
	})

	if err != nil {
		// エラーハンドリング
		status := http.StatusInternalServerError
		code := "INTERNAL_ERROR"
		message := err.Error()

		if err == service.ErrVolumeSizeExceeded {
			status = http.StatusBadRequest
			code = "BAD_REQUEST"
		} else if err == service.ErrForbidden {
			status = http.StatusForbidden
			code = "FORBIDDEN"
		}

		return (*ctx).JSON(status, map[string]string{
			"code":    code,
			"message": message,
		})
	}

	// 201 Created を返す
	return (*ctx).JSON(http.StatusCreated, map[string]interface{}{
		"data": vol,
	})
}

// ListContainerVolumes はコンテナに紐づくボリューム一覧を取得します
func ListContainerVolumes(ctx *echo.Context) error {
	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")

	// コンテキストからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// 一覧取得サービスを呼び出す
	volumes, err := service.ListVolumes((*ctx).Request().Context(), containerID, userID)
	if err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
	}

	// 200 OK を返す
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"items": volumes,
			"total": len(volumes),
		},
	})
}

// DeleteVolume はボリュームを削除します
func DeleteVolume(ctx *echo.Context) error {
	// パスパラメータからボリュームIDを取得
	volumeID := (*ctx).Param("id")

	// コンテキストからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// 削除サービスを呼び出す
	err := service.DeleteVolume((*ctx).Request().Context(), volumeID, userID)
	if err != nil {
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
	}

	// 200 OK を返す
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"id": volumeID,
		},
	})
}
