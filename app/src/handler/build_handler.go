package handler

import (
	"app/logger"
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

// TriggerBuildRequest は TriggerBuild のリクエストボディ
type TriggerBuildRequest struct {
	CommitMessage string `json:"commit_message"` // コミットメッセージ（オプション）
	Author        string `json:"author"`         // コミット著者（オプション）
}

// TriggerBuild は POST /api/v1/deployments/:id/build のハンドラー
func (buildHandler *BuildHandler) TriggerBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")     // パスパラメータから deployment ID を取得する

	var requestBody TriggerBuildRequest                      // リクエストボディの構造体を定義する
	_ = echoCtx.Bind(&requestBody)                          // バインド失敗は無視してオプション扱いにする

	buildData, err := buildHandler.buildService.TriggerBuild(echoCtx.Request().Context(), userID, deploymentID, requestBody.CommitMessage, requestBody.Author) // サービスを呼び出してビルドをトリガーする
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "TriggerBuild", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrBuildConflict) { // ビルド中の場合は 409 を返す
			logger.PrintHandlerError("BuildHandler", "TriggerBuild", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "ビルドが既に進行中です",
			})
		}
		if errors.Is(err, service.ErrDockerfileNotSupported) { // dockerfile タイプは未サポートのため 400 を返す
			logger.PrintHandlerError("BuildHandler", "TriggerBuild", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{
				"error": "dockerfile タイプは現在サポートされていません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "TriggerBuild", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "TriggerBuild", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusCreated, buildData) // 作成したビルドレコードを返す
}

// ListBuilds は GET /api/v1/deployments/:id/builds のハンドラー
func (buildHandler *BuildHandler) ListBuilds(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	deploymentID := echoCtx.Param("id")     // パスパラメータから deployment ID を取得する

	buildList, err := buildHandler.buildService.ListBuilds(echoCtx.Request().Context(), userID, deploymentID) // サービスを呼び出してビルド一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "ListBuilds", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "ListBuilds", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "ListBuilds", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, buildList) // ビルド一覧を返す
}

// ListBuildsByProject は GET /api/v1/projects/:id/builds のハンドラー
func (buildHandler *BuildHandler) ListBuildsByProject(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")        // パスパラメータから project ID を取得する

	buildList, err := buildHandler.buildService.ListBuildsByProject(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出してビルド一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 権限エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "ListBuildsByProject", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "ListBuildsByProject", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "ListBuildsByProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, buildList) // ビルド一覧を返す
}

// GetBuild は GET /api/v1/builds/:id のハンドラー
func (buildHandler *BuildHandler) GetBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	buildID := echoCtx.Param("id")           // パスパラメータからビルド ID を取得する

	buildData, err := buildHandler.buildService.GetBuild(echoCtx.Request().Context(), userID, buildID) // サービスを呼び出してビルドを取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "GetBuild", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "GetBuild", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "GetBuild", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.JSON(http.StatusOK, buildData) // ビルドレコードを返す
}

// CancelBuild は DELETE /api/v1/builds/:id のハンドラー
func (buildHandler *BuildHandler) CancelBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	buildID := echoCtx.Param("id")           // パスパラメータからビルド ID を取得する

	err := buildHandler.buildService.CancelBuild(echoCtx.Request().Context(), userID, buildID) // サービスを呼び出してビルドをキャンセルする
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "CancelBuild", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrBuildNotCancellable) { // キャンセル不可の場合は 409 を返す
			logger.PrintHandlerError("BuildHandler", "CancelBuild", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "ビルドはキャンセルできない状態です",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "CancelBuild", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "CancelBuild", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
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
			logger.PrintHandlerError("BuildHandler", "GetBuildLogs", echoCtx.Request().URL.Path, http.StatusBadRequest, parseErr) // エラーログを出力する
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{
				"error": "since パラメータの形式が不正です（RFC3339 形式で指定してください）",
			})
		}
		sinceTime = &parsedTime // パース結果を設定する
	}

	logContent, lastChunkTime, err := buildHandler.buildService.GetBuildLogs(echoCtx.Request().Context(), userID, buildID, sinceTime) // サービスを呼び出してビルドログを取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "GetBuildLogs", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "GetBuildLogs", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "GetBuildLogs", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{ // その他のエラーは 500 を返す
			"error": "内部サーバーエラー",
		})
	}

	type buildLogsResponse struct {
		Logs          string  `json:"logs"`                     // ログ文字列
		LastTimestamp *string `json:"last_timestamp,omitempty"` // 最終チャンク時刻（差分ポーリング用）
	}
	response := buildLogsResponse{Logs: logContent} // レスポンスを生成する
	if lastChunkTime != nil {
		formatted := lastChunkTime.UTC().Format(time.RFC3339Nano) // RFC3339 形式にフォーマットする
		response.LastTimestamp = &formatted                       // 最終チャンク時刻をセットする
	}
	return echoCtx.JSON(http.StatusOK, response) // ログ文字列と最終時刻を返す
}

// DeleteBuild は DELETE /api/v1/projects/:id/builds/:buildId のハンドラー
func (buildHandler *BuildHandler) DeleteBuild(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)   // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")           // パスパラメータから project ID を取得する
	buildID := echoCtx.Param("buildId")        // パスパラメータからビルド ID を取得する

	if err := buildHandler.buildService.DeleteBuild(echoCtx.Request().Context(), userID, projectID, buildID); err != nil { // サービスを呼び出してビルドを削除する
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合は 403 を返す
			logger.PrintHandlerError("BuildHandler", "DeleteBuild", echoCtx.Request().URL.Path, http.StatusForbidden, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusForbidden, map[string]string{
				"error": "アクセス権限がありません",
			})
		}
		if errors.Is(err, service.ErrBuildConflict) { // ビルド中は削除不可のため 409 を返す
			logger.PrintHandlerError("BuildHandler", "DeleteBuild", echoCtx.Request().URL.Path, http.StatusConflict, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusConflict, map[string]string{
				"error": "ビルド中のため削除できません",
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // リソースが見つからない場合は 404 を返す
			logger.PrintHandlerError("BuildHandler", "DeleteBuild", echoCtx.Request().URL.Path, http.StatusNotFound, err) // エラーログを出力する
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "リソースが見つかりません",
			})
		}
		logger.PrintHandlerError("BuildHandler", "DeleteBuild", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 削除成功時は 204 を返す
}
