package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNew_存在しないパスで404が返る は未登録パスへのリクエストで 404 が返ることを確認する
func TestNew_存在しないパスで404が返る(t *testing.T) {
	echoRouter := New(RouterOptions{}) // ルーターを生成する

	// 存在しないパスへのリクエストを作成する
	request := httptest.NewRequest(http.MethodGet, "/nonexistent-path", nil) // テスト用リクエストを生成する
	responseRecorder := httptest.NewRecorder()                               // テスト用レスポンスレコーダーを生成する

	echoRouter.ServeHTTP(responseRecorder, request) // リクエストを処理する

	// 404 が返ることを確認する
	if responseRecorder.Code != http.StatusNotFound {
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code) // 期待値と実際の値が異なる場合はテスト失敗とする
	}
}
