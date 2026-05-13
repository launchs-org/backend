package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"launchs/shared/database"
	"launchs/shared/model"

	"golang.org/x/net/websocket"
)

type podLogMessage struct {
	PodName   string    `json:"pod_name"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type wsOutMessage struct {
	Event     string    `json:"event"`
	Log       string    `json:"log,omitempty"`
	Pod       string    `json:"pod,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// StreamContainerLogs は WebSocket に履歴ログを送信後、Redis Pub/Sub でリアルタイム配信します。
//
// 重複防止の仕組み:
//  1. Redis 購読を先に開始し、届いたメッセージを一時バッファに蓄積
//  2. DB 履歴ログを全送信し、その開始時刻を記録
//  3. バッファ内のメッセージのうち DB 取得開始時刻より後のものだけ送信（重複をスキップ）
//  4. 以後は Redis から直接ストリーミング
func StreamContainerLogs(ws *websocket.Conn, containerID, ownerID string) error {
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return ErrContainerNotFound
	}

	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return err
	}
	if project.OwnerID != ownerID {
		return ErrForbidden
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WebSocket 切断を検知するゴルーチン
	disconnected := make(chan struct{})
	go func() {
		var buf []byte
		for {
			if err := websocket.Message.Receive(ws, &buf); err != nil {
				close(disconnected)
				return
			}
		}
	}()

	// Redis 購読を DB 取得より先に開始して取りこぼしを防ぐ
	pattern := fmt.Sprintf("stream:pod:%s:*", project.Namespace)
	pubsub := database.RedisClient.PSubscribe(ctx, pattern)
	defer pubsub.Close()

	redisCh := pubsub.Channel()

	// DB 履歴送信中に届いた Redis メッセージをバッファへ蓄積する
	var (
		mu           sync.Mutex
		buffer       []podLogMessage
		buffering    atomic.Bool
		liveMessages = make(chan podLogMessage, 256)
	)
	buffering.Store(true)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-redisCh:
				if !ok {
					return
				}
				var pl podLogMessage
				if err := json.Unmarshal([]byte(msg.Payload), &pl); err != nil {
					continue
				}
				// バッファリング中は buffer へ、完了後は liveMessages へ
				if buffering.Load() {
					mu.Lock()
					buffer = append(buffer, pl)
					mu.Unlock()
				} else {
					liveMessages <- pl
				}
			}
		}
	}()

	// DB 履歴ログを送信し、取得開始時刻を記録
	historyStart := time.Now()
	if raw, err := model.GetContainerLog(containerID); err == nil && len(raw) > 0 {
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			if err := sendLog(ws, container.Name, line, time.Time{}); err != nil {
				return nil
			}
		}
	}

	// バッファリング終了: 以後は liveMessages へ流す
	buffering.Store(false)

	// バッファ内のメッセージのうち historyStart より後のものだけ送信（重複をスキップ）
	mu.Lock()
	buffered := make([]podLogMessage, len(buffer))
	copy(buffered, buffer)
	mu.Unlock()

	for _, pl := range buffered {
		if pl.Timestamp.After(historyStart) {
			if err := sendLog(ws, pl.PodName, pl.Message, pl.Timestamp); err != nil {
				return nil
			}
		}
	}

	// 以後はリアルタイムで Redis を流す
	for {
		select {
		case <-disconnected:
			return nil
		case <-ctx.Done():
			return nil
		case pl, ok := <-liveMessages:
			if !ok {
				return nil
			}
			if err := sendLog(ws, pl.PodName, pl.Message, pl.Timestamp); err != nil {
				return nil
			}
		}
	}
}

func sendLog(ws *websocket.Conn, pod, line string, ts time.Time) error {
	if ts.IsZero() {
		ts = time.Now()
	}
	out := wsOutMessage{
		Event:     "log",
		Log:       line,
		Pod:       pod,
		Timestamp: ts,
	}
	b, _ := json.Marshal(out)
	return websocket.Message.Send(ws, string(b))
}
