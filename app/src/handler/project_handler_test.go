package handler

import (
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

// mockProjectService は ProjectService のテスト用モック実装
type mockProjectService struct {
	createProjectFunc func(ctx context.Context, userID string, req service.CreateProjectRequest) (*models.Project, error)
	listProjectsFunc  func(ctx context.Context, userID string) ([]*models.Project, error)
	getProjectFunc    func(ctx context.Context, projectID string) (*models.Project, error)
	updateProjectFunc func(ctx context.Context, projectID string, req service.UpdateProjectRequest) (*models.Project, error)
	deleteProjectFunc func(ctx context.Context, projectID string) error
}

func (mock *mockProjectService) CreateProject(ctx context.Context, userID string, req service.CreateProjectRequest) (*models.Project, error) {
	return mock.createProjectFunc(ctx, userID, req) // モック関数を呼び出す
}

func (mock *mockProjectService) ListProjects(ctx context.Context, userID string) ([]*models.Project, error) {
	return mock.listProjectsFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockProjectService) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	return mock.getProjectFunc(ctx, projectID) // モック関数を呼び出す
}

func (mock *mockProjectService) UpdateProject(ctx context.Context, projectID string, req service.UpdateProjectRequest) (*models.Project, error) {
	return mock.updateProjectFunc(ctx, projectID, req) // モック関数を呼び出す
}

func (mock *mockProjectService) DeleteProject(ctx context.Context, projectID string) error {
	return mock.deleteProjectFunc(ctx, projectID) // モック関数を呼び出す
}

// setupProjectEchoContext はテスト用の Echo コンテキストを生成するヘルパー関数
func setupProjectEchoContext(method, path, body string, userID string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                            // Echo インスタンスを生成する
	bodyReader := strings.NewReader(body)                                 // リクエストボディを設定する
	request := httptest.NewRequest(method, path, bodyReader)             // テスト用リクエストを生成する
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON) // Content-Type を JSON に設定する
	responseRecorder := httptest.NewRecorder()                            // テスト用レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, responseRecorder)         // Echo コンテキストを生成する
	echoCtx.Set("UserID", userID)                                         // ミドルウェアがセットする UserID を設定する

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

// TestListProjects_正常にproject一覧が返る は GET /projects で project 一覧が返ることを確認する
func TestListProjects_正常にproject一覧が返る(t *testing.T) {
	expectedProjects := []*models.Project{
		{ID: "project-id-1", Name: "project-1", UserID: "user-1", Status: models.ProjectStatusActive},
		{ID: "project-id-2", Name: "project-2", UserID: "user-1", Status: models.ProjectStatusActive},
	}

	mockService := &mockProjectService{
		listProjectsFunc: func(ctx context.Context, userID string) ([]*models.Project, error) {
			return expectedProjects, nil // 期待する一覧を返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                              // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodGet, "/api/v1/projects", "", "user-1", nil) // テスト用コンテキストを生成する

	err := projectHandler.ListProjects(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualProjects []*models.Project
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualProjects); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(actualProjects) != 2 { // project 数を確認する
		t.Errorf("期待する project 数: 2, 実際の project 数: %d", len(actualProjects))
	}
}

// TestCreateProject_正常にprojectが作成される は POST /projects で project が作成されることを確認する
func TestCreateProject_正常にprojectが作成される(t *testing.T) {
	expectedProject := &models.Project{
		ID:        "new-project-id",
		Name:      "my-project",
		UserID:    "user-1",
		Namespace: "my-project",
		Status:    models.ProjectStatusActive,
	}

	mockService := &mockProjectService{
		createProjectFunc: func(ctx context.Context, userID string, req service.CreateProjectRequest) (*models.Project, error) {
			return expectedProject, nil // 作成した project を返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                    // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodPost, "/api/v1/projects", `{"name":"my-project"}`, "user-1", nil) // テスト用コンテキストを生成する

	err := projectHandler.CreateProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusCreated { // 201 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusCreated, responseRecorder.Code)
	}

	var actualProject models.Project
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualProject); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualProject.Name != "my-project" { // project 名を確認する
		t.Errorf("期待する project 名: my-project, 実際の project 名: %s", actualProject.Name)
	}
	if actualProject.Status != models.ProjectStatusActive { // status が active であることを確認する
		t.Errorf("期待する status: active, 実際の status: %s", actualProject.Status)
	}
}

// TestCreateProject_同名projectは500になる は UNIQUE 制約違反で 500 が返ることを確認する
func TestCreateProject_同名projectは500になる(t *testing.T) {
	mockService := &mockProjectService{
		createProjectFunc: func(ctx context.Context, userID string, req service.CreateProjectRequest) (*models.Project, error) {
			return nil, gorm.ErrDuplicatedKey // UNIQUE 制約違反エラーを返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                         // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodPost, "/api/v1/projects", `{"name":"duplicate"}`, "user-1", nil) // テスト用コンテキストを生成する

	err := projectHandler.CreateProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusInternalServerError { // 500 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusInternalServerError, responseRecorder.Code)
	}
}

// TestGetProject_正常にproject詳細が返る は GET /projects/:id で詳細が取得できることを確認する
func TestGetProject_正常にproject詳細が返る(t *testing.T) {
	expectedProject := &models.Project{
		ID:     "project-id-1",
		Name:   "my-project",
		UserID: "user-1",
		Status: models.ProjectStatusActive,
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return expectedProject, nil // 期待する project を返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                          // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodGet, "/api/v1/projects/project-id-1", "", "user-1", map[string]string{"id": "project-id-1"}) // テスト用コンテキストを生成する

	err := projectHandler.GetProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualProject models.Project
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualProject); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualProject.ID != "project-id-1" { // project ID を確認する
		t.Errorf("期待する project ID: project-id-1, 実際の project ID: %s", actualProject.ID)
	}
}

// TestUpdateProject_正常にnameが更新される は PUT /projects/:id で name が更新できることを確認する
func TestUpdateProject_正常にnameが更新される(t *testing.T) {
	updatedName := "updated-project"
	expectedProject := &models.Project{
		ID:     "project-id-1",
		Name:   updatedName,
		UserID: "user-1",
		Status: models.ProjectStatusActive,
	}

	mockService := &mockProjectService{
		updateProjectFunc: func(ctx context.Context, projectID string, req service.UpdateProjectRequest) (*models.Project, error) {
			return expectedProject, nil // 更新後の project を返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                                                      // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodPut, "/api/v1/projects/project-id-1", `{"name":"updated-project"}`, "user-1", map[string]string{"id": "project-id-1"}) // テスト用コンテキストを生成する

	err := projectHandler.UpdateProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusOK, responseRecorder.Code)
	}

	var actualProject models.Project
	if err := json.NewDecoder(responseRecorder.Body).Decode(&actualProject); err != nil { // レスポンスをデコードする
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if actualProject.Name != updatedName { // 更新後の name を確認する
		t.Errorf("期待する name: %s, 実際の name: %s", updatedName, actualProject.Name)
	}
}

// TestDeleteProject_正常に削除される は DELETE /projects/:id で 204 が返ることを確認する
func TestDeleteProject_正常に削除される(t *testing.T) {
	mockService := &mockProjectService{
		deleteProjectFunc: func(ctx context.Context, projectID string) error {
			return nil // 削除成功を返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                                        // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodDelete, "/api/v1/projects/project-id-1", "", "user-1", map[string]string{"id": "project-id-1"}) // テスト用コンテキストを生成する

	err := projectHandler.DeleteProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNoContent { // 204 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNoContent, responseRecorder.Code)
	}
}

// TestGetProject_存在しないprojectは404になる は存在しない project ID で 404 が返ることを確認する
func TestGetProject_存在しないprojectは404になる(t *testing.T) {
	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
			return nil, gorm.ErrRecordNotFound // レコードが存在しないエラーを返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                                             // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodGet, "/api/v1/projects/nonexistent", "", "user-1", map[string]string{"id": "nonexistent"}) // テスト用コンテキストを生成する

	err := projectHandler.GetProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusNotFound { // 404 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusNotFound, responseRecorder.Code)
	}
}

// TestCreateProject_プロジェクト数がQuota超過の場合は403になる はQuota超過時に403が返ることを確認する
func TestCreateProject_プロジェクト数がQuota超過の場合は403になる(t *testing.T) {
	mockService := &mockProjectService{
		createProjectFunc: func(ctx context.Context, userID string, req service.CreateProjectRequest) (*models.Project, error) {
			return nil, &service.QuotaExceededError{Resource: "projects", Current: 3, Limit: 3} // Quota超過エラーを返す
		},
	}

	projectHandler := NewProjectHandler(mockService)                                                                            // ハンドラーを生成する
	echoCtx, responseRecorder := setupProjectEchoContext(http.MethodPost, "/api/v1/projects", `{"name":"my-project"}`, "user-1", nil) // テスト用コンテキストを生成する

	err := projectHandler.CreateProject(echoCtx) // ハンドラーを実行する
	if err != nil {
		t.Fatalf("ハンドラーがエラーを返しました: %v", err)
	}
	if responseRecorder.Code != http.StatusForbidden { // 403 が返ることを確認する
		t.Errorf("期待するステータスコード: %d, 実際のステータスコード: %d", http.StatusForbidden, responseRecorder.Code)
	}
}
