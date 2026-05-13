package controller

import (
	"backend/service"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"golang.org/x/net/websocket"
)

// StreamContainerLogs は WebSocket でコンテナの実行ログをストリーミングします。
// 接続時に DB の履歴ログを送信し、その後 Redis Pub/Sub でリアルタイム配信します。
func StreamContainerLogs(ctx *echo.Context) error {
	containerID := (*ctx).Param("id")
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		if err := service.StreamContainerLogs(ws, containerID, userID); err != nil {
			fmt.Printf("[ws] container logs error container=%s: %v\n", containerID, err)
		}
	}).ServeHTTP((*ctx).Response(), (*ctx).Request())

	return nil
}
