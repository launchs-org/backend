package handler

import (
	"app/logger"
	"app/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	defaultMetricsLimit = 120 // デフォルト取得件数（30 秒×120 件 = 過去 1 時間分）
	maxMetricsLimit     = 120 // 最大取得件数
)

// MetricsHandler は Deployment メトリクス取得の HTTP ハンドラーを提供する
type MetricsHandler struct {
	metricsService service.MetricsService // メトリクスサービスのインターフェース
}

// NewMetricsHandler は MetricsHandler を生成して返す
func NewMetricsHandler(metricsService service.MetricsService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: metricsService, // 依存を注入する
	}
}

// GetDeploymentMetrics は GET /deployments/:id/metrics のハンドラー
func (metricsHandler *MetricsHandler) GetDeploymentMetrics(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")      // パスパラメータから deployment ID を取得する

	// クエリパラメータ limit を解析する
	limit := defaultMetricsLimit // デフォルト値を設定する
	if limitStr := echoCtx.QueryParam("limit"); limitStr != "" {
		parsedLimit, parseErr := strconv.Atoi(limitStr) // 文字列を整数に変換する
		if parseErr != nil || parsedLimit <= 0 {        // 変換失敗または 0 以下の場合はエラーを返す
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{
				"error": "limit は正の整数で指定してください",
			})
		}
		if parsedLimit > maxMetricsLimit { // 最大値を超えた場合は上限に丸める
			parsedLimit = maxMetricsLimit
		}
		limit = parsedLimit // 有効な limit を設定する
	}

	metricsList, err := metricsHandler.metricsService.GetDeploymentMetrics(
		echoCtx.Request().Context(),
		userID,
		deploymentID,
		limit,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // deployment が見つからない場合は 404 を返す
			logger.PrintHandlerError("MetricsHandler", "GetDeploymentMetrics", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "deployment が見つかりません",
			})
		}
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("MetricsHandler", "GetDeploymentMetrics", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセスが許可されていません",
			})
		}
		logger.PrintHandlerError("MetricsHandler", "GetDeploymentMetrics", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // 予期しないエラーをログ出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}

	return echoCtx.JSON(http.StatusOK, map[string]interface{}{
		"metrics": metricsList, // メトリクス一覧を返す
	})
}
