package k8s

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

// HarborClient は Harbor API を操作するクライアント
type HarborClient struct {
	endpoint    string       // Harbor のエンドポイント URL
	robotName   string       // 管理用 robot アカウント名（base64 エンコード済み）
	robotSecret string       // 管理用 robot アカウントのシークレット
	httpClient  *http.Client // HTTP クライアント
}

// Endpoint は Harbor のエンドポイント URL を返す
func (client *HarborClient) Endpoint() string {
	return client.endpoint // エンドポイントを返す
}

// AdminCredential は管理用 robot の認証情報を返す
func (client *HarborClient) AdminCredential() HarborRobotCredential {
	return HarborRobotCredential{
		Name:   client.robotName,   // 管理用 robot 名を返す
		Secret: client.robotSecret, // 管理用 robot のシークレットを返す
	}
}

// NewHarborClient は HarborClient を生成して返す
func NewHarborClient(endpoint, robotName, robotSecret string) *HarborClient {
	return &HarborClient{
		endpoint:    endpoint,             // エンドポイントを設定する
		robotName:   robotName,            // robot アカウント名を設定する
		robotSecret: robotSecret,          // シークレットを設定する
		httpClient: &http.Client{ // TLS 証明書検証をスキップする（自己署名証明書対応）
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // IP SAN なし証明書を許容する
			},
		},
	}
}

// HarborRobotCredential は作成した robot account の認証情報
type HarborRobotCredential struct {
	Name   string // base64 エンコード済み robot アカウント名
	Secret string // robot アカウントのシークレット
}

// harborProjectRequest は Harbor project 作成リクエストのボディ
type harborProjectRequest struct {
	ProjectName string `json:"project_name"` // プロジェクト名
	Public      bool   `json:"public"`       // 公開設定（false = プライベート）
}

// harborRobotRequest は Harbor robot account 作成リクエストのボディ
type harborRobotRequest struct {
	Name        string              `json:"name"`        // robot アカウント名
	Description string              `json:"description"` // 説明
	Duration    int                 `json:"duration"`    // 有効期限（-1 = 無期限）
	Level       string              `json:"level"`       // スコープレベル
	Permissions []harborRobotPermission `json:"permissions"` // 権限リスト
}

// harborRobotPermission は Harbor robot account の権限
type harborRobotPermission struct {
	Kind      string               `json:"kind"`      // リソース種別
	Namespace string               `json:"namespace"` // 対象 namespace（プロジェクト名）
	Access    []harborRobotAccess  `json:"access"`    // アクセス権限リスト
}

// harborRobotAccess は Harbor robot account の個別アクセス権限
type harborRobotAccess struct {
	Resource string `json:"resource"` // リソース
	Action   string `json:"action"`   // アクション
}

// harborRobotResponse は Harbor robot account 作成レスポンス
type harborRobotResponse struct {
	Name   string `json:"name"`   // robot アカウント名（base64 エンコード済み）
	Secret string `json:"secret"` // シークレット
}

// CreateHarborProject は Harbor に project を作成する
func (client *HarborClient) CreateHarborProject(ctx context.Context, projectName string) error {
	requestBody := harborProjectRequest{
		ProjectName: projectName, // プロジェクト名を設定する
		Public:      false,       // プライベートプロジェクトとして作成する
	}
	bodyBytes, err := json.Marshal(requestBody) // リクエストボディを JSON にシリアライズする
	if err != nil {
		return fmt.Errorf("harbor project リクエストのシリアライズに失敗しました: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2.0/projects", client.endpoint) // Harbor API の URL を組み立てる
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes)) // リクエストを生成する
	if err != nil {
		return fmt.Errorf("harbor project 作成リクエストの生成に失敗しました: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")          // Content-Type を設定する
	request.SetBasicAuth(client.robotName, client.robotSecret)       // Basic 認証を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return fmt.Errorf("harbor project 作成リクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusCreated { // 作成成功以外はエラーとする
		return fmt.Errorf("harbor project 作成が失敗しました: status=%d", response.StatusCode)
	}
	return nil
}

// CreateHarborRobotAccount は Harbor project に robot account を作成して認証情報を返す
func (client *HarborClient) CreateHarborRobotAccount(ctx context.Context, projectName string) (*HarborRobotCredential, error) {
	requestBody := harborRobotRequest{
		Name:        fmt.Sprintf("robot-%s", projectName), // robot アカウント名を組み立てる
		Description: fmt.Sprintf("%s project robot account", projectName), // 説明を設定する
		Duration:    -1,       // 無期限に設定する
		Level:       "project", // プロジェクトレベルのスコープに設定する
		Permissions: []harborRobotPermission{
			{
				Kind:      "project",     // プロジェクトリソース種別を設定する
				Namespace: projectName,   // 対象プロジェクトを設定する
				Access: []harborRobotAccess{
					{Resource: "repository", Action: "push"}, // push 権限を付与する
					{Resource: "repository", Action: "pull"}, // pull 権限を付与する
				},
			},
		},
	}
	bodyBytes, err := json.Marshal(requestBody) // リクエストボディを JSON にシリアライズする
	if err != nil {
		return nil, fmt.Errorf("harbor robot account リクエストのシリアライズに失敗しました: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2.0/projects/%s/robots", client.endpoint, projectName) // Harbor API の URL を組み立てる
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes)) // リクエストを生成する
	if err != nil {
		return nil, fmt.Errorf("harbor robot account 作成リクエストの生成に失敗しました: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")    // Content-Type を設定する
	request.SetBasicAuth(client.robotName, client.robotSecret) // Basic 認証を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return nil, fmt.Errorf("harbor robot account 作成リクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusCreated { // 作成成功以外はエラーとする
		return nil, fmt.Errorf("harbor robot account 作成が失敗しました: status=%d", response.StatusCode)
	}

	var robotResponse harborRobotResponse
	if err := json.NewDecoder(response.Body).Decode(&robotResponse); err != nil { // レスポンスをデコードする
		return nil, fmt.Errorf("harbor robot account レスポンスのデコードに失敗しました: %w", err)
	}

	return &HarborRobotCredential{
		Name:   robotResponse.Name,   // robot アカウント名を返す
		Secret: robotResponse.Secret, // シークレットを返す
	}, nil
}

// DeleteHarborProject は Harbor から project を削除する（robot account は自動無効化される）
// project ごとに作成した robot account の認証情報を使って削除する
func (client *HarborClient) DeleteHarborProject(ctx context.Context, projectName string, credential HarborRobotCredential) error {
	url := fmt.Sprintf("%s/api/v2.0/projects/%s", client.endpoint, projectName)  // Harbor API の URL を組み立てる
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil) // DELETE リクエストを生成する
	if err != nil {
		return fmt.Errorf("harbor project 削除リクエストの生成に失敗しました: %w", err)
	}
	request.SetBasicAuth(credential.Name, credential.Secret) // project 専用 robot の認証情報で Basic 認証を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return fmt.Errorf("harbor project 削除リクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent { // 削除成功以外はエラーとする
		return fmt.Errorf("harbor project 削除が失敗しました: status=%d", response.StatusCode)
	}
	return nil
}
