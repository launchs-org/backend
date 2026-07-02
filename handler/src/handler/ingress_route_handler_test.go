package handler

import (
	"handler/models"
	"handler/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockIngressRouteService は IngressRouteService のテスト用モック実装
type mockIngressRouteService struct {
	listIngressRoutesFunc      func(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error)
	createIngressRouteFunc     func(ctx context.Context, userID string, projectID string, name string) (*models.IngressRoute, error)
	updateIngressRouteNameFunc func(ctx context.Context, userID string, ingressRouteID string, newName string) error
	deleteIngressRouteFunc     func(ctx context.Context, userID string, ingressRouteID string) error
	listPathRulesFunc          func(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error)
	createPathRuleFunc         func(ctx context.Context, userID string, ingressRouteID string, req service.CreatePathRuleRequest) (*models.PathRule, error)
	deletePathRuleFunc         func(ctx context.Context, userID string, pathRuleID string) error
}

func (mock *mockIngressRouteService) ListIngressRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error) {
	return mock.listIngressRoutesFunc(ctx, userID, projectID) // モック関数を呼び出す
}

func (mock *mockIngressRouteService) CreateIngressRoute(ctx context.Context, userID string, projectID string, name string) (*models.IngressRoute, error) {
	return mock.createIngressRouteFunc(ctx, userID, projectID, name) // モック関数を呼び出す
}

func (mock *mockIngressRouteService) UpdateIngressRouteName(ctx context.Context, userID string, ingressRouteID string, newName string) error {
	return mock.updateIngressRouteNameFunc(ctx, userID, ingressRouteID, newName) // モック関数を呼び出す
}

func (mock *mockIngressRouteService) DeleteIngressRoute(ctx context.Context, userID string, ingressRouteID string) error {
	return mock.deleteIngressRouteFunc(ctx, userID, ingressRouteID) // モック関数を呼び出す
}

func (mock *mockIngressRouteService) ListPathRules(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error) {
	return mock.listPathRulesFunc(ctx, userID, ingressRouteID) // モック関数を呼び出す
}

func (mock *mockIngressRouteService) CreatePathRule(ctx context.Context, userID string, ingressRouteID string, req service.CreatePathRuleRequest) (*models.PathRule, error) {
	return mock.createPathRuleFunc(ctx, userID, ingressRouteID, req) // モック関数を呼び出す
}

func (mock *mockIngressRouteService) DeletePathRule(ctx context.Context, userID string, pathRuleID string) error {
	return mock.deletePathRuleFunc(ctx, userID, pathRuleID) // モック関数を呼び出す
}

// mockApplyServiceForIngress は ApplyServiceInterface のテスト用スタブ実装
type mockApplyServiceForIngress struct{}

func (mock *mockApplyServiceForIngress) Apply(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
	return &service.ApplyResult{}, nil // テストでは使用しないためデフォルト実装を返す
}

func (mock *mockApplyServiceForIngress) ApplyProject(ctx context.Context, userID string, projectID string) error {
	return nil // テストでは使用しないためデフォルト実装を返す
}

func (mock *mockApplyServiceForIngress) ListApplyHistories(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
	return nil, nil // テストでは使用しないためデフォルト実装を返す
}

// setupIngressRouteEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupIngressRouteEchoContext(method, path, body string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                            // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                 // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)             // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON) // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                            // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)         // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                 // テスト用 UserID を設定する

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

// TestIngressRouteHandler_ListIngressRoutes_正常に一覧が返る は GET で 200 と ingress_route 一覧が返ることを確認する
func TestIngressRouteHandler_ListIngressRoutes_正常に一覧が返る(t *testing.T) {
	expectedIngressRouteList := []*models.IngressRoute{
		{ID: "ingress-route-id-1", ProjectID: "project-id-1", Host: "abc.launchs.org", Status: models.IngressRouteStatusActive},
		{ID: "ingress-route-id-2", ProjectID: "project-id-1", Host: "def.launchs.org", Status: models.IngressRouteStatusPending},
	}

	mockSvc := &mockIngressRouteService{
		listIngressRoutesFunc: func(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error) {
			return expectedIngressRouteList, nil // 一覧を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodGet, "/api/v1/projects/project-id-1/ingress-routes", "", map[string]string{"id": "project-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.ListIngressRoutes(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualList []*models.IngressRoute
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualList); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(actualList) != 2 { // 2件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(actualList))
	}
}

// TestIngressRouteHandler_ListIngressRoutes_権限がない場合は403になる は 403 が返ることを確認する
func TestIngressRouteHandler_ListIngressRoutes_権限がない場合は403になる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		listIngressRoutesFunc: func(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error) {
			return nil, service.ErrForbidden // 権限なしエラーを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodGet, "/api/v1/projects/project-id-1/ingress-routes", "", map[string]string{"id": "project-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.ListIngressRoutes(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestIngressRouteHandler_CreateIngressRoute_正常に作成される は POST で 201 と ingress_route が返ることを確認する
func TestIngressRouteHandler_CreateIngressRoute_正常に作成される(t *testing.T) {
	expectedIngressRoute := &models.IngressRoute{
		ID:        "new-ingress-route-id",           // IngressRoute ID を設定する
		ProjectID: "project-id-1",                   // project_id を設定する
		Host:      "new-ingress-route-id.launchs.org", // 自動生成されたホスト名を設定する
		Status:    models.IngressRouteStatusPending,  // ステータスを設定する
	}

	mockSvc := &mockIngressRouteService{
		createIngressRouteFunc: func(ctx context.Context, userID string, projectID string, name string) (*models.IngressRoute, error) {
			return expectedIngressRoute, nil // 作成した ingress_route を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodPost, "/api/v1/projects/project-id-1/ingress-routes", "", map[string]string{"id": "project-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.CreateIngressRoute(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var actualIngressRoute models.IngressRoute
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualIngressRoute); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualIngressRoute.Status != models.IngressRouteStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", actualIngressRoute.Status)
	}
}

// TestIngressRouteHandler_DeleteIngressRoute_正常に204が返る は DELETE で 204 が返ることを確認する
func TestIngressRouteHandler_DeleteIngressRoute_正常に204が返る(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deleteIngressRouteFunc: func(ctx context.Context, userID string, ingressRouteID string) error {
			return nil // 正常に返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodDelete, "/api/v1/ingress-routes/ingress-route-id-1", "", map[string]string{"id": "ingress-route-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.DeleteIngressRoute(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestIngressRouteHandler_DeleteIngressRoute_存在しない場合は404になる は 404 が返ることを確認する
func TestIngressRouteHandler_DeleteIngressRoute_存在しない場合は404になる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deleteIngressRouteFunc: func(ctx context.Context, userID string, ingressRouteID string) error {
			return gorm.ErrRecordNotFound // 存在しない場合は NotFound を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodDelete, "/api/v1/ingress-routes/ingress-route-id-1", "", map[string]string{"id": "ingress-route-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.DeleteIngressRoute(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestIngressRouteHandler_DeleteIngressRoute_権限がない場合は403になる は 403 が返ることを確認する
func TestIngressRouteHandler_DeleteIngressRoute_権限がない場合は403になる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deleteIngressRouteFunc: func(ctx context.Context, userID string, ingressRouteID string) error {
			return service.ErrForbidden // 権限なしエラーを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodDelete, "/api/v1/ingress-routes/ingress-route-id-1", "", map[string]string{"id": "ingress-route-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.DeleteIngressRoute(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestIngressRouteHandler_CreatePathRule_正常に作成される は POST で 201 と path_rule が返ることを確認する
func TestIngressRouteHandler_CreatePathRule_正常に作成される(t *testing.T) {
	expectedPathRule := &models.PathRule{
		ID:             "path-rule-id-1",             // PathRule ID を設定する
		IngressRouteID: "ingress-route-id-1",         // ingress_route_id を設定する
		PathPrefix:     "/api",                       // パスプレフィックスを設定する
		ServiceID:      "service-id-1",               // service_id を設定する
		Status:         models.PathRuleStatusPending, // ステータスを設定する
	}

	mockSvc := &mockIngressRouteService{
		createPathRuleFunc: func(ctx context.Context, userID string, ingressRouteID string, req service.CreatePathRuleRequest) (*models.PathRule, error) {
			return expectedPathRule, nil // 作成した path_rule を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	requestJSON := `{"path_prefix":"/api","service_id":"service-id-1"}`    // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodPost, "/api/v1/ingress-routes/ingress-route-id-1/path-rules", requestJSON, map[string]string{"id": "ingress-route-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.CreatePathRule(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var actualPathRule models.PathRule
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualPathRule); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualPathRule.Status != models.PathRuleStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", actualPathRule.Status)
	}
	if actualPathRule.PathPrefix != "/api" { // path_prefix が一致することを確認する
		t.Errorf("期待する path_prefix: /api, 実際の path_prefix: %s", actualPathRule.PathPrefix)
	}
}

// TestIngressRouteHandler_DeletePathRule_正常に204が返る は DELETE で 204 が返ることを確認する
func TestIngressRouteHandler_DeletePathRule_正常に204が返る(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deletePathRuleFunc: func(ctx context.Context, userID string, pathRuleID string) error {
			return nil // 正常に返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodDelete, "/api/v1/ingress-routes/ingress-route-id-1/path-rules/path-rule-id-1", "",
		map[string]string{"id": "ingress-route-id-1", "pathRuleID": "path-rule-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.DeletePathRule(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestIngressRouteHandler_DeletePathRule_他ユーザーのリソースは403になる は 403 が返ることを確認する
func TestIngressRouteHandler_DeletePathRule_他ユーザーのリソースは403になる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deletePathRuleFunc: func(ctx context.Context, userID string, pathRuleID string) error {
			return service.ErrForbidden // 権限なしエラーを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodDelete, "/api/v1/ingress-routes/ingress-route-id-1/path-rules/path-rule-id-1", "",
		map[string]string{"id": "ingress-route-id-1", "pathRuleID": "path-rule-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.DeletePathRule(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestIngressRouteHandler_ListPathRules_正常に一覧が返る は GET で 200 と path_rule 一覧が返ることを確認する
func TestIngressRouteHandler_ListPathRules_正常に一覧が返る(t *testing.T) {
	expectedPathRuleList := []*models.PathRule{
		{ID: "path-rule-id-1", PathPrefix: "/api", Status: models.PathRuleStatusActive},
		{ID: "path-rule-id-2", PathPrefix: "/web", Status: models.PathRuleStatusPending},
	}

	mockSvc := &mockIngressRouteService{
		listPathRulesFunc: func(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error) {
			return expectedPathRuleList, nil // 一覧を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodGet, "/api/v1/ingress-routes/ingress-route-id-1/path-rules", "", map[string]string{"id": "ingress-route-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.ListPathRules(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualPathRuleList []*models.PathRule
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualPathRuleList); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(actualPathRuleList) != 2 { // 2件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(actualPathRuleList))
	}
}

// TestIngressRouteHandler_CreatePathRule_サービスエラーで500になる はサービスエラー時に 500 が返ることを確認する
func TestIngressRouteHandler_CreatePathRule_サービスエラーで500になる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		createPathRuleFunc: func(ctx context.Context, userID string, ingressRouteID string, req service.CreatePathRuleRequest) (*models.PathRule, error) {
			return nil, errors.New("DB エラー") // エラーを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc, &mockApplyServiceForIngress{}) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(
		http.MethodPost, "/api/v1/ingress-routes/ingress-route-id-1/path-rules",
		`{"path_prefix":"/api","service_id":"service-id-1"}`,
		map[string]string{"id": "ingress-route-id-1"},
	) // テスト用コンテキストを生成する

	if err := ingressRouteHandler.CreatePathRule(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusInternalServerError { // 500 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusInternalServerError, responseRecorder.Code)
	}
}
