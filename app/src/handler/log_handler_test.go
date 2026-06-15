package handler

import (
	"app/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockLogService は LogService のテスト用モック実装
type mockLogService struct {
	getPodLogsFunc func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error)
}

func (mock *mockLogService) GetPodLogs(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
	return mock.getPodLogsFunc(ctx, userID, deploymentID, since) // モック関数を呼び出す
}

// newGetPodLogsHandlerTestContext は GetPodLogs ハンドラーテスト用の Echo コンテキストを生成するヘルパー関数
func newGetPodLogsHandlerTestContext(deploymentID string, since string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()          // Echo インスタンスを生成する
	requestURL := "/"                   // リクエスト URL を設定する
	if since != "" {                    // since パラメータが指定されている場合はクエリに追加する
		requestURL = "/?since=" + since
	}
	request := httptest.NewRequest(http.MethodGet, requestURL, nil) // テスト用 GET リクエストを生成する
	recorder := httptest.NewRecorder()                              // レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, recorder)           // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                           // テスト用 UserID を設定する
	echoCtx.SetParamNames("id")                                     // パスパラメータ名を設定する
	echoCtx.SetParamValues(deploymentID)                            // パスパラメータ値を設定する
	return echoCtx, recorder
}

// TestGetPodLogs_正常系 は Pod ログが正常に取得できることを確認する
func TestGetPodLogs_正常系(t *testing.T) {
	expectedLogs := "line1\nline2\n" // 期待するログ文字列を設定する

	mockSvc := &mockLogService{
		getPodLogsFunc: func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
			return &service.GetPodLogsResult{Logs: expectedLogs}, nil // テスト用ログを返す
		},
	}

	logHandler := NewLogHandler(mockSvc)                                     // ハンドラーを生成する
	echoCtx, recorder := newGetPodLogsHandlerTestContext("deployment-1", "") // テストコンテキストを生成する

	if err := logHandler.GetPodLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}

	var responseBody map[string]interface{}                                     // レスポンスボディをデコードする
	if err := json.NewDecoder(recorder.Body).Decode(&responseBody); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if responseBody["logs"] != expectedLogs { // ログ内容を確認する
		t.Errorf("期待するログ %q、実際のログ %q", expectedLogs, responseBody["logs"])
	}
}

// TestGetPodLogs_チャンクなし は Pod ログが空の場合に last_timestamp が null で返ることを確認する
func TestGetPodLogs_チャンクなし(t *testing.T) {
	mockSvc := &mockLogService{
		getPodLogsFunc: func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
			return &service.GetPodLogsResult{Logs: "", LastTimestamp: nil}, nil // 空ログを返す
		},
	}

	logHandler := NewLogHandler(mockSvc)                                     // ハンドラーを生成する
	echoCtx, recorder := newGetPodLogsHandlerTestContext("deployment-1", "") // テストコンテキストを生成する

	if err := logHandler.GetPodLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}

	var responseBody map[string]interface{}                                     // レスポンスボディをデコードする
	if err := json.NewDecoder(recorder.Body).Decode(&responseBody); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if responseBody["last_timestamp"] != nil { // last_timestamp が nil であることを確認する
		t.Errorf("last_timestamp が nil ではありません: %v", responseBody["last_timestamp"])
	}
}

// TestGetPodLogs_since指定 は since パラメータ付きでログが取得できることを確認する
func TestGetPodLogs_since指定(t *testing.T) {
	capturedSince := (*time.Time)(nil) // GetPodLogs に渡された since を記録する

	mockSvc := &mockLogService{
		getPodLogsFunc: func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
			capturedSince = since                                              // since を記録する
			return &service.GetPodLogsResult{Logs: "line3\n"}, nil // テスト用ログを返す
		},
	}

	sinceStr := "2026-01-01T00:00:00Z"                                              // テスト用 since 文字列を設定する
	logHandler := NewLogHandler(mockSvc)                                            // ハンドラーを生成する
	echoCtx, recorder := newGetPodLogsHandlerTestContext("deployment-1", sinceStr)  // テストコンテキストを生成する

	if err := logHandler.GetPodLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}
	if capturedSince == nil { // since が nil でないことを確認する
		t.Fatal("since が nil です")
	}
}

// TestGetPodLogs_403_他ユーザー は他ユーザーの Deployment ログ取得で 403 が返ることを確認する
func TestGetPodLogs_403_他ユーザー(t *testing.T) {
	mockSvc := &mockLogService{
		getPodLogsFunc: func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	logHandler := NewLogHandler(mockSvc)                                     // ハンドラーを生成する
	echoCtx, recorder := newGetPodLogsHandlerTestContext("deployment-1", "") // テストコンテキストを生成する

	if err := logHandler.GetPodLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}

// TestGetPodLogs_404_存在しないDeployment は存在しない Deployment のログ取得で 404 が返ることを確認する
func TestGetPodLogs_404_存在しないDeployment(t *testing.T) {
	mockSvc := &mockLogService{
		getPodLogsFunc: func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
			return nil, gorm.ErrRecordNotFound // レコードなしエラーを返す
		},
	}

	logHandler := NewLogHandler(mockSvc)                                     // ハンドラーを生成する
	echoCtx, recorder := newGetPodLogsHandlerTestContext("deployment-1", "") // テストコンテキストを生成する

	if err := logHandler.GetPodLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusNotFound { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusNotFound, recorder.Code)
	}
}

// TestGetPodLogs_sinceパラメータ不正 は不正な since パラメータで 400 が返ることを確認する
func TestGetPodLogs_sinceパラメータ不正(t *testing.T) {
	mockSvc := &mockLogService{
		getPodLogsFunc: func(ctx context.Context, userID string, deploymentID string, since *time.Time) (*service.GetPodLogsResult, error) {
			return nil, nil // 呼ばれないはずなのでデフォルトを返す
		},
	}

	logHandler := NewLogHandler(mockSvc)                                              // ハンドラーを生成する
	echoCtx, recorder := newGetPodLogsHandlerTestContext("deployment-1", "not-a-date") // テストコンテキストを生成する

	if err := logHandler.GetPodLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetPodLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusBadRequest { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusBadRequest, recorder.Code)
	}
}
