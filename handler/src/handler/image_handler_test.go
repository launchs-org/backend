package handler

import (
	"handler/models"
	"handler/service"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// mockImageService は ImageService のテスト用モック実装
type mockImageService struct {
	listImagesByProjectFunc func(ctx context.Context, userID string, projectID string) ([]models.Image, error)
	getImageFunc            func(ctx context.Context, userID string, imageID string) (*models.Image, error)
	deleteImageFunc         func(ctx context.Context, userID string, projectID string, imageID string) error
}

func (mock *mockImageService) ListImagesByProject(ctx context.Context, userID string, projectID string) ([]models.Image, error) {
	return mock.listImagesByProjectFunc(ctx, userID, projectID) // モック関数を呼び出す
}

func (mock *mockImageService) GetImage(ctx context.Context, userID string, imageID string) (*models.Image, error) {
	return mock.getImageFunc(ctx, userID, imageID) // モック関数を呼び出す
}

func (mock *mockImageService) DeleteImage(ctx context.Context, userID string, projectID string, imageID string) error {
	return mock.deleteImageFunc(ctx, userID, projectID, imageID) // モック関数を呼び出す
}

// newImageHandlerTestContext は ImageHandler テスト用の Echo コンテキストを生成するヘルパー関数
func newImageHandlerTestContext(method string, projectID string, imageID string) (echo.Context, *httptest.ResponseRecorder) {
	echoInstance := echo.New()                                // Echo インスタンスを生成する
	request := httptest.NewRequest(method, "/", nil)          // テスト用リクエストを生成する
	recorder := httptest.NewRecorder()                        // レスポンスレコーダーを生成する
	echoCtx := echoInstance.NewContext(request, recorder)      // Echo コンテキストを生成する
	echoCtx.Set("UserID", "test-user-id")                     // テスト用 UserID を設定する
	echoCtx.SetParamNames("id", "imageId")                    // パスパラメータ名を設定する
	echoCtx.SetParamValues(projectID, imageID)                // パスパラメータ値を設定する
	return echoCtx, recorder
}

// TestListImagesByProject_正常系 はイメージ一覧が正常に取得できることを確認する
func TestListImagesByProject_正常系(t *testing.T) {
	mockSvc := &mockImageService{
		listImagesByProjectFunc: func(ctx context.Context, userID string, projectID string) ([]models.Image, error) {
			return []models.Image{{ID: "image-1", ProjectID: projectID}}, nil // イメージ一覧を返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodGet, "project-1", "")

	if err := imageHandler.ListImagesByProject(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ListImagesByProject() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}
}

// TestListImagesByProject_権限エラー は権限エラー時に403が返ることを確認する
func TestListImagesByProject_権限エラー(t *testing.T) {
	mockSvc := &mockImageService{
		listImagesByProjectFunc: func(ctx context.Context, userID string, projectID string) ([]models.Image, error) {
			return nil, service.ErrForbidden // 権限エラーを返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodGet, "project-1", "")

	if err := imageHandler.ListImagesByProject(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("ListImagesByProject() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}

// TestGetImage_正常系 はイメージが正常に取得できることを確認する
func TestGetImage_正常系(t *testing.T) {
	mockSvc := &mockImageService{
		getImageFunc: func(ctx context.Context, userID string, imageID string) (*models.Image, error) {
			return &models.Image{ID: imageID}, nil // イメージレコードを返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodGet, "", "image-1")

	if err := imageHandler.GetImage(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetImage() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusOK { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusOK, recorder.Code)
	}
}

// TestGetImage_NotFound はイメージが存在しない場合に404が返ることを確認する
func TestGetImage_NotFound(t *testing.T) {
	mockSvc := &mockImageService{
		getImageFunc: func(ctx context.Context, userID string, imageID string) (*models.Image, error) {
			return nil, gorm.ErrRecordNotFound // レコードなしエラーを返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodGet, "", "image-1")

	if err := imageHandler.GetImage(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("GetImage() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusNotFound { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusNotFound, recorder.Code)
	}
}

// TestDeleteImage_正常系 はイメージが正常に削除できることを確認する
func TestDeleteImage_正常系(t *testing.T) {
	mockSvc := &mockImageService{
		deleteImageFunc: func(ctx context.Context, userID string, projectID string, imageID string) error {
			return nil // 正常終了を返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodDelete, "project-1", "image-1")

	if err := imageHandler.DeleteImage(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("DeleteImage() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusNoContent { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusNoContent, recorder.Code)
	}
}

// TestDeleteImage_使用中エラー は使用中のイメージ削除時に409が返ることを確認する
func TestDeleteImage_使用中エラー(t *testing.T) {
	mockSvc := &mockImageService{
		deleteImageFunc: func(ctx context.Context, userID string, projectID string, imageID string) error {
			return service.ErrImageInUse // 使用中エラーを返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodDelete, "project-1", "image-1")

	if err := imageHandler.DeleteImage(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("DeleteImage() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusConflict { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusConflict, recorder.Code)
	}
}

// TestDeleteImage_権限エラー は権限エラー時に403が返ることを確認する
func TestDeleteImage_権限エラー(t *testing.T) {
	mockSvc := &mockImageService{
		deleteImageFunc: func(ctx context.Context, userID string, projectID string, imageID string) error {
			return service.ErrForbidden // 権限エラーを返す
		},
	}
	imageHandler := NewImageHandler(mockSvc) // ハンドラーを生成する
	echoCtx, recorder := newImageHandlerTestContext(http.MethodDelete, "project-1", "image-1")

	if err := imageHandler.DeleteImage(echoCtx); err != nil { // ハンドラーを実行する
		t.Fatalf("DeleteImage() がエラーを返しました: %v", err)
	}
	if recorder.Code != http.StatusForbidden { // ステータスコードを確認する
		t.Errorf("期待するステータスコード %d、実際のコード %d", http.StatusForbidden, recorder.Code)
	}
}
