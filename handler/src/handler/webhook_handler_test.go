package handler

import (
	"handler/middlewares"
	"handler/models"
	"handler/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockWebhookService は WebhookService のテスト用モック実装
type mockWebhookService struct {
	createWebhookFunc        func(ctx context.Context, userID string, deploymentID string, req service.CreateWebhookRequest) (*models.DeploymentWebhook, error)
	getWebhookFunc           func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error)
	deleteWebhookFunc        func(ctx context.Context, userID string, webhookID string) error
	triggerBuildByWebhookFunc func(ctx context.Context, deploymentID string, secret string, commitMessage string, author string) (*models.DeploymentBuild, error)
	getBuildByWebhookFunc    func(ctx context.Context, deploymentID string, secret string, buildID string) (*models.DeploymentBuild, error)
	applyByWebhookFunc       func(ctx context.Context, deploymentID string, secret string) (*service.ApplyResult, error)
}

func (mock *mockWebhookService) CreateWebhook(ctx context.Context, userID string, deploymentID string, req service.CreateWebhookRequest) (*models.DeploymentWebhook, error) {
	return mock.createWebhookFunc(ctx, userID, deploymentID, req) // モック関数を呼び出す
}

func (mock *mockWebhookService) GetWebhook(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error) {
	return mock.getWebhookFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockWebhookService) DeleteWebhook(ctx context.Context, userID string, webhookID string) error {
	return mock.deleteWebhookFunc(ctx, userID, webhookID) // モック関数を呼び出す
}

func (mock *mockWebhookService) TriggerBuildByWebhook(ctx context.Context, deploymentID string, secret string, commitMessage string, author string) (*models.DeploymentBuild, error) {
	if mock.triggerBuildByWebhookFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.triggerBuildByWebhookFunc(ctx, deploymentID, secret, commitMessage, author)
	}
	return &models.DeploymentBuild{ID: "build-id-1"}, nil // デフォルトはビルドレコードを返す
}

func (mock *mockWebhookService) GetBuildByWebhook(ctx context.Context, deploymentID string, secret string, buildID string) (*models.DeploymentBuild, error) {
	if mock.getBuildByWebhookFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.getBuildByWebhookFunc(ctx, deploymentID, secret, buildID)
	}
	return &models.DeploymentBuild{ID: buildID}, nil // デフォルトはビルドレコードを返す
}

func (mock *mockWebhookService) ApplyByWebhook(ctx context.Context, deploymentID string, secret string) (*service.ApplyResult, error) {
	if mock.applyByWebhookFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.applyByWebhookFunc(ctx, deploymentID, secret)
	}
	return &service.ApplyResult{}, nil // デフォルトは正常終了を返す
}

func (mock *mockWebhookService) UpdateImageAndApplyByWebhook(ctx context.Context, deploymentID string, secret string, imageURL string) (*service.ApplyResult, error) {
	return &service.ApplyResult{}, nil // テストでは使用しないためデフォルト正常終了を返す
}

// setupWebhookEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupWebhookEchoContext(method, path, body string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                            // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                 // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)             // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON) // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                            // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)         // Echo コンテキストを生成する
	echoCtx.Set("claim", &middlewares.AccessTokenClaim{UserID: "test-user-id"}) // JWT クレームを設定する
	echoCtx.Set("UserID", "test-user-id")                                       // テスト用 UserID を設定する

	if len(params) > 0 { // パスパラメータが存在する場合は設定する
		paramNames := make([]string, 0, len(params))
		paramValues := make([]string, 0, len(params))
		for paramName, paramValue := range params {
			paramNames = append(paramNames, paramName)
			paramValues = append(paramValues, paramValue)
		}
		echoCtx.SetParamNames(paramNames...)   // パラメータ名を設定する
		echoCtx.SetParamValues(paramValues...) // パラメータ値を設定する
	}

	return echoCtx, responseRecorder
}

// TestCreateWebhook_正常にwebhookが作成される は POST で webhook が作成されることを確認する
func TestCreateWebhook_正常にwebhookが作成される(t *testing.T) {
	mockSvc := &mockWebhookService{
		createWebhookFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateWebhookRequest) (*models.DeploymentWebhook, error) {
			return &models.DeploymentWebhook{
				ID:            "webhook-id-1",                      // 作成した webhook を返す
				DeploymentID:  deploymentID,
				Secret:        "generated-secret",
				GithubRepoURL: req.GithubRepoURL,
				IsActive:      true,
			}, nil
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	body := `{"github_repo_url":"https://github.com/org/repo"}`
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodPost, "/api/v1/deployments/dep-id-1/webhooks", body, map[string]string{"id": "dep-id-1"})

	err := webhookHandler.CreateWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータス: 201, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody models.DeploymentWebhook
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", err)
	}
	if responseBody.ID != "webhook-id-1" { // ID が一致することを確認する
		t.Errorf("期待する ID: webhook-id-1, 実際の ID: %s", responseBody.ID)
	}
	if responseBody.Secret == "" { // シークレットが設定されていることを確認する
		t.Error("シークレットが設定されていません")
	}
}

// TestCreateWebhook_他ユーザーのDeploymentは403が返る は他ユーザーの deployment に POST すると 403 が返ることを確認する
func TestCreateWebhook_他ユーザーのDeploymentは403が返る(t *testing.T) {
	mockSvc := &mockWebhookService{
		createWebhookFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateWebhookRequest) (*models.DeploymentWebhook, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	body := `{"github_repo_url":"https://github.com/org/repo"}`
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodPost, "/api/v1/deployments/dep-id-1/webhooks", body, map[string]string{"id": "dep-id-1"})

	err := webhookHandler.CreateWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestGetWebhook_正常にwebhookが取得される は GET で webhook が取得されることを確認する
func TestGetWebhook_正常にwebhookが取得される(t *testing.T) {
	mockSvc := &mockWebhookService{
		getWebhookFunc: func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error) {
			return &models.DeploymentWebhook{
				ID:            "webhook-id-1", // webhook を返す
				DeploymentID:  deploymentID,
				Secret:        "my-secret",
				GithubRepoURL: "https://github.com/org/repo",
				IsActive:      true,
			}, nil
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodGet, "/api/v1/deployments/dep-id-1/webhooks", "", map[string]string{"id": "dep-id-1"})

	err := webhookHandler.GetWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody models.DeploymentWebhook
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", err)
	}
	if responseBody.ID != "webhook-id-1" { // ID が一致することを確認する
		t.Errorf("期待する ID: webhook-id-1, 実際の ID: %s", responseBody.ID)
	}
}

// TestGetWebhook_他ユーザーのDeploymentは403が返る は他ユーザーの deployment の GET で 403 が返ることを確認する
func TestGetWebhook_他ユーザーのDeploymentは403が返る(t *testing.T) {
	mockSvc := &mockWebhookService{
		getWebhookFunc: func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodGet, "/api/v1/deployments/dep-id-1/webhooks", "", map[string]string{"id": "dep-id-1"})

	err := webhookHandler.GetWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteWebhook_正常に削除される は DELETE で webhook が削除されることを確認する
func TestDeleteWebhook_正常に削除される(t *testing.T) {
	mockSvc := &mockWebhookService{
		deleteWebhookFunc: func(ctx context.Context, userID string, webhookID string) error {
			return nil // 削除成功を返す
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodDelete, "/api/v1/webhooks/webhook-id-1", "", map[string]string{"id": "webhook-id-1"})

	err := webhookHandler.DeleteWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteWebhook_他ユーザーのWebhookは403が返る は他ユーザーの webhook を DELETE すると 403 が返ることを確認する
func TestDeleteWebhook_他ユーザーのWebhookは403が返る(t *testing.T) {
	mockSvc := &mockWebhookService{
		deleteWebhookFunc: func(ctx context.Context, userID string, webhookID string) error {
			return service.ErrForbidden // 権限エラーを返す
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodDelete, "/api/v1/webhooks/webhook-id-1", "", map[string]string{"id": "webhook-id-1"})

	err := webhookHandler.DeleteWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestGetWebhook_存在しないwebhookは404が返る は存在しない webhook の GET で 404 が返ることを確認する
func TestGetWebhook_存在しないwebhookは404が返る(t *testing.T) {
	mockSvc := &mockWebhookService{
		getWebhookFunc: func(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error) {
			return nil, gorm.ErrRecordNotFound // 未発見エラーを返す
		},
	}

	webhookHandler := NewWebhookHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupWebhookEchoContext(http.MethodGet, "/api/v1/deployments/dep-id-1/webhooks", "", map[string]string{"id": "dep-id-1"})

	err := webhookHandler.GetWebhook(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetWebhook がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータス: 404, 実際のステータス: %d", responseRecorder.Code)
	}
}
