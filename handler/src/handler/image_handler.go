package handler

import (
	"handler/logger"
	"handler/service"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ImageHandler はイメージ管理の HTTP ハンドラーを提供する
type ImageHandler struct {
	imageService service.ImageService // イメージサービスのインターフェース
}

// NewImageHandler は ImageHandler を生成して返す
func NewImageHandler(imageService service.ImageService) *ImageHandler {
	return &ImageHandler{
		imageService: imageService, // 依存を注入する
	}
}

// ListImagesByProject は GET /api/v1/projects/:id/images のハンドラー
func (imageHandler *ImageHandler) ListImagesByProject(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")        // パスパラメータから project ID を取得する

	imageList, err := imageHandler.imageService.ListImagesByProject(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出してイメージ一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("ImageHandler", "ListImagesByProject", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("ImageHandler", "ListImagesByProject", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ImageHandler", "ListImagesByProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, imageList) // イメージ一覧を返す
}

// GetImage は GET /api/v1/images/:imageId のハンドラー
func (imageHandler *ImageHandler) GetImage(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	imageID := echoCtx.Param("imageId")     // パスパラメータからイメージ ID を取得する

	imageData, err := imageHandler.imageService.GetImage(echoCtx.Request().Context(), userID, imageID) // サービスを呼び出してイメージを取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("ImageHandler", "GetImage", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("ImageHandler", "GetImage", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ImageHandler", "GetImage", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, imageData) // イメージレコードを返す
}

// DeleteImage は DELETE /api/v1/projects/:id/images/:imageId のハンドラー
func (imageHandler *ImageHandler) DeleteImage(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")        // パスパラメータから project ID を取得する
	imageID := echoCtx.Param("imageId")     // パスパラメータからイメージ ID を取得する

	if err := imageHandler.imageService.DeleteImage(echoCtx.Request().Context(), userID, projectID, imageID); err != nil { // サービスを呼び出してイメージを削除する
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("ImageHandler", "DeleteImage", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrImageInUse) { // 使用中の場合は 409 を返す
			logger.PrintHandlerError("ImageHandler", "DeleteImage", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "このイメージは Deployment から参照されているため削除できません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("ImageHandler", "DeleteImage", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("ImageHandler", "DeleteImage", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}
