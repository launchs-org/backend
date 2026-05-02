package controller

import (
	"backend/service"
	"backend/utils"
	"net/http"

	"github.com/labstack/echo/v5"
)

// UploadTar はビルドされたtarファイルを受け取るハンドラーです
func UploadTar(ctx *echo.Context) error {
	authHeader := (*ctx).Request().Header.Get("Authorization")
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return (*ctx).String(http.StatusUnauthorized, "認証に失敗しました: トークンがありません")
	}
	tokenString := authHeader[7:]

	claim, err := utils.VerifyJobToken(tokenString)
	if err != nil {
		return (*ctx).String(http.StatusUnauthorized, "認証に失敗しました: "+err.Error())
	}

	jobID := claim.JobID
	imageName := claim.ImageName
	imageTag := claim.ImageTag

	if jobID == "" || imageName == "" || imageTag == "" {
		return (*ctx).String(http.StatusBadRequest, "トークンに必要な情報が含まれていません")
	}

	err = service.HandleUploadTar((*ctx).Request().Body, jobID, imageName, imageTag)
	if err != nil {
		return (*ctx).String(http.StatusInternalServerError, "保存に失敗しました: "+err.Error())
	}

	return (*ctx).String(http.StatusAccepted, "accepted")
}
