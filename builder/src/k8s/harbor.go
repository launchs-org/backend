package k8s

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

// HarborClient は Harbor API を操作するクライアント（builder 用：サイズ取得のみ）
type HarborClient struct {
	endpoint   string       // Harbor の API エンドポイント URL
	httpClient *http.Client // HTTP クライアント
}

// NewHarborClient は HarborClient を生成して返す
func NewHarborClient(endpoint string) *HarborClient {
	return &HarborClient{
		endpoint: endpoint, // エンドポイントを設定する
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 自己署名証明書を許容する
			},
		},
	}
}

// harborArtifact は Harbor アーティファクト取得レスポンス
type harborArtifact struct {
	Size int64 `json:"size"` // アーティファクトのバイトサイズ
}

// GetArtifactSize は Harbor 上の特定タグのアーティファクト1件のサイズをバイト単位で返す
func (client *HarborClient) GetArtifactSize(ctx context.Context, projectName, repositoryName, tag, robotName, robotSecret string) (int64, error) {
	artifactURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		client.endpoint, projectName, repositoryName, tag) // タグ指定のアーティファクト取得URLを組み立てる

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil) // GET リクエストを生成する
	if err != nil {
		return 0, fmt.Errorf("harbor アーティファクト取得リクエストの生成に失敗しました: %w", err) // 生成エラーを返す
	}
	request.SetBasicAuth(robotName, robotSecret) // robot 認証情報で Basic 認証を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return 0, fmt.Errorf("harbor アーティファクト取得リクエストの送信に失敗しました: %w", err) // 送信エラーを返す
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusOK { // 200 以外はサイズ取得不可として 0 を返す
		return 0, nil
	}

	var artifactData harborArtifact                                             // レスポンスをパースする
	if err := json.NewDecoder(response.Body).Decode(&artifactData); err != nil { // JSON デコードする
		return 0, nil // デコード失敗は非致命的なため 0 を返す
	}

	return artifactData.Size, nil // アーティファクトのサイズを返す
}
