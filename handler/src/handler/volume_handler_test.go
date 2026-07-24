package handler

import (
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

// mockVolumeService は VolumeService のテスト用モック実装
type mockVolumeService struct {
	listVolumesFunc        func(ctx context.Context, userID string, projectID string) ([]*models.Volume, error)
	createVolumeFunc       func(ctx context.Context, userID string, projectID string, req service.CreateVolumeRequest) (*models.Volume, error)
	deleteVolumeFunc       func(ctx context.Context, userID string, volumeID string) error
	listVolumeMountsFunc   func(ctx context.Context, userID string, deploymentID string) ([]*models.VolumeMount, error)
	createVolumeMountFunc  func(ctx context.Context, userID string, deploymentID string, req service.CreateVolumeMountRequest) (*models.VolumeMount, error)
	deleteVolumeMountFunc  func(ctx context.Context, userID string, mountID string) error
}

func (mock *mockVolumeService) ListVolumes(ctx context.Context, userID string, projectID string) ([]*models.Volume, error) {
	return mock.listVolumesFunc(ctx, userID, projectID) // モック関数を呼び出す
}

func (mock *mockVolumeService) CreateVolume(ctx context.Context, userID string, projectID string, req service.CreateVolumeRequest) (*models.Volume, error) {
	return mock.createVolumeFunc(ctx, userID, projectID, req) // モック関数を呼び出す
}

func (mock *mockVolumeService) DeleteVolume(ctx context.Context, userID string, volumeID string) error {
	return mock.deleteVolumeFunc(ctx, userID, volumeID) // モック関数を呼び出す
}

func (mock *mockVolumeService) ListVolumeMounts(ctx context.Context, userID string, deploymentID string) ([]*models.VolumeMount, error) {
	if mock.listVolumeMountsFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.listVolumeMountsFunc(ctx, userID, deploymentID)
	}
	return []*models.VolumeMount{}, nil // デフォルトは空一覧を返す
}

func (mock *mockVolumeService) CreateVolumeMount(ctx context.Context, userID string, deploymentID string, req service.CreateVolumeMountRequest) (*models.VolumeMount, error) {
	if mock.createVolumeMountFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createVolumeMountFunc(ctx, userID, deploymentID, req)
	}
	return &models.VolumeMount{}, nil // デフォルトは空のマウント設定を返す
}

func (mock *mockVolumeService) DeleteVolumeMount(ctx context.Context, userID string, mountID string) error {
	if mock.deleteVolumeMountFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteVolumeMountFunc(ctx, userID, mountID)
	}
	return nil // デフォルトは nil を返す
}

// setupVolumeEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupVolumeEchoContext(method, path, body string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                            // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                 // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)             // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON) // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                            // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)         // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id") // テスト用 UserID を設定する

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

// TestListVolumes_正常に一覧が取得される は GET で volume 一覧が返ることを確認する
func TestListVolumes_正常に一覧が取得される(t *testing.T) {
	mockSvc := &mockVolumeService{
		listVolumesFunc: func(ctx context.Context, userID string, projectID string) ([]*models.Volume, error) {
			return []*models.Volume{
				{ID: "volume-id-1", ProjectID: "project-id-1", Name: "vol-a", SizeMB: 512},  // volume を返す
				{ID: "volume-id-2", ProjectID: "project-id-1", Name: "vol-b", SizeMB: 1024}, // volume を返す
			}, nil
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodGet, "/api/v1/projects/project-id-1/volumes", "", map[string]string{"id": "project-id-1"})

	err := volumeHandler.ListVolumes(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ListVolumes がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody []*models.Volume
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", err)
	}
	if len(responseBody) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(responseBody))
	}
}

// TestListVolumes_他ユーザーは403が返る は他ユーザーのプロジェクトにアクセスすると 403 が返ることを確認する
func TestListVolumes_他ユーザーは403が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		listVolumesFunc: func(ctx context.Context, userID string, projectID string) ([]*models.Volume, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodGet, "/api/v1/projects/project-id-1/volumes", "", map[string]string{"id": "project-id-1"})

	err := volumeHandler.ListVolumes(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ListVolumes がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestCreateVolume_正常にvolumeが作成される は POST で volume が作成されることを確認する
func TestCreateVolume_正常にvolumeが作成される(t *testing.T) {
	mockSvc := &mockVolumeService{
		createVolumeFunc: func(ctx context.Context, userID string, projectID string, req service.CreateVolumeRequest) (*models.Volume, error) {
			return &models.Volume{
				ID:        "new-volume-id",              // 作成した volume を返す
				ProjectID: projectID,
				Name:      req.Name,
				SizeMB:    req.SizeMB,
				Status:    models.VolumeStatusPending,
			}, nil
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	body := `{"name":"my-volume","size_mb":512}`
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/volumes", body, map[string]string{"id": "project-id-1"})

	err := volumeHandler.CreateVolume(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateVolume がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータス: 201, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody models.Volume
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", err)
	}
	if responseBody.ID != "new-volume-id" { // ID が返ることを確認する
		t.Errorf("期待する ID: new-volume-id, 実際の ID: %s", responseBody.ID)
	}
}

// TestCreateVolume_他ユーザーは403が返る は他ユーザーのプロジェクトに POST すると 403 が返ることを確認する
func TestCreateVolume_他ユーザーは403が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		createVolumeFunc: func(ctx context.Context, userID string, projectID string, req service.CreateVolumeRequest) (*models.Volume, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	body := `{"name":"vol","size_mb":512}`
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodPost, "/api/v1/projects/project-id-1/volumes", body, map[string]string{"id": "project-id-1"})

	err := volumeHandler.CreateVolume(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateVolume がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteVolume_正常にvolumeが削除される は DELETE で volume が削除されることを確認する
func TestDeleteVolume_正常にvolumeが削除される(t *testing.T) {
	mockSvc := &mockVolumeService{
		deleteVolumeFunc: func(ctx context.Context, userID string, volumeID string) error {
			return nil // 削除成功を返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodDelete, "/api/v1/volumes/volume-id-1", "", map[string]string{"id": "volume-id-1"})

	err := volumeHandler.DeleteVolume(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteVolume がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータス: 204, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteVolume_他ユーザーは403が返る は他ユーザーの volume を DELETE すると 403 が返ることを確認する
func TestDeleteVolume_他ユーザーは403が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		deleteVolumeFunc: func(ctx context.Context, userID string, volumeID string) error {
			return service.ErrForbidden // 権限エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodDelete, "/api/v1/volumes/volume-id-1", "", map[string]string{"id": "volume-id-1"})

	err := volumeHandler.DeleteVolume(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteVolume がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteVolume_存在しないvolumeは404が返る は存在しない volume を DELETE すると 404 が返ることを確認する
func TestDeleteVolume_存在しないvolumeは404が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		deleteVolumeFunc: func(ctx context.Context, userID string, volumeID string) error {
			return gorm.ErrRecordNotFound // レコードなしエラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodDelete, "/api/v1/volumes/nonexistent-id", "", map[string]string{"id": "nonexistent-id"})

	err := volumeHandler.DeleteVolume(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteVolume がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータス: 404, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestListVolumeMounts_正常に一覧が取得される は GET でマウント一覧が返ることを確認する
func TestListVolumeMounts_正常に一覧が取得される(t *testing.T) {
	mockSvc := &mockVolumeService{
		listVolumeMountsFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.VolumeMount, error) {
			return []*models.VolumeMount{
				{ID: "mount-id-1", DeploymentID: "deployment-id-1", VolumeID: "volume-id-1", MountPath: "/data"}, // マウント設定を返す
				{ID: "mount-id-2", DeploymentID: "deployment-id-1", VolumeID: "volume-id-2", MountPath: "/logs"}, // マウント設定を返す
			}, nil
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodGet, "/api/v1/deployments/deployment-id-1/volume-mounts", "", map[string]string{"id": "deployment-id-1"})

	err := volumeHandler.ListVolumeMounts(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ListVolumeMounts がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // 200 が返ることを確認する
		t.Errorf("期待するステータス: 200, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody []*models.VolumeMount
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", err)
	}
	if len(responseBody) != 2 { // 2 件返ることを確認する
		t.Errorf("期待する件数: 2, 実際の件数: %d", len(responseBody))
	}
}

// TestListVolumeMounts_他ユーザーは403が返る は他ユーザーが 403 を受け取ることを確認する
func TestListVolumeMounts_他ユーザーは403が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		listVolumeMountsFunc: func(ctx context.Context, userID string, deploymentID string) ([]*models.VolumeMount, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodGet, "/api/v1/deployments/deployment-id-1/volume-mounts", "", map[string]string{"id": "deployment-id-1"})

	err := volumeHandler.ListVolumeMounts(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ListVolumeMounts がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestCreateVolumeMount_正常にマウント設定が作成される は POST でマウント設定が作成されることを確認する
func TestCreateVolumeMount_正常にマウント設定が作成される(t *testing.T) {
	mockSvc := &mockVolumeService{
		createVolumeMountFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateVolumeMountRequest) (*models.VolumeMount, error) {
			return &models.VolumeMount{
				ID:           "new-mount-id",   // 作成したマウント設定を返す
				DeploymentID: deploymentID,
				VolumeID:     req.VolumeID,
				MountPath:    req.MountPath,
				Status:       models.VolumeMountStatusPending,
			}, nil
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	body := `{"volume_id":"volume-id-1","mount_path":"/data"}`
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodPost, "/api/v1/deployments/deployment-id-1/volume-mounts", body, map[string]string{"id": "deployment-id-1"})

	err := volumeHandler.CreateVolumeMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateVolumeMount がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータス: 201, 実際のステータス: %d", responseRecorder.Code)
	}

	var responseBody models.VolumeMount
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); err != nil { // レスポンスをパースする
		t.Fatalf("レスポンスのパースに失敗しました: %v", err)
	}
	if responseBody.ID != "new-mount-id" { // ID が返ることを確認する
		t.Errorf("期待する ID: new-mount-id, 実際の ID: %s", responseBody.ID)
	}
}

// TestCreateVolumeMount_重複MountPathは409が返る は重複マウントパスで 409 が返ることを確認する
func TestCreateVolumeMount_重複MountPathは409が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		createVolumeMountFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateVolumeMountRequest) (*models.VolumeMount, error) {
			return nil, service.ErrDuplicateVolumeMount // 重複エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	body := `{"volume_id":"volume-id-1","mount_path":"/data"}`
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodPost, "/api/v1/deployments/deployment-id-1/volume-mounts", body, map[string]string{"id": "deployment-id-1"})

	err := volumeHandler.CreateVolumeMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateVolumeMount がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusConflict { // 409 が返ることを確認する
		t.Errorf("期待するステータス: 409, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestCreateVolumeMount_他ユーザーは403が返る は他ユーザーが 403 を受け取ることを確認する
func TestCreateVolumeMount_他ユーザーは403が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		createVolumeMountFunc: func(ctx context.Context, userID string, deploymentID string, req service.CreateVolumeMountRequest) (*models.VolumeMount, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	body := `{"volume_id":"volume-id-1","mount_path":"/data"}`
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodPost, "/api/v1/deployments/deployment-id-1/volume-mounts", body, map[string]string{"id": "deployment-id-1"})

	err := volumeHandler.CreateVolumeMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("CreateVolumeMount がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteVolumeMount_正常にマウント設定が削除される は DELETE でマウント設定が削除されることを確認する
func TestDeleteVolumeMount_正常にマウント設定が削除される(t *testing.T) {
	mockSvc := &mockVolumeService{
		deleteVolumeMountFunc: func(ctx context.Context, userID string, mountID string) error {
			return nil // 削除成功を返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodDelete, "/api/v1/volume-mounts/mount-id-1", "", map[string]string{"id": "mount-id-1"})

	err := volumeHandler.DeleteVolumeMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteVolumeMount がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータス: 204, 実際のステータス: %d", responseRecorder.Code)
	}
}

// TestDeleteVolumeMount_他ユーザーは403が返る は他ユーザーが 403 を受け取ることを確認する
func TestDeleteVolumeMount_他ユーザーは403が返る(t *testing.T) {
	mockSvc := &mockVolumeService{
		deleteVolumeMountFunc: func(ctx context.Context, userID string, mountID string) error {
			return service.ErrForbidden // 権限エラーを返す
		},
	}

	volumeHandler := NewVolumeHandler(mockSvc) // ハンドラーを生成する
	echoCtx, responseRecorder := setupVolumeEchoContext(http.MethodDelete, "/api/v1/volume-mounts/mount-id-1", "", map[string]string{"id": "mount-id-1"})

	err := volumeHandler.DeleteVolumeMount(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("DeleteVolumeMount がエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータス: 403, 実際のステータス: %d", responseRecorder.Code)
	}
}
