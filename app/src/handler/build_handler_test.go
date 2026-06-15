package handler

import (
	"app/models"
	"app/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// mockBuildService は BuildService のテスト用モック実装
type mockBuildService struct {
	triggerBuildFunc func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error)
	cancelBuildFunc  func(ctx context.Context, userID string, buildID string) error
	getBuildLogsFunc func(ctx context.Context, userID string, buildID string, since *time.Time) (string, error)
}

func (mock *mockBuildService) TriggerBuild(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error) {
	return mock.triggerBuildFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockBuildService) CancelBuild(ctx context.Context, userID string, buildID string) error {
	return mock.cancelBuildFunc(ctx, userID, buildID) // モック関数を呼び出す
}

func (mock *mockBuildService) GetBuildLogs(ctx context.Context, userID string, buildID string, since *time.Time) (string, error) {
	return mock.getBuildLogsFunc(ctx, userID, buildID, since) // モック関数を呼び出す
}

// newCancelBuildHandlerTestContext は CancelBuild ハンドラーテスト用の Echo コンテキストを生成するヘルパー関数
func newCancelBuildHandlerTestContext(buildID string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                                    // Echo インスタンスを生成する
	request := httptest.NewRequest(http.MethodDelete, "/", nil)                   // テスト用 DELETE リクエストを生成する
	recorder := httptest.NewRecorder()                                            // レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, recorder)                         // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                         // テスト用 UserID を設定する
	echoCtx.SetParamNames("id")                                                   // パスパラメータ名を設定する
	echoCtx.SetParamValues(buildID)                                               // パスパラメータ値を設定する
	return echoCtx, recorder
}

// newBuildHandlerTestContext は BuildHandler テスト用の Echo コンテキストを生成するヘルパー関数
func newBuildHandlerTestContext(deploymentID string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                                // Echo インスタンスを生成する
	request := httptest.NewRequest(http.MethodPost, "/", nil)                 // テスト用リクエストを生成する
	recorder := httptest.NewRecorder()                                        // レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, recorder)                     // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                     // テスト用 UserID を設定する
	echoCtx.SetParamNames("id")                                               // パスパラメータ名を設定する
	echoCtx.SetParamValues(deploymentID)                                      // パスパラメータ値を設定する
	return echoCtx, recorder
}

// TestTriggerBuild_正常系 はビルドが正常にトリガーされることを確認する
func TestTriggerBuild_正常系(t *testing.T) {
	expectedBuild := &models.DeploymentBuild{
		ID:           "build-id-1",    // 期待するビルド ID を設定する
		DeploymentID: "deployment-1",  // 期待するデプロイメント ID を設定する
		Status:       models.BuildStatusPending, // 期待するステータスを設定する
	}

	mockSvc := &mockBuildService{
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error) {
			return expectedBuild, nil // 正常なビルドレコードを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newBuildHandlerTestContext("deployment-1")

	if err := buildHandler.TriggerBuild(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("TriggerBuild() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusCreated { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusCreated, recorder.Code)
	}

	var responseBuild models.DeploymentBuild                                   // レスポンスボディをデコードする
	if err := json.NewDecoder(recorder.Body).Decode(&responseBuild); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if responseBuild.ID != expectedBuild.ID { // ビルド ID を確認する
		t.Errorf("期待するビルド ID %s、実際の ID %s", expectedBuild.ID, responseBuild.ID)
	}
}

// TestTriggerBuild_403_他ユーザーのデプロイメント は他ユーザーのデプロイメントに POST すると 403 が返ることを確認する
func TestTriggerBuild_403_他ユーザーのデプロイメント(t *testing.T) {
	mockSvc := &mockBuildService{
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newBuildHandlerTestContext("deployment-1")

	if err := buildHandler.TriggerBuild(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("TriggerBuild() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}

// TestTriggerBuild_409_ビルド中 はビルド中に再ビルドをトリガーすると 409 が返ることを確認する
func TestTriggerBuild_409_ビルド中(t *testing.T) {
	mockSvc := &mockBuildService{
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error) {
			return nil, service.ErrBuildConflict // コンフリクトエラーを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newBuildHandlerTestContext("deployment-1")

	if err := buildHandler.TriggerBuild(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("TriggerBuild() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusConflict { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusConflict, recorder.Code)
	}
}

// TestCancelBuild_正常系 はビルドが正常にキャンセルされることを確認する
func TestCancelBuild_正常系(t *testing.T) {
	mockSvc := &mockBuildService{
		cancelBuildFunc: func(ctx context.Context, userID string, buildID string) error {
			return nil // キャンセル成功を返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newCancelBuildHandlerTestContext("build-1")

	if err := buildHandler.CancelBuild(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("CancelBuild() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}
}

// TestCancelBuild_409_完了済みビルド は完了済みビルドをキャンセルすると 409 が返ることを確認する
func TestCancelBuild_409_完了済みビルド(t *testing.T) {
	mockSvc := &mockBuildService{
		cancelBuildFunc: func(ctx context.Context, userID string, buildID string) error {
			return service.ErrBuildNotCancellable // キャンセル不可エラーを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newCancelBuildHandlerTestContext("build-1")

	if err := buildHandler.CancelBuild(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("CancelBuild() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusConflict { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusConflict, recorder.Code)
	}
}

// TestCancelBuild_403_他ユーザー は他ユーザーのビルドをキャンセルすると 403 が返ることを確認する
func TestCancelBuild_403_他ユーザー(t *testing.T) {
	mockSvc := &mockBuildService{
		cancelBuildFunc: func(ctx context.Context, userID string, buildID string) error {
			return service.ErrForbidden // 所有権エラーを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newCancelBuildHandlerTestContext("build-1")

	if err := buildHandler.CancelBuild(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("CancelBuild() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}

// newGetBuildLogsHandlerTestContext は GetBuildLogs ハンドラーテスト用の Echo コンテキストを生成するヘルパー関数
func newGetBuildLogsHandlerTestContext(buildID string, since string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                             // Echo インスタンスを生成する
	requestURL := "/"                                                      // リクエスト URL を設定する
	if since != "" {                                                       // since パラメータが指定されている場合はクエリに追加する
		requestURL = "/?since=" + since
	}
	request := httptest.NewRequest(http.MethodGet, requestURL, nil)        // テスト用 GET リクエストを生成する
	recorder := httptest.NewRecorder()                                     // レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, recorder)                  // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                  // テスト用 UserID を設定する
	echoCtx.SetParamNames("id")                                            // パスパラメータ名を設定する
	echoCtx.SetParamValues(buildID)                                        // パスパラメータ値を設定する
	return echoCtx, recorder
}

// TestGetBuildLogs_正常系 はビルドログが正常に取得できることを確認する
func TestGetBuildLogs_正常系(t *testing.T) {
	mockSvc := &mockBuildService{
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, error) {
			return "line1\nline2\n", nil // テスト用ログを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)                     // ハンドラーを生成する
	echoCtx, recorder := newGetBuildLogsHandlerTestContext("build-1", "")

	if err := buildHandler.GetBuildLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetBuildLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}

	var responseBody map[string]string                                      // レスポンスボディをデコードする
	if err := json.NewDecoder(recorder.Body).Decode(&responseBody); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if responseBody["logs"] != "line1\nline2\n" { // ログ内容を確認する
		t.Errorf("期待するログ %q、実際のログ %q", "line1\nline2\n", responseBody["logs"])
	}
}

// TestGetBuildLogs_since指定 は since パラメータ付きでログが取得できることを確認する
func TestGetBuildLogs_since指定(t *testing.T) {
	capturedSince := (*time.Time)(nil) // GetBuildLogs に渡された since を記録する

	mockSvc := &mockBuildService{
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, error) {
			capturedSince = since   // since を記録する
			return "line3\n", nil   // テスト用ログを返す
		},
	}

	sinceStr := "2026-01-01T00:00:00Z"                                      // テスト用 since 文字列を設定する
	buildHandler := NewBuildHandler(mockSvc)                                // ハンドラーを生成する
	echoCtx, recorder := newGetBuildLogsHandlerTestContext("build-1", sinceStr)

	if err := buildHandler.GetBuildLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetBuildLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}
	if capturedSince == nil { // since が nil でないことを確認する
		t.Fatal("since が nil です")
	}
}

// TestGetBuildLogs_403_他ユーザー は他ユーザーのビルドログ取得で 403 が返ることを確認する
func TestGetBuildLogs_403_他ユーザー(t *testing.T) {
	mockSvc := &mockBuildService{
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, error) {
			return "", service.ErrForbidden // 所有権エラーを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)                     // ハンドラーを生成する
	echoCtx, recorder := newGetBuildLogsHandlerTestContext("build-1", "")

	if err := buildHandler.GetBuildLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetBuildLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}

// TestGetBuildLogs_sinceパラメータ不正 は不正な since パラメータで 400 が返ることを確認する
func TestGetBuildLogs_sinceパラメータ不正(t *testing.T) {
	mockSvc := &mockBuildService{
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, error) {
			return "", nil // 呼ばれないはずなのでデフォルトを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)                           // ハンドラーを生成する
	echoCtx, recorder := newGetBuildLogsHandlerTestContext("build-1", "not-a-date")

	if err := buildHandler.GetBuildLogs(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetBuildLogs() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusBadRequest { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusBadRequest, recorder.Code)
	}
}
