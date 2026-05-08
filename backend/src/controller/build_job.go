package controller

import (
	"launchs/shared/model"
	"backend/service"
	"context"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
)

// GetBuildJobLogs はビルドジョブのログを取得するハンドラーです
func GetBuildJobLogs(ctx *echo.Context) error {
	// パスパラメータからビルドジョブIDを取得
	buildJobID := (*ctx).Param("id")
	// JWTからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サービスを呼び出してログを取得
	logs, err := service.GetBuildJobLogs((*ctx).Request().Context(), buildJobID, userID)

	if err != nil {
		switch err {
		case service.ErrBuildJobNotFound:
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "ビルドジョブが見つかりません",
			})
		case service.ErrForbidden:
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		default:
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": err.Error(),
			})
		}
	}

	// レスポンスを返す
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"log": logs,
		},
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 開発環境のため
	},
}

// StreamBuildJobLogsWS はビルドジョブのログをWebSocketでストリーミングするハンドラーです
func StreamBuildJobLogsWS(ctx *echo.Context) error {
	buildJobID := (*ctx).Param("id")
	// JWT認証 (WebSocketのためクエリパラメータなどからトークンを取得する必要があるかもしれないが、
	// ここでは簡単のためミドルウェアでセットされたUserIDを使用。
	// ただし WebSocket アップグレード後はミドルウェアの context がどうなるか注意)
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サブプロトコルからトークンを取得している場合、ハンドシェイク時にそれを返す必要がある
	responseHeader := http.Header{}
	if protocol := (*ctx).Request().Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		responseHeader.Set("Sec-WebSocket-Protocol", protocol)
	}

	ws, err := upgrader.Upgrade((*ctx).Response(), (*ctx).Request(), responseHeader)
	if err != nil {
		return err
	}
	defer ws.Close()

	// プロジェクト所有者チェック
	job, err := model.GetBuildJobByID(buildJobID)
	if err != nil {
		return ws.WriteJSON(map[string]string{"error": "job not found"})
	}
	project, err := model.GetProjectByID(job.ProjectID)
	if err != nil {
		return ws.WriteJSON(map[string]string{"error": "project not found"})
	}
	if project.OwnerID != userID {
		return ws.WriteJSON(map[string]string{"error": "forbidden"})
	}

	// コンテキストを作成し、書き込みエラー時にキャンセルできるようにする
	streamCtx, cancel := context.WithCancel((*ctx).Request().Context())
	defer cancel()

	// ログのストリーミングを開始
	err = service.StreamBuildJobLogs(
		streamCtx,
		buildJobID,
		userID,
		func(logStr string) {
			// ログを送信
			err := ws.WriteJSON(map[string]interface{}{
				"event": "log",
				"log":   logStr,
			})
			if err != nil {
				cancel()
				return
			}
		},
		func(status string) {
			// ステータスを送信
			_ = ws.WriteJSON(map[string]interface{}{
				"event":  "status",
				"status": status,
			})
			// 完了した場合は WebSocket を閉じるための情報を送る
			if status == "Succeeded" || status == "Failed" || status == "Complete" {
				_ = ws.WriteJSON(map[string]interface{}{
					"event": "done",
					"status": status,
				})
			}
		},
	)

	if err != nil {
		_ = ws.WriteJSON(map[string]string{"error": err.Error()})
	}

	return nil
}
