package fileio

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
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

// maxUploadSizeBytes はアップロード可能なアーカイブの上限サイズ（500MB）
const maxUploadSizeBytes = 500 * 1024 * 1024

// Upload はデータを一時ファイル共有サービスにアップロードしダウンロードリンクを返す。
// litterbox のレスポンスは JSON ではなくダウンロードURLそのものがプレーンテキストで返る。
// multipart ボディは一時ファイルに書き出してから Content-Length 付きで送信する
// （io.Pipe によるストリーミング送信は litterbox 側で正しく処理されず 412 No file! になることがあったため）
func (client *FileIOClient) Upload(ctx context.Context, fileName string, data io.Reader, size int64) (downloadURL string, err error) {
	if size > maxUploadSizeBytes {
		return "", fmt.Errorf("アーカイブサイズが上限を超えています: size=%d, limit=%d", size, maxUploadSizeBytes)
	}

	tempFile, err := os.CreateTemp("", "litterbox-upload-*.tmp") // multipartボディ全体を書き出す一時ファイルを作成する
	if err != nil {
		return "", fmt.Errorf("一時ファイルの作成に失敗しました: %w", err)
	}
	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath) // 終了時に一時ファイルを削除する
	defer tempFile.Close()

	multipartWriter := multipart.NewWriter(tempFile)
	if writeErr := multipartWriter.WriteField("reqtype", "fileupload"); writeErr != nil { // litterbox 固有の必須フィールドを設定する
		return "", fmt.Errorf("multipartフィールドの書き込みに失敗しました: %w", writeErr)
	}
	if writeErr := multipartWriter.WriteField("time", uploadExpiry); writeErr != nil { // 有効期限を設定する
		return "", fmt.Errorf("multipartフィールドの書き込みに失敗しました: %w", writeErr)
	}
	part, createErr := multipartWriter.CreateFormFile("fileToUpload", fileName) // litterbox のファイルフィールド名は fileToUpload
	if createErr != nil {
		return "", fmt.Errorf("multipartファイルパートの生成に失敗しました: %w", createErr)
	}
	if _, copyErr := io.Copy(part, data); copyErr != nil { // アーカイブ本体を書き込む
		return "", fmt.Errorf("アーカイブ本体の書き込みに失敗しました: %w", copyErr)
	}
	if closeErr := multipartWriter.Close(); closeErr != nil {
		return "", fmt.Errorf("multipartボディのクローズに失敗しました: %w", closeErr)
	}

	fileInfo, statErr := tempFile.Stat() // Content-Length 用に一時ファイルのサイズを取得する
	if statErr != nil {
		return "", fmt.Errorf("一時ファイルの情報取得に失敗しました: %w", statErr)
	}
	if _, seekErr := tempFile.Seek(0, io.SeekStart); seekErr != nil { // 読み込み用に先頭へシークし直す
		return "", fmt.Errorf("一時ファイルのシークに失敗しました: %w", seekErr)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, tempFile) // リクエストを生成する
	if err != nil {
		return "", fmt.Errorf("アップロードリクエストの生成に失敗しました: %w", err)
	}
	request.ContentLength = fileInfo.Size()                                     // Content-Length を明示的に設定する
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
