package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"handler/models"
	"handler/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockDeploymentService は DeploymentService のテスト用モック実装
type mockDeploymentService struct {
	listDeploymentsFunc  func(ctx context.Context, projectID string) ([]models.Deployment, error)
	createDeploymentFunc func(ctx context.Context, req service.CreateDeploymentRequest) (*models.Deployment, error)
	getDeploymentFunc    func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)
	updateDeploymentFunc func(ctx context.Context, userID string, deploymentID string, req service.UpdateDeploymentRequest) (*models.Deployment, error)
	deleteDeploymentFunc func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error)
	getServiceFunc       func(ctx context.Context, userID string, deploymentID string) (*models.Service, error)
	updateServiceFunc    func(ctx context.Context, userID string, deploymentID string, req service.UpdateServiceRequest) (*models.Service, error)
}

func (mock *mockDeploymentService) ListDeployments(ctx context.Context, projectID string) ([]models.Deployment, error) {
	return mock.listDeploymentsFunc(ctx, projectID) // モック関数を呼び出す
}

func (mock *mockDeploymentService) CreateDeployment(ctx context.Context, req service.CreateDeploymentRequest) (*models.Deployment, error) {
	return mock.createDeploymentFunc(ctx, req) // モック関数を呼び出す
}

func (mock *mockDeploymentService) GetDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	return mock.getDeploymentFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockDeploymentService) UpdateDeployment(ctx context.Context, userID string, deploymentID string, req service.UpdateDeploymentRequest) (*models.Deployment, error) {
	return mock.updateDeploymentFunc(ctx, userID, deploymentID, req) // モック関数を呼び出す
}

func (mock *mockDeploymentService) DeleteDeployment(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	return mock.deleteDeploymentFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockDeploymentService) GetService(ctx context.Context, userID string, deploymentID string) (*models.Service, error) {
	return mock.getServiceFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockDeploymentService) UpdateService(ctx context.Context, userID string, deploymentID string, req service.UpdateServiceRequest) (*models.Service, error) {
	return mock.updateServiceFunc(ctx, userID, deploymentID, req) // モック関数を呼び出す
}

func (mock *mockDeploymentService) CreateService(ctx context.Context, userID string, deploymentID string, req service.CreateServiceRequest) (*models.Service, error) {
	return &models.Service{DeploymentID: deploymentID}, nil // デフォルトは空の service を返す
}

func (mock *mockDeploymentService) DeleteService(ctx context.Context, userID string, deploymentID string) error {
	return nil // デフォルトは成功を返す
}

func (mock *mockDeploymentService) DiscardPending(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
	return &models.Deployment{}, nil // テストでは使用しないためデフォルトを返す
}

// setupDeploymentEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupDeploymentEchoContext(method, path, body string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                           // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)             // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON) // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                           // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)        // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                // テスト用 UserID を設定する

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

// TestCreateDeployment_正常にdeploymentが作成される は POST で status=pending、全フィールドが pending_*** に入ることを確認する
func TestCreateDeployment_正常にdeploymentが作成される(t *testing.T) {
	pendingImageID := "image-id-1" // pending_image_id に設定する ID を定義する
	expectedDeployment := &models.Deployment{
		ID:                  "deployment-id-1",              // deployment ID を設定する
		ProjectID:           "project-id-1",                 // project ID を設定する
		Name:                "my-app",                       // deployment 名を設定する
		Type:                models.DeploymentTypeImageURL,  // deployment タイプを設定する
		Status:              models.DeploymentStatusPending, // ステータスを pending に設定する
		AppStatus:           models.AppStatusPending,        // アプリステータスを pending に設定する
		PendingImageID:      &pendingImageID,                // pending image_id を設定する
		PendingInstanceSize: "small",                        // pending instance_size を設定する
		PendingReplicas:     1,                              // pending replicas を設定する
	}

	mockSvc := &mockDeploymentService{
		createDeploymentFunc: func(ctx context.Context, req service.CreateDeploymentRequest) (*models.Deployment, error) {
			return expectedDeployment, nil // 作成した deployment を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                                     // ハンドラーを生成する
	requestJSON := `{"name":"my-app","type":"image_url","image_url":"nginx:latest","instance_size":"small","replicas":1}`                                                       // リクエスト JSON を定義する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/deployments", requestJSON, map[string]string{"id": "project-id-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.CreateDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var actualDeployment models.Deployment
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualDeployment); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualDeployment.Status != models.DeploymentStatusPending { // status が pending であることを確認する
		t.Errorf("期待する status: pending, 実際の status: %s", actualDeployment.Status)
	}
	if actualDeployment.AppStatus != models.AppStatusPending { // app_status が pending であることを確認する
		t.Errorf("期待する app_status: pending, 実際の app_status: %s", actualDeployment.AppStatus)
	}
	if actualDeployment.PendingImageID == nil || *actualDeployment.PendingImageID != "image-id-1" { // pending_image_id が設定されていることを確認する
		t.Errorf("期待する pending_image_id: image-id-1, 実際の pending_image_id: %v", actualDeployment.PendingImageID)
	}
}

// TestCreateDeployment_サービスエラーで500になる はサービスエラー時に 500 が返ることを確認する
func TestCreateDeployment_サービスエラーで500になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		createDeploymentFunc: func(ctx context.Context, req service.CreateDeploymentRequest) (*models.Deployment, error) {
			return nil, errors.New("DB エラー") // エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                                                                // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/deployments", `{"name":"my-app","type":"image_url"}`, map[string]string{"id": "project-id-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.CreateDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusInternalServerError { // 500 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusInternalServerError, responseRecorder.Code)
	}
}

// TestUpdateDeployment_送ったフィールドのみpendingが更新される は PUT で送ったフィールドのみ更新されることを確認する
func TestUpdateDeployment_送ったフィールドのみpendingが更新される(t *testing.T) {
	updatedImageURL := "nginx:1.25"
	updatedImageID := "image-id-updated"
	expectedDeployment := &models.Deployment{
		ID:              "deployment-id-1", // deployment ID を設定する
		PendingImageID:  &updatedImageID,   // 更新後の pending_image_id を設定する
		PendingReplicas: 0,                 // replicas は送っていないので変化しない
	}

	var capturedRequest service.UpdateDeploymentRequest // キャプチャしたリクエストを格納する変数を定義する
	mockSvc := &mockDeploymentService{
		updateDeploymentFunc: func(ctx context.Context, userID string, deploymentID string, req service.UpdateDeploymentRequest) (*models.Deployment, error) {
			capturedRequest = req          // リクエストをキャプチャする
			return expectedDeployment, nil // 更新後の deployment を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                                                  // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPut, "/api/v1/deployments/deployment-id-1", `{"image_url":"nginx:1.25"}`, map[string]string{"id": "deployment-id-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.UpdateDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}
	if capturedRequest.ImageURL == nil || *capturedRequest.ImageURL != updatedImageURL { // image_url が送られていることを確認する
		t.Errorf("image_url が正しく送られていません: %v", capturedRequest.ImageURL)
	}
	if capturedRequest.Replicas != nil { // replicas は送っていないので nil であることを確認する
		t.Errorf("replicas は送っていないので nil であるべきです: %v", capturedRequest.Replicas)
	}
	if capturedRequest.InstanceSize != nil { // instance_size は送っていないので nil であることを確認する
		t.Errorf("instance_size は送っていないので nil であるべきです: %v", capturedRequest.InstanceSize)
	}
}

// TestUpdateDeployment_存在しないdeploymentは404になる は存在しない deployment ID で 404 が返ることを確認する
func TestUpdateDeployment_存在しないdeploymentは404になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		updateDeploymentFunc: func(ctx context.Context, userID string, deploymentID string, req service.UpdateDeploymentRequest) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // レコードが存在しないエラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                                            // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPut, "/api/v1/deployments/nonexistent", `{"image_url":"nginx:latest"}`, map[string]string{"id": "nonexistent"}) // テスト用コンテキストを生成する

	err := deploymentHandler.UpdateDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestDeleteDeployment_statusがdeletingになる は DELETE で status が deleting に変更されることを確認する
func TestDeleteDeployment_statusがdeletingになる(t *testing.T) {
	expectedDeployment := &models.Deployment{
		ID:     "deployment-id-1",               // deployment ID を設定する
		Status: models.DeploymentStatusDeleting, // status が deleting であることを確認する
	}

	mockSvc := &mockDeploymentService{
		deleteDeploymentFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
			return expectedDeployment, nil // deleting 状態の deployment を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                           // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodDelete, "/api/v1/deployments/deployment-id-1", "", map[string]string{"id": "deployment-id-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.DeleteDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualDeployment models.Deployment
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualDeployment); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualDeployment.Status != models.DeploymentStatusDeleting { // status が deleting であることを確認する
		t.Errorf("期待する status: deleting, 実際の status: %s", actualDeployment.Status)
	}
}

// TestDeleteDeployment_存在しないdeploymentは404になる は存在しない deployment ID で 404 が返ることを確認する
func TestDeleteDeployment_存在しないdeploymentは404になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		deleteDeploymentFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
			return nil, gorm.ErrRecordNotFound // レコードが存在しないエラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                   // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodDelete, "/api/v1/deployments/nonexistent", "", map[string]string{"id": "nonexistent"}) // テスト用コンテキストを生成する

	err := deploymentHandler.DeleteDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestCreateDeployment_Service作成失敗時に500になる は Service 作成失敗時に 500 が返ることを確認する
func TestCreateDeployment_Service作成失敗時に500になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		createDeploymentFunc: func(ctx context.Context, req service.CreateDeploymentRequest) (*models.Deployment, error) {
			return nil, errors.New("Service レコードの作成に失敗しました") // Service 作成失敗を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                                                                // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/deployments", `{"name":"my-app","type":"image_url"}`, map[string]string{"id": "project-id-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.CreateDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusInternalServerError { // 500 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusInternalServerError, responseRecorder.Code)
	}
}

// TestGetDeployment_正常にdeployment詳細が返る は GET /deployments/:id で詳細が取得できることを確認する
func TestGetDeployment_正常にdeployment詳細が返る(t *testing.T) {
	expectedDeployment := &models.Deployment{
		ID:        "deployment-id-1",              // deployment ID を設定する
		Name:      "my-app",                       // deployment 名を設定する
		ProjectID: "project-id-1",                 // project ID を設定する
		Status:    models.DeploymentStatusRunning, // status を設定する
	}

	mockSvc := &mockDeploymentService{
		getDeploymentFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
			return expectedDeployment, nil // 期待する deployment を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                        // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/deployment-id-1", "", map[string]string{"id": "deployment-id-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.GetDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualDeployment models.Deployment
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualDeployment); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualDeployment.ID != "deployment-id-1" { // deployment ID を確認する
		t.Errorf("期待する ID: deployment-id-1, 実際の ID: %s", actualDeployment.ID)
	}
}

// TestGetDeployment_レスポンスにapply_progressフィールドが含まれる は apply_progress がレスポンスJSONに正しく含まれることを確認する
func TestGetDeployment_レスポンスにapply_progressフィールドが含まれる(t *testing.T) {
	expectedDeployment := &models.Deployment{
		ID:        "deployment-id-2",
		Name:      "my-app-2",
		ProjectID: "project-id-1",
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusDeploying,
		ApplyProgress: []*models.DeploymentApplyProgress{
			{StepNo: 1, StepName: models.ApplyProgressStepVolume, Status: models.ApplyProgressStepStatusSkipped},
			{StepNo: 4, StepName: models.ApplyProgressStepContainer, Status: models.ApplyProgressStepStatusInProgress},
		},
	}

	mockSvc := &mockDeploymentService{
		getDeploymentFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
			return expectedDeployment, nil
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/deployment-id-2", "", map[string]string{"id": "deployment-id-2"})

	err := deploymentHandler.GetDeployment(echoCtx)
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}

	var actualDeployment models.Deployment
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualDeployment); err != nil {
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(actualDeployment.ApplyProgress) != 2 { // apply_progress が含まれることを確認する
		t.Fatalf("期待する件数: 2, 実際の件数: %d", len(actualDeployment.ApplyProgress))
	}
	if actualDeployment.ApplyProgress[0].StepName != models.ApplyProgressStepVolume { // 内容が一致することを確認する
		t.Errorf("期待するstep_name: volume, 実際: %s", actualDeployment.ApplyProgress[0].StepName)
	}
	if actualDeployment.ApplyProgress[1].Status != models.ApplyProgressStepStatusInProgress {
		t.Errorf("期待するstatus: in_progress, 実際: %s", actualDeployment.ApplyProgress[1].Status)
	}
}

// mockApplyService は ApplyServiceInterface のテスト用モック実装
type mockApplyService struct {
	applyFunc              func(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error)
	listApplyHistoriesFunc func(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error)
}

func (mock *mockApplyService) Apply(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
	return mock.applyFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

func (mock *mockApplyService) ApplyProject(ctx context.Context, userID string, projectID string) (*service.ApplyProjectResult, error) {
	return &service.ApplyProjectResult{}, nil // テストでは使用しないためデフォルト実装を返す
}

func (mock *mockApplyService) GetProjectPendingSummary(ctx context.Context, userID string, projectID string) (*service.ProjectPendingSummary, error) {
	return &service.ProjectPendingSummary{}, nil // テストでは使用しないためデフォルト実装を返す
}

func (mock *mockApplyService) ListApplyHistories(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
	return mock.listApplyHistoriesFunc(ctx, userID, deploymentID) // モック関数を呼び出す
}

// TestApplyDeployment_正常に202とworkflow_idが返る は apply 成功時に 202 Accepted と WorkflowID が返ることを確認する
func TestApplyDeployment_正常に202とworkflow_idが返る(t *testing.T) {
	expectedResult := &service.ApplyResult{
		WorkflowID: "apply-dep-1", // Temporal Workflow ID を設定する
	}

	mockApplySvc := &mockApplyService{
		applyFunc: func(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
			return expectedResult, nil // 正常系の結果を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                      // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/deployments/dep-1/apply", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ApplyDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusAccepted { // 202 Accepted が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusAccepted, responseRecorder.Code)
	}

	var actualResult service.ApplyResult
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualResult); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualResult.WorkflowID != "apply-dep-1" { // WorkflowID を確認する（Temporal 非同期化対応）
		t.Errorf("期待する WorkflowID: apply-dep-1, 実際: %s", actualResult.WorkflowID)
	}
}

// TestApplyDeployment_apply中に再applyすると409が返る は ErrAlreadyApplying のとき 409 が返ることを確認する
func TestApplyDeployment_apply中に再applyすると409が返る(t *testing.T) {
	mockApplySvc := &mockApplyService{
		applyFunc: func(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
			return nil, service.ErrAlreadyApplying // 競合エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                      // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/deployments/dep-1/apply", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ApplyDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusConflict { // 409 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusConflict, responseRecorder.Code)
	}
}

// TestApplyDeployment_存在しないdeploymentIDで404が返る は NotFound のとき 404 が返ることを確認する
func TestApplyDeployment_存在しないdeploymentIDで404が返る(t *testing.T) {
	mockApplySvc := &mockApplyService{
		applyFunc: func(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
			return nil, fmt.Errorf("deployment not found: %w", gorm.ErrRecordNotFound) // 404 エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                              // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/deployments/not-exist/apply", "", map[string]string{"id": "not-exist"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ApplyDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestApplyDeployment_k8s apply失敗時に500が返る は k8s エラーのとき 500 が返ることを確認する
func TestApplyDeployment_k8s_apply失敗時に500が返る(t *testing.T) {
	mockApplySvc := &mockApplyService{
		applyFunc: func(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
			return nil, errors.New("k8s apply: connection refused") // k8s エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                      // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/deployments/dep-1/apply", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ApplyDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusInternalServerError { // 500 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusInternalServerError, responseRecorder.Code)
	}
}

// TestGetDeployment_他ユーザーのdeploymentは403になる は ErrForbidden のとき 403 が返ることを確認する
func TestGetDeployment_他ユーザーのdeploymentは403になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		getDeploymentFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                    // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/dep-1", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.GetDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestUpdateDeployment_他ユーザーのdeploymentは403になる は ErrForbidden のとき 403 が返ることを確認する
func TestUpdateDeployment_他ユーザーのdeploymentは403になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		updateDeploymentFunc: func(ctx context.Context, userID string, deploymentID string, req service.UpdateDeploymentRequest) (*models.Deployment, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                                                // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPut, "/api/v1/deployments/dep-1", `{"image_url":"nginx:latest"}`, map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.UpdateDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestDeleteDeployment_他ユーザーのdeploymentは403になる は ErrForbidden のとき 403 が返ることを確認する
func TestDeleteDeployment_他ユーザーのdeploymentは403になる(t *testing.T) {
	mockSvc := &mockDeploymentService{
		deleteDeploymentFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Deployment, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(mockSvc, nil)                                                                                       // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodDelete, "/api/v1/deployments/dep-1", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.DeleteDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestApplyDeployment_他ユーザーのdeploymentは403になる は ErrForbidden のとき 403 が返ることを確認する
func TestApplyDeployment_他ユーザーのdeploymentは403になる(t *testing.T) {
	mockApplySvc := &mockApplyService{
		applyFunc: func(ctx context.Context, userID string, deploymentID string) (*service.ApplyResult, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                      // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPost, "/api/v1/deployments/dep-1/apply", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ApplyDeployment(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestListApplyHistories_正常に履歴一覧が返る は履歴が存在するとき 200 と一覧が返ることを確認する
func TestListApplyHistories_正常に履歴一覧が返る(t *testing.T) {
	expectedHistoryList := []*models.ApplyHistory{
		{ID: "history-id-1", DeploymentID: "dep-1", Status: models.ApplyStatusApplied}, // 1件目の履歴
		{ID: "history-id-2", DeploymentID: "dep-1", Status: models.ApplyStatusFailed},  // 2件目の履歴
	}

	mockApplySvc := &mockApplyService{
		listApplyHistoriesFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
			return expectedHistoryList, nil // 正常系の結果を返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                               // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/dep-1/apply-histories", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ListApplyHistories(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var responseBody []*models.ApplyHistory
	if err := json.NewDecoder(responseRecorder.Body).Decode(&responseBody); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(responseBody) != 2 { // 2件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(responseBody))
	}
}

// TestListApplyHistories_履歴が存在しない場合は空配列が返る は履歴が0件のとき空配列が返ることを確認する
func TestListApplyHistories_履歴が存在しない場合は空配列が返る(t *testing.T) {
	mockApplySvc := &mockApplyService{
		listApplyHistoriesFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
			return []*models.ApplyHistory{}, nil // 空スライスを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                               // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/dep-1/apply-histories", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ListApplyHistories(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}
}

// TestListApplyHistories_他ユーザーのdeploymentは403になる は所有者でない場合 403 が返ることを確認する
func TestListApplyHistories_他ユーザーのdeploymentは403になる(t *testing.T) {
	mockApplySvc := &mockApplyService{
		listApplyHistoriesFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                               // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/dep-1/apply-histories", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ListApplyHistories(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestListApplyHistories_存在しないdeploymentは404になる は deployment が存在しない場合 404 が返ることを確認する
func TestListApplyHistories_存在しないdeploymentは404になる(t *testing.T) {
	mockApplySvc := &mockApplyService{
		listApplyHistoriesFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.ApplyHistory, error) {
			return nil, gorm.ErrRecordNotFound // レコード不存在エラーを返す
		},
	}

	deploymentHandler := NewDeploymentHandler(nil, mockApplySvc)                                                                                               // ハンドラーを生成する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/api/v1/deployments/dep-1/apply-histories", "", map[string]string{"id": "dep-1"}) // テスト用コンテキストを生成する

	err := deploymentHandler.ListApplyHistories(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestGetService_正常にService設定が返る は GET /deployments/:id/service で 200 が返ることを確認する
func TestGetService_正常にService設定が返る(t *testing.T) {
	mockSvc := &mockDeploymentService{
		getServiceFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Service, error) {
			return &models.Service{DeploymentID: deploymentID, Port: 8080, TargetPort: 3000}, nil // service を返す
		},
	}
	mockApplySvc := &mockApplyService{}
	deploymentHandler := NewDeploymentHandler(mockSvc, mockApplySvc) // ハンドラーを生成する

	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/deployments/deployment-id-1/service", "", map[string]string{"id": "deployment-id-1"}) // Echo コンテキストを生成する

	err := deploymentHandler.GetService(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetService がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}
	var responseBody models.Service
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if responseBody.Port != 8080 { // ポート番号を確認する
		t.Errorf("期待する port: 8080, 実際の port: %d", responseBody.Port)
	}
}

// TestGetService_他ユーザーのDeploymentは403が返る は所有者でない場合に 403 が返ることを確認する
func TestGetService_他ユーザーのDeploymentは403が返る(t *testing.T) {
	mockSvc := &mockDeploymentService{
		getServiceFunc: func(ctx context.Context, userID string, deploymentID string) (*models.Service, error) {
			return nil, service.ErrForbidden // ErrForbidden を返す
		},
	}
	mockApplySvc := &mockApplyService{}
	deploymentHandler := NewDeploymentHandler(mockSvc, mockApplySvc) // ハンドラーを生成する

	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodGet, "/deployments/deployment-id-1/service", "", map[string]string{"id": "deployment-id-1"}) // Echo コンテキストを生成する

	err := deploymentHandler.GetService(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("GetService がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}

// TestUpdateService_pendingフィールドが更新され200が返る は PUT で pending が更新され 200 が返ることを確認する
func TestUpdateService_pendingフィールドが更新され200が返る(t *testing.T) {
	mockSvc := &mockDeploymentService{
		updateServiceFunc: func(ctx context.Context, userID string, deploymentID string, req service.UpdateServiceRequest) (*models.Service, error) {
			port := 9090                                                               // 更新後のポート番号を設定する
			return &models.Service{DeploymentID: deploymentID, PendingPort: port}, nil // 更新後の service を返す
		},
	}
	mockApplySvc := &mockApplyService{}
	deploymentHandler := NewDeploymentHandler(mockSvc, mockApplySvc) // ハンドラーを生成する

	requestBody := `{"port": 9090}`                                                                                                                                          // リクエストボディを設定する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPut, "/deployments/deployment-id-1/service", requestBody, map[string]string{"id": "deployment-id-1"}) // Echo コンテキストを生成する

	err := deploymentHandler.UpdateService(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("UpdateService がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}
	var responseBody models.Service
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if responseBody.PendingPort != 9090 { // pending_port を確認する
		t.Errorf("期待する pending_port: 9090, 実際の pending_port: %d", responseBody.PendingPort)
	}
}

// TestUpdateService_他ユーザーのDeploymentは403が返る は所有者でない場合に 403 が返ることを確認する
func TestUpdateService_他ユーザーのDeploymentは403が返る(t *testing.T) {
	mockSvc := &mockDeploymentService{
		updateServiceFunc: func(ctx context.Context, userID string, deploymentID string, req service.UpdateServiceRequest) (*models.Service, error) {
			return nil, service.ErrForbidden // ErrForbidden を返す
		},
	}
	mockApplySvc := &mockApplyService{}
	deploymentHandler := NewDeploymentHandler(mockSvc, mockApplySvc) // ハンドラーを生成する

	requestBody := `{"port": 9090}`                                                                                                                                          // リクエストボディを設定する
	echoCtx, responseRecorder := setupDeploymentEchoContext(http.MethodPut, "/deployments/deployment-id-1/service", requestBody, map[string]string{"id": "deployment-id-1"}) // Echo コンテキストを生成する

	err := deploymentHandler.UpdateService(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("UpdateService がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}
