package handler

import (
	"app/service"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// BuildHandler はビルドトリガーの HTTP ハンドラーを提供する
type BuildHandler struct {
	buildService service.BuildService // ビルドサービスのインターフェース
}

// NewBuildHandler は BuildHandler を生成して返す
func NewBuildHandler(buildService service.BuildService) *BuildHandler {
	return &BuildHandler{
		buildService: buildService, // 依存を注入する
	}
}

// TriggerBuild は POST /api/v1/deployments/:id/build のハンドラー
func (buildHandler *BuildHandler) TriggerBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")     // パスパラメータから deployment ID を取得する

	buildData, err := buildHandler.buildService.TriggerBuild(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出してビルドをトリガーする
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrBuildConflict) { // ビルド中の場合は 409 を返す
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "ビルドが既に進行中です",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, buildData) // 作成したビルドレコードを返す
}

// CancelBuild は DELETE /api/v1/builds/:id のハンドラー
func (buildHandler *BuildHandler) CancelBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	buildID := echoCtx.Param("id")           // パスパラメータからビルド ID を取得する

	err := buildHandler.buildService.CancelBuild(echoCtx.Request().Context(), userID, buildID) // サービスを呼び出してビルドをキャンセルする
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrBuildNotCancellable) { // キャンセル不可の場合は 409 を返す
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "ビルドはキャンセルできない状態です",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{ // キャンセル成功を返す
		"message": "ビルドをキャンセルしました",
	})
}

// GetBuildLogs は GET /api/v1/builds/:id/logs のハンドラー
func (buildHandler *BuildHandler) GetBuildLogs(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	buildID := echoCtx.Param("id")           // パスパラメータからビルド ID を取得する

	var sinceTime *time.Time                          // since パラメータを格納する変数を定義する
	sinceParam := echoCtx.QueryParam("since")        // クエリパラメータ since を取得する
	if sinceParam != "" {                             // since が指定されている場合はパースする
		parsedTime, parseErr := time.Parse(time.RFC3339, sinceParam) // RFC3339 形式でパースする
		if parseErr != nil {                                          // パースエラーの場合は 400 を返す
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{
				"error": "since パラメータの形式が不正です（RFC3339 形式で指定してください）",
			})
		}
		sinceTime = &parsedTime // パース結果を設定する
	}

	logContent, err := buildHandler.buildService.GetBuildLogs(echoCtx.Request().Context(), userID, buildID, sinceTime) // サービスを呼び出してビルドログを取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{ // ログ文字列を返す
		"logs": logContent,
	})
}
