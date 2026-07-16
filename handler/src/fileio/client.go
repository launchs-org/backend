package fileio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const defaultFileIOEndpoint = "https://file.io" // file.io のアップロードエンドポイント

// uploadExpiry はアップロードされたアーカイブの file.io 上での有効期限。
// アップロードトークン(JWT)の有効期限15分に、ビルドJobがダウンロードを完了するまでの猶予を加えた固定値。
const uploadExpiry = "1h"

// FileIOClient は file.io へのアップロードを行うクライアント
type FileIOClient struct {
	endpoint   string       // file.io のエンドポイント URL（テスト時にモックサーバーへ差し替え可能）
	httpClient *http.Client // HTTP クライアント
}

// NewFileIOClient は FileIOClient を生成して返す
func NewFileIOClient() *FileIOClient {
	return &FileIOClient{
		endpoint:   defaultFileIOEndpoint,
		httpClient: &http.Client{Timeout: 60 * time.Second}, // アップロード用のタイムアウトを設定する
	}
}

// newFileIOClientWithEndpoint はテスト用にエンドポイントを差し替えた FileIOClient を生成する
func newFileIOClientWithEndpoint(endpoint string) *FileIOClient {
	return &FileIOClient{
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// NewFileIOClientForTest はテストコードから他パッケージのテストで利用するため、
// エンドポイントを差し替えた FileIOClient を生成する公開版のヘルパー
func NewFileIOClientForTest(endpoint string) *FileIOClient {
	return newFileIOClientWithEndpoint(endpoint)
}

// fileIOUploadResponse は file.io のアップロードレスポンス
type fileIOUploadResponse struct {
	Success bool   `json:"success"` // アップロード成功フラグ
	Link    string `json:"link"`    // ダウンロードリンク
}

// Upload はデータを file.io にアップロードしダウンロードリンクを返す。
// expires=1h, maxDownloads=1 を明示的に指定し、実際に必要な期間だけリンクを生存させる
// （file.io側のデフォルト有効期限に任せない）。
func (client *FileIOClient) Upload(ctx context.Context, fileName string, data io.Reader, size int64) (downloadURL string, err error) {
	bodyReader, bodyWriter := io.Pipe() // multipartボディをストリームで書き込むためのパイプを生成する
	multipartWriter := multipart.NewWriter(bodyWriter)

	go func() { // multipartボディの書き込みを別goroutineで行う
		defer bodyWriter.Close()
		part, createErr := multipartWriter.CreateFormFile("file", fileName) // fileフィールドを作成する
		if createErr != nil {
			bodyWriter.CloseWithError(createErr)
			return
		}
		if _, copyErr := io.Copy(part, data); copyErr != nil { // アーカイブ本体を書き込む
			bodyWriter.CloseWithError(copyErr)
			return
		}
		bodyWriter.CloseWithError(multipartWriter.Close())
	}()

	url := fmt.Sprintf("%s/?expires=%s&maxDownloads=1", client.endpoint, uploadExpiry) // 有効期限とダウンロード回数上限を明示指定する
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader) // リクエストを生成する
	if err != nil {
		return "", fmt.Errorf("file.io アップロードリクエストの生成に失敗しました: %w", err)
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType()) // Content-Type を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return "", fmt.Errorf("file.io アップロードリクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusOK { // 成功以外はエラーとする
		responseBody, _ := io.ReadAll(response.Body) // エラーレスポンスボディを読み込む
		return "", fmt.Errorf("file.io アップロードが失敗しました: status=%d body=%s", response.StatusCode, string(responseBody))
	}

	var uploadResponse fileIOUploadResponse
	if err := json.NewDecoder(response.Body).Decode(&uploadResponse); err != nil { // レスポンスをデコードする
		return "", fmt.Errorf("file.io アップロードレスポンスのデコードに失敗しました: %w", err)
	}
	if !uploadResponse.Success || uploadResponse.Link == "" { // 成功フラグまたはリンクが不正な場合はエラーとする
		return "", fmt.Errorf("file.io アップロードレスポンスが不正です: %+v", uploadResponse)
	}

	return uploadResponse.Link, nil
}
