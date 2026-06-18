package handler

import (
	"app/middlewares"
	"app/models"
	"app/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockEnvVarService は EnvVarService のテスト用モック実装
type mockEnvVarService struct {
	listEnvVarsFunc  func(ctx context.Context, userID string, projectID string) ([]*models.EnvVar, error)
	createEnvVarFunc func(ctx context.Context, userID string, projectID string, req service.CreateEnvVarRequest) (*models.EnvVar, error)
	updateEnvVarFunc func(ctx context.Context, userID string, envVarID string, req service.UpdateEnvVarRequest) (*models.EnvVar, error)
	deleteEnvVarFunc func(ctx context.Context, userID string, envVarID string) error
}

func (mock *mockEnvVarService) ListEnvVars(ctx context.Context, userID string, projectID string) ([]*models.EnvVar, error) {
	return mock.listEnvVarsFunc(ctx, userID, projectID) // モック関数を呼び出す
}

func (mock *mockEnvVarService) CreateEnvVar(ctx context.Context, userID string, projectID string, req service.CreateEnvVarRequest) (*models.EnvVar, error) {
	return mock.createEnvVarFunc(ctx, userID, projectID, req) // モック関数を呼び出す
}

func (mock *mockEnvVarService) UpdateEnvVar(ctx context.Context, userID string, envVarID string, req service.UpdateEnvVarRequest) (*models.EnvVar, error) {
	return mock.updateEnvVarFunc(ctx, userID, envVarID, req) // モック関数を呼び出す
}

func (mock *mockEnvVarService) DeleteEnvVar(ctx context.Context, userID string, envVarID string) error {
	return mock.deleteEnvVarFunc(ctx, userID, envVarID) // モック関数を呼び出す
}

// setupEnvVarEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupEnvVarEchoContext(method, path, body string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                            // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                 // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)             // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON) // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                            // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)         // Echo コンテキストを生成する
	echoCtx.Set("claim", &middlewares.AccessTokenClaim{UserID: "test-user-id"}) // テスト用クレームを設定する
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

// TestListEnvVars_正常に一覧が取得される は GET で env_var 一覧が返ることを確認する
func TestListEnvVars_正常に一覧が取得される(t *testing.T) {
	mockSvc := &mockEnvVarService{
		listEnvVarsFunc: func(ctx context.Context, userID string, projectID string) ([]*models.EnvVar, error) {
			return []*models.EnvVar{
				{ID: "env-var-id-1", ProjectID: "project-id-1", Key: "KEY1", Value: "value1", IsSecret: false}, // env_var を返す
				{ID: "env-var-id-2", ProjectID: "project-id-1", Key: "SECRET_KEY", Value: "secret", IsSecret: true}, // シークレット env_var を返す
			}, nil
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodGet, "/api/v1/projects/project-id-1/env-vars", "", map[string]string{"id": "project-id-1"})

	err := envVarHandler.ListEnvVars(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var responseList []map[string]interface{}
	if err := json.NewDecoder(responseRecorder.Body).Decode(&responseList); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(responseList) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(responseList))
	}
	// シークレット値がマスクされていることを確認する
	for _, item := range responseList {
		if item["is_secret"].(bool) {
			if item["value"].(string) != maskedValue { // マスクされていることを確認する
				t.Errorf("シークレット値がマスクされていません: %s", item["value"])
			}
		}
	}
}

// TestListEnvVars_他ユーザーのProjectは403が返る は他ユーザーの project へのアクセスで 403 が返ることを確認する
func TestListEnvVars_他ユーザーのProjectは403が返る(t *testing.T) {
	mockSvc := &mockEnvVarService{
		listEnvVarsFunc: func(ctx context.Context, userID string, projectID string) ([]*models.EnvVar, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodGet, "/api/v1/projects/other-project-id/env-vars", "", map[string]string{"id": "other-project-id"})

	err := envVarHandler.ListEnvVars(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestCreateEnvVar_正常にenv_varが作成される は POST で env_var が作成されることを確認する
func TestCreateEnvVar_正常にenv_varが作成される(t *testing.T) {
	expectedEnvVar := &models.EnvVar{
		ID:        "new-env-var-id",
		ProjectID: "project-id-1",
		Key:       "MY_KEY",
		Value:     "my-value",
		IsSecret:  false,
	}

	mockSvc := &mockEnvVarService{
		createEnvVarFunc: func(ctx context.Context, userID string, projectID string, req service.CreateEnvVarRequest) (*models.EnvVar, error) {
			return expectedEnvVar, nil // 作成した env_var を返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil)                                                                             // ハンドラーを生成する
	requestJSON := `{"key":"MY_KEY","value":"my-value","is_secret":false}`                                                 // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/env-vars", requestJSON, map[string]string{"id": "project-id-1"})

	err := envVarHandler.CreateEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var responseBody map[string]interface{}
	if err := json.NewDecoder(responseRecorder.Body).Decode(&responseBody); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if responseBody["key"].(string) != "MY_KEY" { // key が一致することを確認する
		t.Errorf("期待する key: MY_KEY, 実際の key: %s", responseBody["key"])
	}
}

// TestCreateEnvVar_is_secret_trueの値はマスクされる は is_secret=true の環境変数値がレスポンスでマスクされることを確認する
func TestCreateEnvVar_is_secret_trueの値はマスクされる(t *testing.T) {
	expectedEnvVar := &models.EnvVar{
		ID:        "secret-env-var-id",
		ProjectID: "project-id-1",
		Key:       "SECRET_KEY",
		Value:     "super-secret-value",
		IsSecret:  true,
	}

	mockSvc := &mockEnvVarService{
		createEnvVarFunc: func(ctx context.Context, userID string, projectID string, req service.CreateEnvVarRequest) (*models.EnvVar, error) {
			return expectedEnvVar, nil // 作成した env_var を返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil)                                                                                  // ハンドラーを生成する
	requestJSON := `{"key":"SECRET_KEY","value":"super-secret-value","is_secret":true}`                                         // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/env-vars", requestJSON, map[string]string{"id": "project-id-1"})

	err := envVarHandler.CreateEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}

	var responseBody map[string]interface{}
	if err := json.NewDecoder(responseRecorder.Body).Decode(&responseBody); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if responseBody["value"].(string) != maskedValue { // マスクされていることを確認する
		t.Errorf("シークレット値がマスクされていません: %s", responseBody["value"])
	}
}

// TestCreateEnvVar_他ユーザーのProjectは403が返る は他ユーザーの project へのアクセスで 403 が返ることを確認する
func TestCreateEnvVar_他ユーザーのProjectは403が返る(t *testing.T) {
	mockSvc := &mockEnvVarService{
		createEnvVarFunc: func(ctx context.Context, userID string, projectID string, req service.CreateEnvVarRequest) (*models.EnvVar, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil)                                                                             // ハンドラーを生成する
	requestJSON := `{"key":"KEY","value":"val","is_secret":false}`                                                         // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPost, "/api/v1/projects/other-project/env-vars", requestJSON, map[string]string{"id": "other-project"})

	err := envVarHandler.CreateEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestUpdateEnvVar_正常にenv_varが更新される は PUT で env_var が更新されることを確認する
func TestUpdateEnvVar_正常にenv_varが更新される(t *testing.T) {
	updatedKey := "UPDATED_KEY"
	expectedEnvVar := &models.EnvVar{
		ID:        "env-var-id-1",
		ProjectID: "project-id-1",
		Key:       updatedKey,
		Value:     "updated-value",
		IsSecret:  false,
	}

	mockSvc := &mockEnvVarService{
		updateEnvVarFunc: func(ctx context.Context, userID string, envVarID string, req service.UpdateEnvVarRequest) (*models.EnvVar, error) {
			return expectedEnvVar, nil // 更新した env_var を返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil)                                                                         // ハンドラーを生成する
	requestJSON := `{"key":"UPDATED_KEY","value":"updated-value"}`                                                     // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPut, "/api/v1/env-vars/env-var-id-1", requestJSON, map[string]string{"id": "env-var-id-1"})

	err := envVarHandler.UpdateEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var responseBody map[string]interface{}
	if err := json.NewDecoder(responseRecorder.Body).Decode(&responseBody); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if responseBody["key"].(string) != "UPDATED_KEY" { // key が更新されていることを確認する
		t.Errorf("期待する key: UPDATED_KEY, 実際の key: %s", responseBody["key"])
	}
}

// TestUpdateEnvVar_他ユーザーのProjectの環境変数は403が返る は他ユーザーの環境変数更新で 403 が返ることを確認する
func TestUpdateEnvVar_他ユーザーのProjectの環境変数は403が返る(t *testing.T) {
	mockSvc := &mockEnvVarService{
		updateEnvVarFunc: func(ctx context.Context, userID string, envVarID string, req service.UpdateEnvVarRequest) (*models.EnvVar, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil)                                                                         // ハンドラーを生成する
	requestJSON := `{"key":"KEY"}`                                                                                     // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPut, "/api/v1/env-vars/other-env-var-id", requestJSON, map[string]string{"id": "other-env-var-id"})

	err := envVarHandler.UpdateEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestDeleteEnvVar_正常にenv_varが削除される は DELETE で 204 が返ることを確認する
func TestDeleteEnvVar_正常にenv_varが削除される(t *testing.T) {
	mockSvc := &mockEnvVarService{
		deleteEnvVarFunc: func(ctx context.Context, userID string, envVarID string) error {
			return nil // 削除成功を返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodDelete, "/api/v1/env-vars/env-var-id-1", "", map[string]string{"id": "env-var-id-1"})

	err := envVarHandler.DeleteEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestDeleteEnvVar_他ユーザーのProjectの環境変数は403が返る は他ユーザーの環境変数削除で 403 が返ることを確認する
func TestDeleteEnvVar_他ユーザーのProjectの環境変数は403が返る(t *testing.T) {
	mockSvc := &mockEnvVarService{
		deleteEnvVarFunc: func(ctx context.Context, userID string, envVarID string) error {
			return service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodDelete, "/api/v1/env-vars/other-env-var-id", "", map[string]string{"id": "other-env-var-id"})

	err := envVarHandler.DeleteEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestDeleteEnvVar_存在しない場合は404が返る は NotFound エラーで 404 が返ることを確認する
func TestDeleteEnvVar_存在しない場合は404が返る(t *testing.T) {
	mockSvc := &mockEnvVarService{
		deleteEnvVarFunc: func(ctx context.Context, userID string, envVarID string) error {
			return gorm.ErrRecordNotFound // NotFound エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(mockSvc, nil) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodDelete, "/api/v1/env-vars/nonexistent-id", "", map[string]string{"id": "nonexistent-id"})

	err := envVarHandler.DeleteEnvVar(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// mockEnvVarMountService は EnvVarMountService のテスト用モック実装
type mockEnvVarMountService struct {
	listEnvVarMountsFunc  func(ctx context.Context, userID string, deploymentID string) ([]*models.EnvVarMount, error)
	createEnvVarMountFunc func(ctx context.Context, userID string, deploymentID string, req service.CreateEnvVarMountRequest) (*models.EnvVarMount, error)
	deleteEnvVarMountFunc func(ctx context.Context, userID string, mountID string) error
}

func (mock *mockEnvVarMountService) ListEnvVarMounts(ctx context.Context, userID string, deploymentID string) ([]*models.EnvVarMount, error) {
	return mock.listEnvVarMountsFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockEnvVarMountService) CreateEnvVarMount(ctx context.Context, userID string, deploymentID string, req service.CreateEnvVarMountRequest) (*models.EnvVarMount, error) {
	return mock.createEnvVarMountFunc(ctx, userID, deploymentID, req) // モック関数を呼び出す
}

func (mock *mockEnvVarMountService) DeleteEnvVarMount(ctx context.Context, userID string, mountID string) error {
	return mock.deleteEnvVarMountFunc(ctx, userID, mountID) // モック関数を呼び出す
}

// TestListEnvVarMounts_正常に一覧が取得される は GET でマウント設定一覧が返ることを確認する
func TestListEnvVarMounts_正常に一覧が取得される(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		listEnvVarMountsFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.EnvVarMount, error) {
			return []*models.EnvVarMount{
				{ID: "mount-id-1", DeploymentID: "deployment-id-1", EnvVarID: "env-var-id-1"}, // マウント設定を返す
			}, nil
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodGet, "/api/v1/deployments/deployment-id-1/env-var-mounts", "", map[string]string{"id": "deployment-id-1"})

	err := envVarHandler.ListEnvVarMounts(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}
}

// TestListEnvVarMounts_他ユーザーのDeploymentは403が返る は他ユーザーの deployment へのアクセスで 403 が返ることを確認する
func TestListEnvVarMounts_他ユーザーのDeploymentは403が返る(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		listEnvVarMountsFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.EnvVarMount, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodGet, "/api/v1/deployments/other-deployment-id/env-var-mounts", "", map[string]string{"id": "other-deployment-id"})

	err := envVarHandler.ListEnvVarMounts(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestCreateEnvVarMount_正常にマウント設定が作成される は POST でマウント設定が作成されることを確認する
func TestCreateEnvVarMount_正常にマウント設定が作成される(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		createEnvVarMountFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateEnvVarMountRequest) (*models.EnvVarMount, error) {
			return &models.EnvVarMount{ID: "mount-id-1", DeploymentID: deploymentID, EnvVarID: req.EnvVarID}, nil // 作成したマウント設定を返す
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc)                                                                                                // ハンドラーを生成する
	requestJSON := `{"env_var_id":"env-var-id-1","override_key":"MY_KEY"}`                                                                              // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPost, "/api/v1/deployments/deployment-id-1/env-var-mounts", requestJSON, map[string]string{"id": "deployment-id-1"})

	err := envVarHandler.CreateEnvVarMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}
}

// TestCreateEnvVarMount_重複マウントは409が返る は重複マウントで 409 が返ることを確認する
func TestCreateEnvVarMount_重複マウントは409が返る(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		createEnvVarMountFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateEnvVarMountRequest) (*models.EnvVarMount, error) {
			return nil, service.ErrDuplicateMount // 重複エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc)                                                                                                // ハンドラーを生成する
	requestJSON := `{"env_var_id":"env-var-id-1"}`                                                                                                      // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPost, "/api/v1/deployments/deployment-id-1/env-var-mounts", requestJSON, map[string]string{"id": "deployment-id-1"})

	err := envVarHandler.CreateEnvVarMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusConflict { // 409 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusConflict, responseRecorder.Code)
	}
}

// TestCreateEnvVarMount_他ユーザーのDeploymentは403が返る は他ユーザーの deployment へのマウントで 403 が返ることを確認する
func TestCreateEnvVarMount_他ユーザーのDeploymentは403が返る(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		createEnvVarMountFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateEnvVarMountRequest) (*models.EnvVarMount, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc)                                                                                                // ハンドラーを生成する
	requestJSON := `{"env_var_id":"env-var-id-1"}`                                                                                                      // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodPost, "/api/v1/deployments/other-deployment-id/env-var-mounts", requestJSON, map[string]string{"id": "other-deployment-id"})

	err := envVarHandler.CreateEnvVarMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestDeleteEnvVarMount_正常にマウント設定が削除される は DELETE でマウント設定が削除されることを確認する
func TestDeleteEnvVarMount_正常にマウント設定が削除される(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		deleteEnvVarMountFunc: func(ctx context.Context, userID string, mountID string) error {
			return nil // 削除成功を返す
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodDelete, "/api/v1/env-var-mounts/mount-id-1", "", map[string]string{"id": "mount-id-1"})

	err := envVarHandler.DeleteEnvVarMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestDeleteEnvVarMount_他ユーザーのDeploymentのマウントは403が返る は他ユーザーの deployment のマウント削除で 403 が返ることを確認する
func TestDeleteEnvVarMount_他ユーザーのDeploymentのマウントは403が返る(t *testing.T) {
	mockMountSvc := &mockEnvVarMountService{
		deleteEnvVarMountFunc: func(ctx context.Context, userID string, mountID string) error {
			return service.ErrForbidden // 権限エラーを返す
		},
	}

	envVarHandler := NewEnvVarHandler(nil, mockMountSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupEnvVarEchoContext(http.MethodDelete, "/api/v1/env-var-mounts/other-mount-id", "", map[string]string{"id": "other-mount-id"})

	err := envVarHandler.DeleteEnvVarMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}
