package k8s

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	// robotName が base64 エンコード済みの場合はデコードして平文に戻す
	if decoded, err := base64.StdEncoding.DecodeString(robotName); err == nil {
		robotName = string(decoded)
	}
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
		Public:      true,       // 公開設定（false = プライベート）
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
		responseBody, _ := io.ReadAll(response.Body) // エラーレスポンスボディを読み込む
		return fmt.Errorf("harbor project 作成が失敗しました: status=%d body=%s", response.StatusCode, string(responseBody))
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
				Access: []harborRobotAccess{ // プロジェクトの全権限を付与する
					// create
					{Resource: "artifact",            Action: "create"},
					{Resource: "artifact-label",      Action: "create"},
					{Resource: "export-cve",          Action: "create"},
					{Resource: "immutable-tag",       Action: "create"},
					{Resource: "label",               Action: "create"},
					{Resource: "member",              Action: "create"},
					{Resource: "metadata",            Action: "create"},
					{Resource: "notification-policy", Action: "create"},
					{Resource: "preheat-policy",      Action: "create"},
					{Resource: "robot",               Action: "create"},
					{Resource: "sbom",                Action: "create"},
					{Resource: "scan",                Action: "create"},
					{Resource: "scanner",             Action: "create"},
					{Resource: "tag",                 Action: "create"},
					{Resource: "tag-retention",       Action: "create"},
					// delete
					{Resource: "artifact",            Action: "delete"},
					{Resource: "artifact-label",      Action: "delete"},
					{Resource: "immutable-tag",       Action: "delete"},
					{Resource: "label",               Action: "delete"},
					{Resource: "member",              Action: "delete"},
					{Resource: "metadata",            Action: "delete"},
					{Resource: "notification-policy", Action: "delete"},
					{Resource: "preheat-policy",      Action: "delete"},
					{Resource: "project",             Action: "delete"},
					{Resource: "repository",          Action: "delete"},
					{Resource: "robot",               Action: "delete"},
					{Resource: "tag",                 Action: "delete"},
					{Resource: "tag-retention",       Action: "delete"},
					// list
					{Resource: "accessory",           Action: "list"},
					{Resource: "artifact",            Action: "list"},
					{Resource: "immutable-tag",       Action: "list"},
					{Resource: "label",               Action: "list"},
					{Resource: "log",                 Action: "list"},
					{Resource: "member",              Action: "list"},
					{Resource: "metadata",            Action: "list"},
					{Resource: "notification-policy", Action: "list"},
					{Resource: "preheat-policy",      Action: "list"},
					{Resource: "repository",          Action: "list"},
					{Resource: "robot",               Action: "list"},
					{Resource: "tag",                 Action: "list"},
					{Resource: "tag-retention",       Action: "list"},
					// pull / push
					{Resource: "repository",          Action: "pull"},
					{Resource: "repository",          Action: "push"},
					// read
					{Resource: "artifact",            Action: "read"},
					{Resource: "artifact-addition",   Action: "read"},
					{Resource: "export-cve",          Action: "read"},
					{Resource: "label",               Action: "read"},
					{Resource: "member",              Action: "read"},
					{Resource: "metadata",            Action: "read"},
					{Resource: "notification-policy", Action: "read"},
					{Resource: "preheat-policy",      Action: "read"},
					{Resource: "project",             Action: "read"},
					{Resource: "quota",               Action: "read"},
					{Resource: "repository",          Action: "read"},
					{Resource: "robot",               Action: "read"},
					{Resource: "sbom",                Action: "read"},
					{Resource: "scan",                Action: "read"},
					{Resource: "scanner",             Action: "read"},
					{Resource: "tag-retention",       Action: "read"},
					// stop
					{Resource: "sbom",                Action: "stop"},
					{Resource: "scan",                Action: "stop"},
					// update
					{Resource: "immutable-tag",       Action: "update"},
					{Resource: "label",               Action: "update"},
					{Resource: "member",              Action: "update"},
					{Resource: "metadata",            Action: "update"},
					{Resource: "notification-policy", Action: "update"},
					{Resource: "preheat-policy",      Action: "update"},
					{Resource: "project",             Action: "update"},
					{Resource: "repository",          Action: "update"},
					{Resource: "tag-retention",       Action: "update"},
				},
			},
		},
	}
	bodyBytes, err := json.Marshal(requestBody) // リクエストボディを JSON にシリアライズする
	if err != nil {
		return nil, fmt.Errorf("harbor robot account リクエストのシリアライズに失敗しました: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2.0/robots", client.endpoint) // Harbor API の URL を組み立てる（v2.14 では /robots エンドポイントを使う）
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
		responseBody, _ := io.ReadAll(response.Body) // エラーレスポンスボディを読み込む
		return nil, fmt.Errorf("harbor robot account 作成が失敗しました: status=%d body=%s", response.StatusCode, string(responseBody))
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

// harborRepository は Harbor リポジトリ一覧取得レスポンスの要素
type harborRepository struct {
	Name string `json:"name"` // リポジトリ名（"projectName/imageName" 形式）
}

// listHarborRepositories は Harbor project 内のリポジトリ一覧を取得する
func (client *HarborClient) listHarborRepositories(ctx context.Context, projectName string, credential HarborRobotCredential) ([]harborRepository, error) {
	url := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories?page_size=100", client.endpoint, projectName) // リポジトリ一覧 API の URL を組み立てる
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)                              // GET リクエストを生成する
	if err != nil {
		return nil, fmt.Errorf("harbor リポジトリ一覧リクエストの生成に失敗しました: %w", err)
	}
	request.SetBasicAuth(credential.Name, credential.Secret) // robot 認証情報で Basic 認証を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return nil, fmt.Errorf("harbor リポジトリ一覧リクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusOK { // 200 以外はエラーとする
		responseBody, _ := io.ReadAll(response.Body) // エラーボディを読み込む
		return nil, fmt.Errorf("harbor リポジトリ一覧の取得に失敗しました: status=%d body=%s", response.StatusCode, string(responseBody))
	}

	var repositoryList []harborRepository                      // レスポンスをパースする
	if err := json.NewDecoder(response.Body).Decode(&repositoryList); err != nil { // JSON デコードする
		return nil, fmt.Errorf("harbor リポジトリ一覧のデコードに失敗しました: %w", err)
	}
	return repositoryList, nil
}

// deleteHarborRepository は Harbor project 内の 1 つのリポジトリを削除する
func (client *HarborClient) deleteHarborRepository(ctx context.Context, projectName string, repositoryName string, credential HarborRobotCredential) error {
	url := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s", client.endpoint, projectName, repositoryName) // リポジトリ削除 API の URL を組み立てる
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)                                 // DELETE リクエストを生成する
	if err != nil {
		return fmt.Errorf("harbor リポジトリ削除リクエストの生成に失敗しました: %w", err)
	}
	request.SetBasicAuth(credential.Name, credential.Secret) // robot 認証情報で Basic 認証を設定する

	response, err := client.httpClient.Do(request) // リクエストを送信する
	if err != nil {
		return fmt.Errorf("harbor リポジトリ削除リクエストの送信に失敗しました: %w", err)
	}
	defer response.Body.Close() // レスポンスボディを閉じる

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent { // 削除成功以外はエラーとする
		responseBody, _ := io.ReadAll(response.Body) // エラーボディを読み込む
		return fmt.Errorf("harbor リポジトリ削除が失敗しました: status=%d body=%s", response.StatusCode, string(responseBody))
	}
	return nil
}

// DeleteHarborProject は Harbor から project を削除する（robot account は自動無効化される）
// project ごとに作成した robot account の認証情報を使って削除する
// Harbor は project 内にリポジトリが残っていると 412 を返すため、先にリポジトリを全削除する
func (client *HarborClient) DeleteHarborProject(ctx context.Context, projectName string, credential HarborRobotCredential) error {
	// プロジェクト内のリポジトリを全取得して先に削除する（残っていると 412 になるため）
	repositoryList, err := client.listHarborRepositories(ctx, projectName, credential) // リポジトリ一覧を取得する
	if err != nil {
		return fmt.Errorf("harbor リポジトリ一覧の取得に失敗しました: %w", err)
	}

	for _, repositoryData := range repositoryList { // 各リポジトリを削除する
		// Harbor のリポジトリ名は "projectName/imageName" 形式で返るため imageName 部分のみを取り出す
		repoShortName := repositoryData.Name                                                                 // フルネームを保持する
		if len(projectName)+1 < len(repositoryData.Name) {                                                   // "projectName/" プレフィックスを除去する
			repoShortName = repositoryData.Name[len(projectName)+1:] // スラッシュの後ろだけ取り出す
		}
		if err := client.deleteHarborRepository(ctx, projectName, repoShortName, credential); err != nil { // リポジトリを削除する
			return fmt.Errorf("harbor リポジトリ '%s' の削除に失敗しました: %w", repoShortName, err)
		}
	}

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
		responseBody, _ := io.ReadAll(response.Body) // エラーボディを読み込む
		return fmt.Errorf("harbor project 削除が失敗しました: status=%d body=%s", response.StatusCode, string(responseBody))
	}
	return nil
}
