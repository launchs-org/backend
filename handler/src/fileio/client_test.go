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
	mockServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) { // モックfile.ioサーバーを起動する
		if request.URL.Query().Get("expires") != uploadExpiry { // expiresパラメータが指定されていることを確認する
			t.Errorf("expiresパラメータが期待値と異なります: got=%s", request.URL.Query().Get("expires"))
		}
		if request.URL.Query().Get("maxDownloads") != "1" { // maxDownloadsパラメータが指定されていることを確認する
			t.Errorf("maxDownloadsパラメータが期待値と異なります: got=%s", request.URL.Query().Get("maxDownloads"))
		}

		fileContent, _, err := request.FormFile("file") // アップロードされたファイルを取得する
		if err != nil {
			t.Fatalf("フォームファイルの取得に失敗しました: %v", err)
		}
		defer fileContent.Close()

		bodyBytes, _ := io.ReadAll(fileContent) // ファイル内容を読み込む
		if string(bodyBytes) != "test archive content" {
			t.Errorf("アップロードされた内容が期待値と異なります: got=%s", string(bodyBytes))
		}

		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"success":true,"link":"https://file.io/testlink"}`)) // 成功レスポンスを返す
	}))
	defer mockServer.Close()

	fileIOClient := newFileIOClientWithEndpoint(mockServer.URL) // モックサーバーへ向けたクライアントを生成する

	downloadURL, err := fileIOClient.Upload(context.Background(), "archive.tar.gz", strings.NewReader("test archive content"), int64(len("test archive content"))) // アップロードを実行する
	if err != nil {
		t.Fatalf("Uploadが失敗しました: %v", err)
	}
	if downloadURL != "https://file.io/testlink" {
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
	mockServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) { // successがfalseのレスポンスを返すモックサーバーを起動する
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"success":false}`))
	}))
	defer mockServer.Close()

	fileIOClient := newFileIOClientWithEndpoint(mockServer.URL)

	_, err := fileIOClient.Upload(context.Background(), "archive.tar.gz", strings.NewReader("data"), 4) // アップロードを実行する
	if err == nil {
		t.Fatalf("エラーが返されるべきですが、nilが返されました")
	}
}
