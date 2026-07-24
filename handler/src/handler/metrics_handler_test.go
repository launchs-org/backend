package handler

import (
	"handler/models"
	"handler/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockMetricsService は MetricsService のテスト用モック実装
type mockMetricsService struct {
	getDeploymentMetricsFunc func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error)
}

func (mock *mockMetricsService) GetDeploymentMetrics(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
	return mock.getDeploymentMetricsFunc(ctx, userID, deploymentID, limit) // モック関数を呼び出す
}

// setupMetricsEchoContext はメトリクスハンドラーテスト用の Echo コンテキストを生成するヘルパー関数
func setupMetricsEchoContext(method, path string, params map[string]string, queryParams map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                             // Echo インスタンスを生成する
	request := httptest.NewRequest(method, path, nil)                      // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)  // Content-Type を JSON に設定する

	if len(queryParams) > 0 { // クエリパラメータが存在する場合は設定する
		queryValues := request.URL.Query()
		for queryKey, queryValue := range queryParams {
			queryValues.Set(queryKey, queryValue) // クエリパラメータを設定する
		}
		request.URL.RawQuery = queryValues.Encode() // クエリ文字列をエンコードする
	}

	responseRecorder := httptest.NewRecorder()                             // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)          // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                  // テスト用 UserID を設定する

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

// TestGetDeploymentMetrics_正常にメトリクスが返る は GET /deployments/:id/metrics で正常レスポンスが返ることを確認する
func TestGetDeploymentMetrics_正常にメトリクスが返る(t *testing.T) {
	recordedAt := time.Now() // 記録日時を設定する
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			return []*models.DeploymentMetrics{
				{
					ID:            "metrics-id-1",
					DeploymentID:  "deployment-id-1",
					PodName:       "test-pod-1",
					CPUMillicores: 150,
					MemoryBytes:   134217728,
					ReadyReplicas: 1,
					TotalReplicas: 1,
					RecordedAt:    recordedAt,
				},
			}, nil // メトリクスを返す
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/deployment-id-1/metrics",
		map[string]string{"id": "deployment-id-1"},
		nil,
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 OK を確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody map[string]interface{}
	if jsonErr := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); jsonErr != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", jsonErr)
	}
	metricsList, ok := responseBody["metrics"].([]interface{}) // metrics フィールドを確認する
	if !ok {
		t.Fatal("レスポンスに metrics フィールドが存在しません")
	}
	if len(metricsList) != 1 { // 1 件返ることを確認する
		t.Errorf("期待する件数: 1, 実際の件数: %d", len(metricsList))
	}
}

// TestGetDeploymentMetrics_limitパラメータあり は limit クエリパラメータが正しく処理されることを確認する
func TestGetDeploymentMetrics_limitパラメータあり(t *testing.T) {
	receivedLimit := 0 // 受け取った limit を記録する変数
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			receivedLimit = limit        // 受け取った limit を記録する
			return []*models.DeploymentMetrics{}, nil
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/deployment-id-1/metrics",
		map[string]string{"id": "deployment-id-1"},
		map[string]string{"limit": "60"}, // limit=60 を指定する
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 OK を確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}
	if receivedLimit != 60 { // limit=60 がサービスに渡されることを確認する
		t.Errorf("期待する limit: 60, 実際の limit: %d", receivedLimit)
	}
}

// TestGetDeploymentMetrics_Deployment存在しない は deployment が存在しない場合に 404 が返ることを確認する
func TestGetDeploymentMetrics_Deployment存在しない(t *testing.T) {
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			return nil, gorm.ErrRecordNotFound // 存在しないエラーを返す
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/nonexistent/metrics",
		map[string]string{"id": "nonexistent"},
		nil,
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 を確認する
		t.Errorf("期待するステータス: 404, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestGetDeploymentMetrics_他ユーザーのDeployment は他ユーザーの deployment にアクセスした場合に 403 が返ることを確認する
func TestGetDeploymentMetrics_他ユーザーのDeployment(t *testing.T) {
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			return nil, service.ErrForbidden // 禁止エラーを返す
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/other-deployment/metrics",
		map[string]string{"id": "other-deployment"},
		nil,
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 を確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestGetDeploymentMetrics_不正なlimitパラメータ は limit に不正な値が指定された場合に 400 が返ることを確認する
func TestGetDeploymentMetrics_不正なlimitパラメータ(t *testing.T) {
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			return []*models.DeploymentMetrics{}, nil
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/deployment-id-1/metrics",
		map[string]string{"id": "deployment-id-1"},
		map[string]string{"limit": "invalid"}, // 不正な limit を指定する
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusBadRequest { // 400 を確認する
		t.Errorf("期待するステータス: 400, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestGetDeploymentMetrics_limitが上限を超えた場合は8640に丸められる は limit=99999 指定時に 8640 に丸められることを確認する
func TestGetDeploymentMetrics_limitが上限を超えた場合は8640に丸められる(t *testing.T) {
	receivedLimit := 0 // サービスに渡された limit を記録する変数
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			receivedLimit = limit // 受け取った limit を記録する
			return []*models.DeploymentMetrics{}, nil
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/deployment-id-1/metrics",
		map[string]string{"id": "deployment-id-1"},
		map[string]string{"limit": "99999"}, // 上限を超える limit を指定する
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 OK を確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}
	if receivedLimit != maxMetricsLimit { // 上限値 8640 に丸められることを確認する
		t.Errorf("期待する limit: %d, 実際の limit: %d", maxMetricsLimit, receivedLimit)
	}
}

// TestGetDeploymentMetrics_limit未指定時はデフォルト値720が使用される は limit 未指定時にデフォルト値 720 が使われることを確認する
func TestGetDeploymentMetrics_limit未指定時はデフォルト値720が使用される(t *testing.T) {
	receivedLimit := 0 // サービスに渡された limit を記録する変数
	mockSvc := &mockMetricsService{
		getDeploymentMetricsFunc: func(ctx context.Context, userID string, deploymentID string, limit int) ([]*models.DeploymentMetrics, error) {
			receivedLimit = limit // 受け取った limit を記録する
			return []*models.DeploymentMetrics{}, nil
		},
	}

	metricsHandler := NewMetricsHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupMetricsEchoContext(
		http.MethodGet,
		"/api/v1/deployments/deployment-id-1/metrics",
		map[string]string{"id": "deployment-id-1"},
		nil, // limit を指定しない
	)

	err := metricsHandler.GetDeploymentMetrics(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetDeploymentMetrics がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 OK を確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}
	if receivedLimit != defaultMetricsLimit { // デフォルト値 720 が使われることを確認する
		t.Errorf("期待する limit: %d, 実際の limit: %d", defaultMetricsLimit, receivedLimit)
	}
}
