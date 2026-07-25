package fileio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ArchiveServerClient はクラスタ内の archive-server にアーカイブをアップロードするクライアント。
// 本番環境では handler がインターネットに公開されているため、buildkit からのみアクセス可能な
// archive-server を経由してビルダーにアーカイブを受け渡す。
type ArchiveServerClient struct {
	endpoint     string       // archive-server のアップロードエンドポイント URL
	downloadBase string       // builder に渡すダウンロードURLのベース（archive-server の公開名）
	httpClient   *http.Client // HTTP クライアント
}

// NewArchiveServerClient は ArchiveServerClient を生成する
func NewArchiveServerClient(endpoint string, downloadBase string) *ArchiveServerClient {
	return &ArchiveServerClient{
		endpoint:     endpoint,
		downloadBase: downloadBase,
		httpClient:   &http.Client{Timeout: 60 * time.Second}, // アップロード用のタイムアウトを設定する
	}
}

// Upload はデータを archive-server にアップロードし、ダウンロードURLを返す
func (client *ArchiveServerClient) Upload(ctx context.Context, fileName string, data io.Reader, size int64) (downloadURL string, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, data) // リクエストを生成する
	if err != nil {
		return "", fmt.Errorf("アップロードリクエストの生成に失敗しました: %w", err)
	}
	request.ContentLength = size // Content-Length を明示的に設定する
	request.Header.Set("Content-Type", "application/octet-stream")

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return "", fmt.Errorf("アップロードリクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("アップロードレスポンスの読み込みに失敗しました: %w", err)
	}

	if response.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("アップロードが失敗しました: status=%d body=%s", response.StatusCode, truncateForLog(responseBody))
	}

	id, parseErr := parseUploadID(responseBody)
	if parseErr != nil {
		return "", fmt.Errorf("アップロードレスポンスが不正です: %w", parseErr)
	}

	return strings.TrimSuffix(client.downloadBase, "/") + "/" + id, nil
}

// uploadResponse は archive-server の POST /archives レスポンスボディ
type uploadResponse struct {
	ID string `json:"id"`
}

// parseUploadID はアップロードレスポンスのJSONからIDを抽出する
func parseUploadID(body []byte) (string, error) {
	var parsed uploadResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.ID == "" {
		return "", fmt.Errorf("レスポンスにidが含まれていません: body=%s", truncateForLog(body))
	}
	return parsed.ID, nil
}
