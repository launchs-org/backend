package fileio

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadSuccess(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) { // モックlitterboxサーバーを起動する
		if err := request.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("マルチパートフォームの解析に失敗しました: %v", err)
		}
		if got := request.FormValue("reqtype"); got != "fileupload" { // reqtypeフィールドが指定されていることを確認する
			t.Errorf("reqtypeフィールドが期待値と異なります: got=%s", got)
		}
		if got := request.FormValue("time"); got != uploadExpiry { // timeフィールドが指定されていることを確認する
			t.Errorf("timeフィールドが期待値と異なります: got=%s", got)
		}

		fileContent, _, err := request.FormFile("fileToUpload") // アップロードされたファイルを取得する
		if err != nil {
			t.Fatalf("フォームファイルの取得に失敗しました: %v", err)
		}
		defer fileContent.Close()

		bodyBytes, _ := io.ReadAll(fileContent) // ファイル内容を読み込む
		if string(bodyBytes) != "test archive content" {
			t.Errorf("アップロードされた内容が期待値と異なります: got=%s", string(bodyBytes))
		}

		responseWriter.Write([]byte("https://litter.catbox.moe/testlink.tar.gz")) // 成功時はダウンロードURLをプレーンテキストで返す
	}))
	defer mockServer.Close()

	fileIOClient := newFileIOClientWithEndpoint(mockServer.URL) // モックサーバーへ向けたクライアントを生成する

	downloadURL, err := fileIOClient.Upload(context.Background(), "archive.tar.gz", strings.NewReader("test archive content"), int64(len("test archive content"))) // アップロードを実行する
	if err != nil {
		t.Fatalf("Uploadが失敗しました: %v", err)
	}
	if downloadURL != "https://litter.catbox.moe/testlink.tar.gz" {
		t.Fatalf("ダウンロードURLが期待値と異なります: got=%s", downloadURL)
	}
}

func TestUploadFailureStatus(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) { // 常に500を返すモックサーバーを起動する
		responseWriter.WriteHeader(http.StatusInternalServerError)
		responseWriter.Write([]byte("internal error"))
	}))
	defer mockServer.Close()

	fileIOClient := newFileIOClientWithEndpoint(mockServer.URL)

	_, err := fileIOClient.Upload(context.Background(), "archive.tar.gz", strings.NewReader("data"), 4) // アップロードを実行する
	if err == nil {
		t.Fatalf("エラーが返されるべきですが、nilが返されました")
	}
}

func TestUploadFailureResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) { // URL形式ではないレスポンスを返すモックサーバーを起動する
		responseWriter.Write([]byte("Invalid file type."))
	}))
	defer mockServer.Close()

	fileIOClient := newFileIOClientWithEndpoint(mockServer.URL)

	_, err := fileIOClient.Upload(context.Background(), "archive.tar.gz", strings.NewReader("data"), 4) // アップロードを実行する
	if err == nil {
		t.Fatalf("エラーが返されるべきですが、nilが返されました")
	}
}
