package handler

import (
	"handler/models"
	"handler/service"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// mockBuildService は BuildService のテスト用モック実装
type mockBuildService struct {
	triggerBuildFunc        func(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error)
	uploadBuildArchiveFunc  func(ctx context.Context, userID string, deploymentID string, fileName string, fileReader io.Reader, fileSize int64) (string, error)
	cancelBuildFunc         func(ctx context.Context, userID string, buildID string) error
	getBuildLogsFunc        func(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error)
	getBuildFunc            func(ctx context.Context, userID string, buildID string) (*models.DeploymentBuild, error)
	listBuildsFunc          func(ctx context.Context, userID string, deploymentID string) ([]models.DeploymentBuild, error)
	listBuildsByProjectFunc func(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error)
}

func (mock *mockBuildService) TriggerBuild(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) {
	return mock.triggerBuildFunc(ctx, userID, deploymentID, commitMessage, author, archiveUploadToken, buildDirectory) // モック関数を呼び出す
}

func (mock *mockBuildService) UploadBuildArchive(ctx context.Context, userID string, deploymentID string, fileName string, fileReader io.Reader, fileSize int64) (string, error) {
	if mock.uploadBuildArchiveFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.uploadBuildArchiveFunc(ctx, userID, deploymentID, fileName, fileReader, fileSize)
	}
	return "", nil // デフォルトは空文字を返す
}

func (mock *mockBuildService) CancelBuild(ctx context.Context, userID string, buildID string) error {
	return mock.cancelBuildFunc(ctx, userID, buildID) // モック関数を呼び出す
}

func (mock *mockBuildService) GetBuildLogs(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) {
	return mock.getBuildLogsFunc(ctx, userID, buildID, since) // モック関数を呼び出す
}

func (mock *mockBuildService) GetBuild(ctx context.Context, userID string, buildID string) (*models.DeploymentBuild, error) {
	if mock.getBuildFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.getBuildFunc(ctx, userID, buildID)
	}
	return &models.DeploymentBuild{ID: buildID}, nil // デフォルトはビルドレコードを返す
}

func (mock *mockBuildService) ListBuilds(ctx context.Context, userID string, deploymentID string) ([]models.DeploymentBuild, error) {
	if mock.listBuildsFunc != nil {
		return mock.listBuildsFunc(ctx, userID, deploymentID) // モック関数を呼び出す
	}
	return []models.DeploymentBuild{}, nil // デフォルトは空リストを返す
}

func (mock *mockBuildService) ListBuildsByProject(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error) {
	if mock.listBuildsByProjectFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.listBuildsByProjectFunc(ctx, userID, projectID)
	}
	return []models.DeploymentBuild{}, nil // デフォルトは空リストを返す
}

func (mock *mockBuildService) DeleteBuild(ctx context.Context, userID string, projectID string, buildID string) error {
	return nil // テストでは使用しない
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
		DeploymentID: func() *string { deploymentIDValue := "deployment-1"; return &deploymentIDValue }(), // 期待するデプロイメント ID を設定する
		Status:       models.BuildStatusPending, // 期待するステータスを設定する
	}

	mockSvc := &mockBuildService{
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) {
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
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) {
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
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) {
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
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) {
			return "line1\nline2\n", nil, nil // テスト用ログを返す
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
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) {
			capturedSince = since      // since を記録する
			return "line3\n", nil, nil // テスト用ログを返す
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
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) {
			return "", nil, service.ErrForbidden // 所有権エラーを返す
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
		getBuildLogsFunc: func(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) {
			return "", nil, nil // 呼ばれないはずなのでデフォルトを返す
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

// newListBuildsByProjectHandlerTestContext は ListBuildsByProject ハンドラーテスト用の Echo コンテキストを生成するヘルパー関数
func newListBuildsByProjectHandlerTestContext(projectID string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                                // Echo インスタンスを生成する
	request := httptest.NewRequest(http.MethodGet, "/", nil)                  // テスト用 GET リクエストを生成する
	recorder := httptest.NewRecorder()                                        // レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, recorder)                     // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                                     // テスト用 UserID を設定する
	echoCtx.SetParamNames("id")                                               // パスパラメータ名を設定する
	echoCtx.SetParamValues(projectID)                                         // パスパラメータ値を設定する
	return echoCtx, recorder
}

// TestListBuildsByProject_正常系 はプロジェクト単位のビルド一覧が正常に取得できることを確認する
func TestListBuildsByProject_正常系(t *testing.T) {
	deploymentIDValue := "deployment-1" // ポインタ用変数を宣言する
	expectedBuilds := []models.DeploymentBuild{
		{
			ID:           "build-1",         // ビルド ID を設定する
			ProjectID:    "project-1",        // プロジェクト ID を設定する
			DeploymentID: &deploymentIDValue, // デプロイメント ID を設定する
			Status:       models.BuildStatusSucceeded, // 成功ステータスを設定する
		},
	}

	mockSvc := &mockBuildService{
		listBuildsByProjectFunc: func(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error) {
			return expectedBuilds, nil // テスト用ビルド一覧を返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)                                         // ハンドラーを生成する
	echoCtx, recorder := newListBuildsByProjectHandlerTestContext("project-1")

	if err := buildHandler.ListBuildsByProject(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ListBuildsByProject() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}

	var responseBuilds []models.DeploymentBuild                                        // レスポンスボディをデコードする
	if err := json.NewDecoder(recorder.Body).Decode(&responseBuilds); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if len(responseBuilds) != 1 { // ビルド件数を確認する
		t.Errorf("期待するビルド件数 1、実際の件数 %d", len(responseBuilds))
	}
	if responseBuilds[0].ID != "build-1" { // ビルド ID を確認する
		t.Errorf("期待するビルド ID %s、実際の ID %s", "build-1", responseBuilds[0].ID)
	}
}

// TestListBuildsByProject_空リスト はビルドが存在しない場合に空リストを返すことを確認する
func TestListBuildsByProject_空リスト(t *testing.T) {
	mockSvc := &mockBuildService{
		listBuildsByProjectFunc: func(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error) {
			return []models.DeploymentBuild{}, nil // 空リストを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)                                         // ハンドラーを生成する
	echoCtx, recorder := newListBuildsByProjectHandlerTestContext("project-1")

	if err := buildHandler.ListBuildsByProject(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ListBuildsByProject() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}

	var responseBuilds []models.DeploymentBuild                                        // レスポンスボディをデコードする
	if err := json.NewDecoder(recorder.Body).Decode(&responseBuilds); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if len(responseBuilds) != 0 { // 空リストであることを確認する
		t.Errorf("期待するビルド件数 0、実際の件数 %d", len(responseBuilds))
	}
}

// TestListBuildsByProject_403_他ユーザー は他ユーザーのプロジェクトのビルド一覧取得で 403 が返ることを確認する
func TestListBuildsByProject_403_他ユーザー(t *testing.T) {
	mockSvc := &mockBuildService{
		listBuildsByProjectFunc: func(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error) {
			return nil, service.ErrForbidden // 所有権エラーを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)                                         // ハンドラーを生成する
	echoCtx, recorder := newListBuildsByProjectHandlerTestContext("project-1")

	if err := buildHandler.ListBuildsByProject(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ListBuildsByProject() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}

// newUploadBuildArchiveTestContext は UploadBuildArchive ハンドラーテスト用のmultipartリクエストによる Echo コンテキストを生成するヘルパー関数
func newUploadBuildArchiveTestContext(deploymentID string, fileName string, fileContent []byte) (echo.Context, *httptest.ResponseRecorder) {
	bodyBuffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(bodyBuffer)
	part, _ := multipartWriter.CreateFormFile("archive", fileName) // archiveフィールドを作成する
	part.Write(fileContent)                                        // ファイル内容を書き込む
	multipartWriter.Close()

	echoInstance := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/", bodyBuffer)                        // テスト用リクエストを生成する
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())               // Content-Typeを設定する
	recorder := httptest.NewRecorder()
	echoCtx := echoInstance.NewContext(request, recorder)
	echoCtx.Set("UserID", "test-user-id")
	echoCtx.SetParamNames("id")
	echoCtx.SetParamValues(deploymentID)
	return echoCtx, recorder
}

// TestUploadBuildArchive_正常系 はアーカイブアップロードが正常に処理されトークンが返ることを確認する
func TestUploadBuildArchive_正常系(t *testing.T) {
	mockSvc := &mockBuildService{
		uploadBuildArchiveFunc: func(ctx context.Context, userID string, deploymentID string, fileName string, fileReader io.Reader, fileSize int64) (string, error) {
			return "test-upload-token", nil // 正常なトークンを返す
		},
	}

	buildHandler := NewBuildHandler(mockSvc)
	echoCtx, recorder := newUploadBuildArchiveTestContext("deployment-1", "source.zip", []byte{0x50, 0x4b, 0x03, 0x04})

	if err := buildHandler.UploadBuildArchive(echoCtx); err != nil {
		t.Fatalf("UploadBuildArchive() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗しました: %v", err)
	}
	if response["upload_token"] != "test-upload-token" {
		t.Errorf("期待するトークン %s、実際のトークン %s", "test-upload-token", response["upload_token"])
	}
}

// TestUploadBuildArchive_不正な拡張子 は不正な拡張子のファイルで400が返ることを確認する
func TestUploadBuildArchive_不正な拡張子(t *testing.T) {
	mockSvc := &mockBuildService{}

	buildHandler := NewBuildHandler(mockSvc)
	echoCtx, recorder := newUploadBuildArchiveTestContext("deployment-1", "source.txt", []byte("plain text"))

	if err := buildHandler.UploadBuildArchive(echoCtx); err != nil {
		t.Fatalf("UploadBuildArchive() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusBadRequest, recorder.Code)
	}
}

// TestUploadBuildArchive_デプロイメントタイプ不一致 はサービス層のエラーが400にマッピングされることを確認する
func TestUploadBuildArchive_デプロイメントタイプ不一致(t *testing.T) {
	mockSvc := &mockBuildService{
		uploadBuildArchiveFunc: func(ctx context.Context, userID string, deploymentID string, fileName string, fileReader io.Reader, fileSize int64) (string, error) {
			return "", service.ErrDeploymentTypeMismatch
		},
	}

	buildHandler := NewBuildHandler(mockSvc)
	echoCtx, recorder := newUploadBuildArchiveTestContext("deployment-1", "source.zip", []byte{0x50, 0x4b, 0x03, 0x04})

	if err := buildHandler.UploadBuildArchive(echoCtx); err != nil {
		t.Fatalf("UploadBuildArchive() がエラーを返しました: %v", err)
	}

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusBadRequest, recorder.Code)
	}
}

// TestTriggerBuild_アーカイブフィールドバインド はArchiveUploadTokenとBuildDirectoryが正しくサービスに渡されることを確認する
func TestTriggerBuild_アーカイブフィールドバインド(t *testing.T) {
	var capturedToken, capturedDirectory string
	mockSvc := &mockBuildService{
		triggerBuildFunc: func(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) {
			capturedToken = archiveUploadToken
			capturedDirectory = buildDirectory
			return &models.DeploymentBuild{ID: "build-1"}, nil
		},
	}

	buildHandler := NewBuildHandler(mockSvc)

	echoInstance := echo.New()
	requestBody := bytes.NewReader([]byte(`{"archive_upload_token":"test-token","build_directory":"./app"}`))
	request := httptest.NewRequest(http.MethodPost, "/", requestBody)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	echoCtx := echoInstance.NewContext(request, recorder)
	echoCtx.Set("UserID", "test-user-id")
	echoCtx.SetParamNames("id")
	echoCtx.SetParamValues("deployment-1")

	if err := buildHandler.TriggerBuild(echoCtx); err != nil {
		t.Fatalf("TriggerBuild() がエラーを返しました: %v", err)
	}

	if capturedToken != "test-token" {
		t.Errorf("期待するトークン %s、実際のトークン %s", "test-token", capturedToken)
	}
	if capturedDirectory != "./app" {
		t.Errorf("期待するディレクトリ %s、実際のディレクトリ %s", "./app", capturedDirectory)
	}
}
