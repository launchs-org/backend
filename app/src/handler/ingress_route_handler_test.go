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

// mockIngressRouteService は IngressRouteService のテスト用モック実装
type mockIngressRouteService struct {
	getIngressRouteFunc    func(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error)
	createIngressRouteFunc func(ctx context.Context, userID string, projectID string, req service.CreateIngressRouteRequest) (*models.IngressRoute, error)
	updateIngressRouteFunc func(ctx context.Context, userID string, projectID string, req service.UpdateIngressRouteRequest) (*models.IngressRoute, error)
	deleteIngressRouteFunc func(ctx context.Context, userID string, projectID string) error
	listRoutesFunc         func(ctx context.Context, userID string, projectID string) ([]*models.IngressRouteRoute, error)
	addRouteFunc           func(ctx context.Context, userID string, projectID string, req service.AddRouteRequest) (*models.IngressRouteRoute, error)
	updateRouteFunc        func(ctx context.Context, userID string, projectID string, routeID string, req service.UpdateRouteRequest) (*models.IngressRouteRoute, error)
	deleteRouteFunc        func(ctx context.Context, userID string, projectID string, routeID string) error
}

func (mock *mockIngressRouteService) GetIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error) {
	if mock.getIngressRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.getIngressRouteFunc(ctx, userID, projectID)
	}
	return &models.IngressRoute{ProjectID: projectID}, nil // デフォルトは空の ingress_route を返す
}

func (mock *mockIngressRouteService) CreateIngressRoute(ctx context.Context, userID string, projectID string, req service.CreateIngressRouteRequest) (*models.IngressRoute, error) {
	if mock.createIngressRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createIngressRouteFunc(ctx, userID, projectID, req)
	}
	return &models.IngressRoute{ProjectID: projectID}, nil // デフォルトは空の ingress_route を返す
}

func (mock *mockIngressRouteService) UpdateIngressRoute(ctx context.Context, userID string, projectID string, req service.UpdateIngressRouteRequest) (*models.IngressRoute, error) {
	if mock.updateIngressRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateIngressRouteFunc(ctx, userID, projectID, req)
	}
	return &models.IngressRoute{ProjectID: projectID}, nil // デフォルトは空の ingress_route を返す
}

func (mock *mockIngressRouteService) DeleteIngressRoute(ctx context.Context, userID string, projectID string) error {
	if mock.deleteIngressRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteIngressRouteFunc(ctx, userID, projectID)
	}
	return nil // デフォルトは成功を返す
}

func (mock *mockIngressRouteService) ListRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRouteRoute, error) {
	if mock.listRoutesFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.listRoutesFunc(ctx, userID, projectID)
	}
	return []*models.IngressRouteRoute{}, nil // デフォルトは空のスライスを返す
}

func (mock *mockIngressRouteService) AddRoute(ctx context.Context, userID string, projectID string, req service.AddRouteRequest) (*models.IngressRouteRoute, error) {
	if mock.addRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.addRouteFunc(ctx, userID, projectID, req)
	}
	return &models.IngressRouteRoute{Status: models.IngressRouteRouteStatusPending}, nil // デフォルトは pending ルートエントリを返す
}

func (mock *mockIngressRouteService) UpdateRoute(ctx context.Context, userID string, projectID string, routeID string, req service.UpdateRouteRequest) (*models.IngressRouteRoute, error) {
	if mock.updateRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateRouteFunc(ctx, userID, projectID, routeID, req)
	}
	return &models.IngressRouteRoute{ID: routeID}, nil // デフォルトは空のルートエントリを返す
}

func (mock *mockIngressRouteService) DeleteRoute(ctx context.Context, userID string, projectID string, routeID string) error {
	if mock.deleteRouteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteRouteFunc(ctx, userID, projectID, routeID)
	}
	return nil // デフォルトは成功を返す
}

// setupIngressRouteEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupIngressRouteEchoContext(method, path, body string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                             // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                  // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)              // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)  // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                             // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)          // Echo コンテキストを生成する
	echoCtx.Set("claim", &middlewares.AccessTokenClaim{UserID: "test-user-id"}) // テスト用クレームを設定する

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

// TestGetIngressRoute_正常に取得できる は GET /projects/:id/ingress-route で 200 が返ることを確認する
func TestGetIngressRoute_正常に取得できる(t *testing.T) {
	expectedIngressRoute := &models.IngressRoute{
		ID:        "ir-1",     // ingress_route ID を設定する
		ProjectID: "proj-1",  // project ID を設定する
		Host:      "test.example.com", // ホストを設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	mockSvc := &mockIngressRouteService{
		getIngressRouteFunc: func(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error) {
			return expectedIngressRoute, nil // 正常に ingress_route を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodGet, "/api/v1/projects/proj-1/ingress-route", "", map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.GetIngressRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualIngressRoute models.IngressRoute
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualIngressRoute); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualIngressRoute.Host != "test.example.com" { // ホストが期待と一致することを確認する
		t.Errorf("期待するホスト: test.example.com, 実際のホスト: %s", actualIngressRoute.Host)
	}
}

// TestGetIngressRoute_404が返る は ingress_route が見つからない場合に 404 が返ることを確認する
func TestGetIngressRoute_404が返る(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		getIngressRouteFunc: func(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error) {
			return nil, gorm.ErrRecordNotFound // レコードが見つからないエラーを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodGet, "/api/v1/projects/proj-1/ingress-route", "", map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.GetIngressRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestCreateIngressRoute_正常に作成できる は POST /projects/:id/ingress-route で 201 が返ることを確認する
func TestCreateIngressRoute_正常に作成できる(t *testing.T) {
	expectedIngressRoute := &models.IngressRoute{
		ID:        "ir-1",               // ingress_route ID を設定する
		ProjectID: "proj-1",            // project ID を設定する
		Host:      "custom.example.com", // ホストを設定する
		Status:    models.IngressRouteStatusPending, // ステータスを設定する
	}
	mockSvc := &mockIngressRouteService{
		createIngressRouteFunc: func(ctx context.Context, userID string, projectID string, req service.CreateIngressRouteRequest) (*models.IngressRoute, error) {
			return expectedIngressRoute, nil // 正常に ingress_route を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	requestJSON := `{"host":"custom.example.com"}`         // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodPost, "/api/v1/projects/proj-1/ingress-route", requestJSON, map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.CreateIngressRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var actualIngressRoute models.IngressRoute
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualIngressRoute); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualIngressRoute.Host != "custom.example.com" { // ホストが期待と一致することを確認する
		t.Errorf("期待するホスト: custom.example.com, 実際のホスト: %s", actualIngressRoute.Host)
	}
}

// TestDeleteIngressRoute_正常に削除できる は DELETE /projects/:id/ingress-route で 204 が返ることを確認する
func TestDeleteIngressRoute_正常に削除できる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deleteIngressRouteFunc: func(ctx context.Context, userID string, projectID string) error {
			return nil // 正常に削除を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodDelete, "/api/v1/projects/proj-1/ingress-route", "", map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.DeleteIngressRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestAddRoute_正常にルートを追加できる は POST /projects/:id/ingress-route/routes で 201 が返ることを確認する
func TestAddRoute_正常にルートを追加できる(t *testing.T) {
	expectedRoute := &models.IngressRouteRoute{
		ID:           "route-1",                            // ルートエントリ ID を設定する
		DeploymentID: "deploy-1",                          // DeploymentID を設定する
		PathPrefix:   "/api",                              // パスプレフィックスを設定する
		Port:         8080,                                // ポートを設定する
		Status:       models.IngressRouteRouteStatusPending, // ステータスを設定する
	}
	mockSvc := &mockIngressRouteService{
		addRouteFunc: func(ctx context.Context, userID string, projectID string, req service.AddRouteRequest) (*models.IngressRouteRoute, error) {
			return expectedRoute, nil // 正常にルートエントリを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	requestJSON := `{"deployment_id":"deploy-1","path_prefix":"/api","port":8080}` // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodPost, "/api/v1/projects/proj-1/ingress-route/routes", requestJSON, map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.AddRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var actualRoute models.IngressRouteRoute
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualRoute); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualRoute.Status != models.IngressRouteRouteStatusPending { // ステータスが pending であることを確認する
		t.Errorf("期待するステータス: pending, 実際のステータス: %s", actualRoute.Status)
	}
}

// TestAddRoute_DeploymentNotBelongToProjectで400が返る は ErrDeploymentNotBelongToProject 発生時に 400 が返ることを確認する
func TestAddRoute_DeploymentNotBelongToProjectで400が返る(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		addRouteFunc: func(ctx context.Context, userID string, projectID string, req service.AddRouteRequest) (*models.IngressRouteRoute, error) {
			return nil, service.ErrDeploymentNotBelongToProject // プロジェクト不一致エラーを返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	requestJSON := `{"deployment_id":"deploy-other","path_prefix":"/api","port":8080}` // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodPost, "/api/v1/projects/proj-1/ingress-route/routes", requestJSON, map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.AddRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusBadRequest { // 400 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusBadRequest, responseRecorder.Code)
	}
}

// TestDeleteRoute_正常にルートを削除できる は DELETE /projects/:id/ingress-route/routes/:routeId で 204 が返ることを確認する
func TestDeleteRoute_正常にルートを削除できる(t *testing.T) {
	mockSvc := &mockIngressRouteService{
		deleteRouteFunc: func(ctx context.Context, userID string, projectID string, routeID string) error {
			return nil // 正常に削除を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodDelete, "/api/v1/projects/proj-1/ingress-route/routes/route-1", "", map[string]string{"id": "proj-1", "routeId": "route-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.DeleteRoute(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestListRoutes_正常に一覧を取得できる は GET /projects/:id/ingress-route/routes で 200 が返ることを確認する
func TestListRoutes_正常に一覧を取得できる(t *testing.T) {
	expectedRouteList := []*models.IngressRouteRoute{
		{ID: "route-1", PathPrefix: "/api", Port: 8080, Status: models.IngressRouteRouteStatusActive},   // アクティブなルートエントリ
		{ID: "route-2", PathPrefix: "/web", Port: 3000, Status: models.IngressRouteRouteStatusPending},  // 保留中のルートエントリ
	}
	mockSvc := &mockIngressRouteService{
		listRoutesFunc: func(ctx context.Context, userID string, projectID string) ([]*models.IngressRouteRoute, error) {
			return expectedRouteList, nil // ルートエントリ一覧を返す
		},
	}

	ingressRouteHandler := NewIngressRouteHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupIngressRouteEchoContext(http.MethodGet, "/api/v1/projects/proj-1/ingress-route/routes", "", map[string]string{"id": "proj-1"}) // テスト用コンテキストを生成する

	err := ingressRouteHandler.ListRoutes(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualRouteList []*models.IngressRouteRoute
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualRouteList); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(actualRouteList) != 2 { // ルートエントリが 2 件であることを確認する
		t.Errorf("期待するルートエントリ件数: 2, 実際のルートエントリ件数: %d", len(actualRouteList))
	}
}
