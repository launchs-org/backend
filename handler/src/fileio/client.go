package fileio

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// defaultFileIOEndpoint は一時ファイル共有サービス litterbox のアップロードエンドポイント。
// （旧 file.io は匿名アップロードAPIが廃止され Cloudflare 経由でトップページへリダイレクトされるようになったため litterbox に切り替えた）
const defaultFileIOEndpoint = "https://litterbox.catbox.moe/resources/internals/api.php"

// uploadExpiry はアップロードされたアーカイブの litterbox 上での有効期限。
// アップロードトークン(JWT)の有効期限15分に、ビルドJobがダウンロードを完了するまでの猶予を加えた固定値。
// litterbox が受け付ける値は 1h/12h/24h/72h のみ。
const uploadExpiry = "1h"

// FileIOClient は一時ファイル共有サービスへのアップロードを行うクライアント
type FileIOClient struct {
	endpoint   string       // アップロードエンドポイント URL（テスト時にモックサーバーへ差し替え可能）
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

// Upload はデータを一時ファイル共有サービスにアップロードしダウンロードリンクを返す。
// litterbox のレスポンスは JSON ではなくダウンロードURLそのものがプレーンテキストで返る。
func (client *FileIOClient) Upload(ctx context.Context, fileName string, data io.Reader, size int64) (downloadURL string, err error) {
	bodyReader, bodyWriter := io.Pipe() // multipartボディをストリームで書き込むためのパイプを生成する
	multipartWriter := multipart.NewWriter(bodyWriter)

	go func() { // multipartボディの書き込みを別goroutineで行う
		defer bodyWriter.Close()
		if writeErr := multipartWriter.WriteField("reqtype", "fileupload"); writeErr != nil { // litterbox 固有の必須フィールドを設定する
			bodyWriter.CloseWithError(writeErr)
			return
		}
		if writeErr := multipartWriter.WriteField("time", uploadExpiry); writeErr != nil { // 有効期限を設定する
			bodyWriter.CloseWithError(writeErr)
			return
		}
		part, createErr := multipartWriter.CreateFormFile("fileToUpload", fileName) // litterbox のファイルフィールド名は fileToUpload
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

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bodyReader) // リクエストを生成する
	if err != nil {
		return "", fmt.Errorf("アップロードリクエストの生成に失敗しました: %w", err)
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType()) // Content-Type を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return "", fmt.Errorf("アップロードリクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	responseBody, err := io.ReadAll(response.Body) // レスポンスボディを読み込む
	if err != nil {
		return "", fmt.Errorf("アップロードレスポンスの読み込みに失敗しました: %w", err)
	}

	if response.StatusCode != http.StatusOK { // 成功以外はエラーとする
		return "", fmt.Errorf("アップロードが失敗しました: status=%d body=%s", response.StatusCode, truncateForLog(responseBody))
	}

	link := strings.TrimSpace(string(responseBody)) // レスポンスはダウンロードURLそのもののプレーンテキスト
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") { // URL形式でない場合は不正なレスポンスとする
		return "", fmt.Errorf("アップロードレスポンスが不正です: body=%s", truncateForLog(responseBody))
	}

	return link, nil
}

// truncateForLog はログ出力用にレスポンスボディを短縮する（HTMLエラーページ等でログが肥大化するのを防ぐ）
func truncateForLog(body []byte) string {
	const maxLen = 300
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "...(truncated)"
}
