package handler

import (
	"handler/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// mockQuotaService は QuotaService のテスト用モック実装
type mockQuotaService struct {
	getQuotaFunc    func(ctx context.Context, userID string) (*service.QuotaResponse, error)                        // GetQuota のモック関数
	updateQuotaFunc func(ctx context.Context, userID string, req service.UpdateQuotaRequest) (*service.QuotaResponse, error) // UpdateQuota のモック関数
}

func (mock *mockQuotaService) GetQuota(ctx context.Context, userID string) (*service.QuotaResponse, error) {
	return mock.getQuotaFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockQuotaService) UpdateQuota(ctx context.Context, userID string, req service.UpdateQuotaRequest) (*service.QuotaResponse, error) {
	return mock.updateQuotaFunc(ctx, userID, req) // モック関数を呼び出す
}

// setupEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupEchoContext(method, path, body string, userID string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                               // Echo インスタンスを生成する
	var requestBody *strings.Reader                                          // リクエストボディを定義する
	if body != "" {
		requestBody = strings.NewReader(body) // ボディが存在する場合は設定する
	} else {
		requestBody = strings.NewReader("") // ボディが空の場合は空文字列を設定する
	}
	request := httptest.NewRequest(method, path, requestBody)               // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)    // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                               // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)            // Echo コンテキストを生成する
	echoCtx.Set("UserID", userID) // ミドルウェアがセットする UserID をコンテキストに設定する
	return echoCtx, responseRecorder
}

// TestGetQuota_正常にquotaと使用量が返る は GET /users/:user_id/quota が quota と現在使用量を返すことを確認する
func TestGetQuota_正常にquotaと使用量が返る(t *testing.T) {
	expectedResponse := &service.QuotaResponse{ // 期待するレスポンスを定義する
		UserID:                   "test-user-id",
		PlanID:                   "plan-free-id",
		MaxProjects:              5,
		MaxDeployments:           20,
		MaxReplicasPerDeployment: 5,
		MaxVolumes:               10,
		MaxVolumeSizeMB:          10240,
		MaxTotalVolumeMB:         51200,
		InstanceLimits:           map[string]int{"small": 5, "medium": 2},
		CurrentProjects:          2,
		CurrentDeployments:       3,
		CurrentVolumes:           1,
		CurrentTotalVolumeMB:     2048,
	}

	mockService := &mockQuotaService{
		getQuotaFunc: func(ctx context.Context, userID string) (*service.QuotaResponse, error) {
			return expectedResponse, nil // 期待するレスポンスを返す
		},
	}

	quotaHandler := NewUserQuotaHandler(mockService)                         // ハンドラーを生成する
	echoCtx, responseRecorder := setupEchoContext(http.MethodGet, "/api/v1/users/test-user-id/quota", "", "test-user-id") // テスト用コンテキストを生成する

	err := quotaHandler.GetQuota(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err) // ハンドラーエラーをテスト失敗とする
	}

	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualResponse service.QuotaResponse
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualResponse); err != nil { // レスポンスボディをデコードする
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}

	if actualResponse.UserID != expectedResponse.UserID { // ユーザーIDを確認する
		t.Errorf("期待するUserID: %s, 実際のUserID: %s", expectedResponse.UserID, actualResponse.UserID)
	}
	if actualResponse.CurrentProjects != expectedResponse.CurrentProjects { // 現在のプロジェクト数を確認する
		t.Errorf("期待するCurrentProjects: %d, 実際のCurrentProjects: %d", expectedResponse.CurrentProjects, actualResponse.CurrentProjects)
	}
	if actualResponse.CurrentDeployments != expectedResponse.CurrentDeployments { // 現在のデプロイメント数を確認する
		t.Errorf("期待するCurrentDeployments: %d, 実際のCurrentDeployments: %d", expectedResponse.CurrentDeployments, actualResponse.CurrentDeployments)
	}
}

// TestGetQuota_レコードが存在しないユーザーでもデフォルト値で200が返る は auto-create 動作を確認する
func TestGetQuota_レコードが存在しないユーザーでもデフォルト値で200が返る(t *testing.T) {
	defaultResponse := &service.QuotaResponse{ // デフォルト値のレスポンスを定義する
		UserID:                   "new-user-id",
		PlanID:                   "plan-free-id",
		MaxProjects:              5,
		MaxDeployments:           20,
		MaxReplicasPerDeployment: 5,
		MaxVolumes:               10,
		MaxVolumeSizeMB:          10240,
		MaxTotalVolumeMB:         51200,
		InstanceLimits:           map[string]int{},
		CurrentProjects:          0,
		CurrentDeployments:       0,
		CurrentVolumes:           0,
		CurrentTotalVolumeMB:     0,
	}

	mockService := &mockQuotaService{
		getQuotaFunc: func(ctx context.Context, userID string) (*service.QuotaResponse, error) {
			return defaultResponse, nil // 新規ユーザーでもデフォルト値を返す
		},
	}

	quotaHandler := NewUserQuotaHandler(mockService)                         // ハンドラーを生成する
	echoCtx, responseRecorder := setupEchoContext(http.MethodGet, "/api/v1/users/new-user-id/quota", "", "new-user-id") // テスト用コンテキストを生成する

	err := quotaHandler.GetQuota(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err) // ハンドラーエラーをテスト失敗とする
	}

	if responseRecorder.Code != http.StatusOK { // 新規ユーザーでも 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualResponse service.QuotaResponse
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualResponse); err != nil { // レスポンスボディをデコードする
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}

	if actualResponse.MaxProjects != 5 { // デフォルトのプロジェクト上限を確認する
		t.Errorf("期待するMaxProjects: 5, 実際のMaxProjects: %d", actualResponse.MaxProjects)
	}
	if actualResponse.MaxDeployments != 20 { // デフォルトのデプロイメント上限を確認する
		t.Errorf("期待するMaxDeployments: 20, 実際のMaxDeployments: %d", actualResponse.MaxDeployments)
	}
	if actualResponse.CurrentProjects != 0 { // 新規ユーザーのプロジェクト数が 0 であることを確認する
		t.Errorf("期待するCurrentProjects: 0, 実際のCurrentProjects: %d", actualResponse.CurrentProjects)
	}
}

// TestUpdateQuota_部分更新が正しく反映される は PUT /users/:user_id/quota で部分更新できることを確認する
func TestUpdateQuota_部分更新が正しく反映される(t *testing.T) {
	updatedMaxDeployments := 30  // 更新後のデプロイメント上限
	updatedMaxTotalVolumeMB := 20480 // 更新後のボリューム総容量上限

	updatedResponse := &service.QuotaResponse{ // 更新後のレスポンスを定義する
		UserID:                   "test-user-id",
		PlanID:                   "plan-free-id",
		MaxProjects:              5,
		MaxDeployments:           updatedMaxDeployments,
		MaxReplicasPerDeployment: 5,
		MaxVolumes:               10,
		MaxVolumeSizeMB:          10240,
		MaxTotalVolumeMB:         updatedMaxTotalVolumeMB,
		InstanceLimits:           map[string]int{},
		CurrentProjects:          2,
		CurrentDeployments:       3,
		CurrentVolumes:           1,
		CurrentTotalVolumeMB:     2048,
	}

	var capturedRequest service.UpdateQuotaRequest // キャプチャしたリクエストを保持する変数
	mockService := &mockQuotaService{
		updateQuotaFunc: func(ctx context.Context, userID string, req service.UpdateQuotaRequest) (*service.QuotaResponse, error) {
			capturedRequest = req         // リクエストをキャプチャする
			return updatedResponse, nil   // 更新後のレスポンスを返す
		},
	}

	quotaHandler := NewUserQuotaHandler(mockService) // ハンドラーを生成する
	requestBodyJSON := `{"override_max_deployments": 30, "override_max_total_volume_mb": 20480}` // 部分更新リクエストボディ
	echoCtx, responseRecorder := setupEchoContext(http.MethodPut, "/api/v1/users/test-user-id/quota", requestBodyJSON, "test-user-id") // テスト用コンテキストを生成する

	err := quotaHandler.UpdateQuota(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err) // ハンドラーエラーをテスト失敗とする
	}

	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	if capturedRequest.OverrideMaxDeployments == nil || *capturedRequest.OverrideMaxDeployments != 30 { // 部分更新リクエストの override_max_deployments を確認する
		t.Errorf("期待するOverrideMaxDeployments: 30, 実際の値: %v", capturedRequest.OverrideMaxDeployments)
	}
	if capturedRequest.OverrideMaxTotalVolumeMB == nil || *capturedRequest.OverrideMaxTotalVolumeMB != 20480 { // 部分更新リクエストの override_max_total_volume_mb を確認する
		t.Errorf("期待するOverrideMaxTotalVolumeMB: 20480, 実際の値: %v", capturedRequest.OverrideMaxTotalVolumeMB)
	}
	if capturedRequest.OverrideMaxProjects != nil { // 指定していないフィールドが nil のままであることを確認する
		t.Errorf("OverrideMaxProjects は nil であるべきですが、値が設定されています: %d", *capturedRequest.OverrideMaxProjects)
	}

	var actualResponse service.QuotaResponse
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualResponse); err != nil { // レスポンスボディをデコードする
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}

	if actualResponse.MaxDeployments != updatedMaxDeployments { // 更新後のデプロイメント上限を確認する
		t.Errorf("期待するMaxDeployments: %d, 実際のMaxDeployments: %d", updatedMaxDeployments, actualResponse.MaxDeployments)
	}
	if actualResponse.MaxTotalVolumeMB != updatedMaxTotalVolumeMB { // 更新後のボリューム総容量上限を確認する
		t.Errorf("期待するMaxTotalVolumeMB: %d, 実際のMaxTotalVolumeMB: %d", updatedMaxTotalVolumeMB, actualResponse.MaxTotalVolumeMB)
	}
}
