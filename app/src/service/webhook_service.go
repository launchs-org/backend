package service

import (
	"app/models"
	"app/repository"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// WebhookService は Webhook CRUD のビジネスロジックを定義するインターフェース
type WebhookService interface {
	CreateWebhook(ctx context.Context, userID string, deploymentID string, req CreateWebhookRequest) (*models.DeploymentWebhook, error) // webhook を作成する
	GetWebhook(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error)                              // webhook を取得する
	DeleteWebhook(ctx context.Context, userID string, webhookID string) error                                                           // webhook を削除する
	ReceiveGithubWebhook(ctx context.Context, deploymentID string, signature string, body []byte) error                                 // GitHub push イベントを受信して Apply をトリガーする
}

// ErrInvalidSignature は HMAC 署名が不正な場合のエラー
var ErrInvalidSignature = errors.New("invalid signature") // 署名不正エラーを定義する

// CreateWebhookRequest は POST /deployments/:id/webhooks のリクエスト構造体
type CreateWebhookRequest struct {
	GithubRepoURL string `json:"github_repo_url"` // GitHub リポジトリ URL
}

// webhookServiceImpl は WebhookService の実装
type webhookServiceImpl struct {
	webhookRepo    repository.WebhookRepository    // webhook リポジトリ
	deploymentRepo repository.DeploymentRepository // deployment リポジトリ（認可チェックに使用する）
	projectRepo    repository.ProjectRepository    // project リポジトリ（認可チェックに使用する）
	applyService   ApplyServiceInterface           // apply サービス（Webhook 経由の Apply 実行に使用する）
}

// NewWebhookService は WebhookService の実装を返す
func NewWebhookService(
	webhookRepo repository.WebhookRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	applyService ApplyServiceInterface,
) WebhookService {
	return &webhookServiceImpl{
		webhookRepo:    webhookRepo,    // webhook リポジトリを注入する
		deploymentRepo: deploymentRepo, // deployment リポジトリを注入する
		projectRepo:    projectRepo,    // project リポジトリを注入する
		applyService:   applyService,   // apply サービスを注入する
	}
}

// checkDeploymentOwner は deploymentID に対応する deployment の ProjectID を取得し、Project の UserID と userID を比較して認可チェックを行う
func (svc *webhookServiceImpl) checkDeploymentOwner(ctx context.Context, userID string, deploymentID string) error {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // ユーザー ID が一致しない場合は forbidden を返す
		return ErrForbidden // アクセス拒否エラーを返す
	}
	return nil // 認可チェック成功を返す
}

// generateSecret は 32 バイトのランダムな hex 文字列を生成する
func generateSecret() (string, error) {
	secretBytes := make([]byte, 32)                    // 32 バイトのバッファを確保する
	if _, err := rand.Read(secretBytes); err != nil { // ランダムバイト列を生成する
		return "", err // 生成エラーを返す
	}
	return hex.EncodeToString(secretBytes), nil // hex 文字列に変換して返す
}

// CreateWebhook は webhook を作成する
func (svc *webhookServiceImpl) CreateWebhook(ctx context.Context, userID string, deploymentID string, req CreateWebhookRequest) (*models.DeploymentWebhook, error) {
	if err := svc.checkDeploymentOwner(ctx, userID, deploymentID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}

	secret, err := generateSecret() // シークレットを自動生成する
	if err != nil {
		return nil, err // 生成エラーを返す
	}

	webhookData := &models.DeploymentWebhook{
		DeploymentID:  deploymentID,      // deployment ID を設定する
		Secret:        secret,            // 自動生成したシークレットを設定する
		GithubRepoURL: req.GithubRepoURL, // GitHub リポジトリ URL を設定する
		IsActive:      true,              // デフォルトで有効に設定する
	}
	if err := svc.webhookRepo.Create(ctx, webhookData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return webhookData, nil // 作成した webhook を返す
}

// GetWebhook は deploymentID に紐づく webhook を返す
func (svc *webhookServiceImpl) GetWebhook(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error) {
	if err := svc.checkDeploymentOwner(ctx, userID, deploymentID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}
	return svc.webhookRepo.FindByDeploymentID(ctx, deploymentID) // リポジトリ経由で取得する
}

// DeleteWebhook は webhookID に対応する webhook を削除する
func (svc *webhookServiceImpl) DeleteWebhook(ctx context.Context, userID string, webhookID string) error {
	webhookData, err := svc.webhookRepo.FindByID(ctx, webhookID) // webhook を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkDeploymentOwner(ctx, userID, webhookData.DeploymentID); err != nil { // 認可チェックを行う
		return err // エラーを返す
	}
	return svc.webhookRepo.Delete(ctx, webhookID) // リポジトリ経由で削除する
}

// GithubPushPayload は GitHub push イベントのペイロード構造体
type GithubPushPayload struct {
	Ref        string `json:"ref"`         // refs/heads/{branch} 形式のブランチ参照
	After      string `json:"after"`       // push 後の commit SHA
	Repository struct {
		FullName string `json:"full_name"` // リポジトリのフルネーム（org/repo 形式）
	} `json:"repository"`
}

// ReceiveGithubWebhook は GitHub push イベントを受信して HMAC 署名を検証し、ブランチが一致する場合に Apply をトリガーする
func (svc *webhookServiceImpl) ReceiveGithubWebhook(ctx context.Context, deploymentID string, signature string, body []byte) error {
	webhookData, err := svc.webhookRepo.FindByDeploymentID(ctx, deploymentID) // deployment に紐づく webhook を取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	// HMAC-SHA256 署名を検証する
	mac := hmac.New(sha256.New, []byte(webhookData.Secret)) // HMAC を生成する
	mac.Write(body)                                          // リクエストボディを HMAC に書き込む
	expectedSig := fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil))) // 期待する署名を生成する
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) { // 署名が一致しない場合は不正エラーを返す
		return ErrInvalidSignature // 署名不正エラーを返す
	}

	// push ペイロードをパースする
	var pushPayload GithubPushPayload                          // ペイロードを格納する変数を定義する
	if err := json.Unmarshal(body, &pushPayload); err != nil { // JSON をパースする
		return fmt.Errorf("ペイロードのパースに失敗しました: %w", err) // パースエラーを返す
	}

	// deployment の github_branch と push の ref を比較する
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	expectedRef := fmt.Sprintf("refs/heads/%s", deploymentData.GithubBranch) // 期待する ref を生成する
	if pushPayload.Ref != expectedRef {                                        // ブランチが一致しない場合はスキップする
		return nil // apply をトリガーしない
	}

	// 所有者の userID を取得する
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("project の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	// pending_github_commit_sha を push の commit SHA に更新する
	if err := svc.deploymentRepo.UpdatePendingGithubCommitSHA(ctx, deploymentID, pushPayload.After); err != nil { // pending commit SHA を更新する
		return fmt.Errorf("commit SHA の更新に失敗しました: %w", err) // 更新エラーを返す
	}

	// Apply をトリガーする
	if _, err := svc.applyService.Apply(ctx, projectData.UserID, deploymentID); err != nil { // Apply をトリガーする
		return fmt.Errorf("Apply の実行に失敗しました: %w", err) // Apply エラーを返す
	}
	return nil // 正常終了を返す
}
