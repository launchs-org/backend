package handler

import (
	"handler/logger"
	"handler/service"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// LogHandler は Pod ログ取得の HTTP ハンドラーを提供する
type LogHandler struct {
	logService service.LogService // ログサービスのインターフェース
}

// NewLogHandler は LogHandler を生成して返す
func NewLogHandler(logService service.LogService) *LogHandler {
	return &LogHandler{
		logService: logService, // 依存を注入する
	}
}

// GetPodLogs は GET /api/v1/deployments/:id/logs のハンドラー
func (logHandler *LogHandler) GetPodLogs(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)   // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")        // パスパラメータから deployment ID を取得する

	var sinceTime *time.Time                          // since パラメータを格納する変数を定義する
	sinceParam := echoCtx.QueryParam("since")        // クエリパラメータ since を取得する
	if sinceParam != "" {                             // since が指定されている場合はパースする
		parsedTime, parseErr := time.Parse(time.RFC3339, sinceParam) // RFC3339 形式でパースする
		if parseErr != nil {                                          // パースエラーの場合は 400 を返す
			logger.PrintHandlerError("LogHandler", "GetPodLogs", echoCtx.Request().URL.Path, http.StatusBadRequest, parseErr) // エラーログを出力する
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{
				"error": "since パラメータの形式が不正です（RFC3339 形式で指定してください）",
			})
		}
		sinceTime = &parsedTime // パース結果を設定する
	}

	result, err := logHandler.logService.GetPodLogs(echoCtx.Request().Context(), userID, deploymentID, sinceTime) // サービスを呼び出して Pod ログを取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("LogHandler", "GetPodLogs", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("LogHandler", "GetPodLogs", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("LogHandler", "GetPodLogs", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, result) // ログ結果を返す
}
