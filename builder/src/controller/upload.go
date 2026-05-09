package controller

import (
	"net/http"

	"builder/service"

	"github.com/labstack/echo/v5"
)

// UploadTar はビルド済み tar を受け取り Harbor にプッシュするハンドラーです
func UploadTar(ctx *echo.Context) error {
	r := (*ctx).Request()

	jobID := r.Header.Get("X-Job-Id")
	imageName := r.Header.Get("X-Image-Name")
	imageTag := r.Header.Get("X-Image-Tag")

	if jobID == "" || imageName == "" || imageTag == "" {
		return (*ctx).String(http.StatusBadRequest, "必須ヘッダーが不足しています: X-Job-Id, X-Image-Name, X-Image-Tag")
	}

	if err := service.HandleUploadTar(r.Context(), r.Body, jobID, imageName, imageTag); err != nil {
		return (*ctx).String(http.StatusInternalServerError, "処理に失敗しました: "+err.Error())
	}

	return (*ctx).String(http.StatusAccepted, "accepted")
}
