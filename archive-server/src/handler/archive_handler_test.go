package handler

import (
	"archive-server/storage"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestArchiveHandler_UploadAndDownload はアップロードしたアーカイブをダウンロードできることを確認する
func TestArchiveHandler_UploadAndDownload(t *testing.T) {
	fileStorage := storage.NewFileStorage(t.TempDir())
	archiveHandler := NewArchiveHandler(fileStorage)
	echoRouter := echo.New()

	content := "encrypted-archive-content"

	uploadRequest := httptest.NewRequest(http.MethodPost, "/archives", strings.NewReader(content))
	uploadRecorder := httptest.NewRecorder()
	uploadCtx := echoRouter.NewContext(uploadRequest, uploadRecorder)

	if err := archiveHandler.Upload(uploadCtx); err != nil { // アップロードを実行する
		t.Fatalf("Upload() がエラーを返しました: %v", err) // アップロード失敗時はテスト失敗とする
	}
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("期待するステータスコードは201ですが %d でした", uploadRecorder.Code) // ステータス不一致時はテスト失敗とする
	}

	var uploadBody map[string]string
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &uploadBody); err != nil {
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	uploadedID := uploadBody["id"]
	if uploadedID == "" {
		t.Fatal("アップロードレスポンスにIDが含まれていません") // ID未取得時はテスト失敗とする
	}

	downloadRequest := httptest.NewRequest(http.MethodGet, "/archives/"+uploadedID, nil)
	downloadRecorder := httptest.NewRecorder()
	downloadCtx := echoRouter.NewContext(downloadRequest, downloadRecorder)
	downloadCtx.SetParamNames("id")
	downloadCtx.SetParamValues(uploadedID)

	if err := archiveHandler.Download(downloadCtx); err != nil { // ダウンロードを実行する
		t.Fatalf("Download() がエラーを返しました: %v", err) // ダウンロード失敗時はテスト失敗とする
	}
	if downloadRecorder.Body.String() != content {
		t.Fatalf("ダウンロード内容が一致しません: got=%s, want=%s", downloadRecorder.Body.String(), content) // 内容不一致時はテスト失敗とする
	}

	if _, err := fileStorage.Open(uploadedID); err != storage.ErrNotFound { // ワンタイムダウンロードのため配信後は削除されているべき
		t.Fatal("ダウンロード後もアーカイブが残っています") // 削除されていない場合はテスト失敗とする
	}
}

// TestArchiveHandler_DownloadNotFound は存在しないIDのダウンロードで404を返すことを確認する
func TestArchiveHandler_DownloadNotFound(t *testing.T) {
	fileStorage := storage.NewFileStorage(t.TempDir())
	archiveHandler := NewArchiveHandler(fileStorage)
	echoRouter := echo.New()

	downloadRequest := httptest.NewRequest(http.MethodGet, "/archives/nonexistent-id", nil)
	downloadRecorder := httptest.NewRecorder()
	downloadCtx := echoRouter.NewContext(downloadRequest, downloadRecorder)
	downloadCtx.SetParamNames("id")
	downloadCtx.SetParamValues("nonexistent-id")

	if err := archiveHandler.Download(downloadCtx); err != nil {
		t.Fatalf("Download() がエラーを返しました: %v", err)
	}
	if downloadRecorder.Code != http.StatusNotFound {
		t.Fatalf("期待するステータスコードは404ですが %d でした", downloadRecorder.Code) // ステータス不一致時はテスト失敗とする
	}
}

// TestArchiveHandler_Delete はアップロードしたアーカイブを削除できることを確認する
func TestArchiveHandler_Delete(t *testing.T) {
	fileStorage := storage.NewFileStorage(t.TempDir())
	archiveHandler := NewArchiveHandler(fileStorage)
	echoRouter := echo.New()

	id, err := fileStorage.Save(strings.NewReader("data")) // 事前にアーカイブを保存しておく
	if err != nil {
		t.Fatalf("事前準備の保存に失敗しました: %v", err)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/archives/"+id, nil)
	deleteRecorder := httptest.NewRecorder()
	deleteCtx := echoRouter.NewContext(deleteRequest, deleteRecorder)
	deleteCtx.SetParamNames("id")
	deleteCtx.SetParamValues(id)

	if err := archiveHandler.Delete(deleteCtx); err != nil { // 削除を実行する
		t.Fatalf("Delete() がエラーを返しました: %v", err) // 削除失敗時はテスト失敗とする
	}
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("期待するステータスコードは204ですが %d でした", deleteRecorder.Code) // ステータス不一致時はテスト失敗とする
	}

	if _, err := fileStorage.Open(id); err != storage.ErrNotFound {
		t.Fatal("削除後もアーカイブが残っています") // 削除されていない場合はテスト失敗とする
	}
}
