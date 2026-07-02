package service

import (
	"handler/models"
	"handler/repository"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// WebhookService は Webhook CRUD のビジネスロジックを定義するインターフェース
type WebhookService interface {
	CreateWebhook(ctx context.Context, userID string, deploymentID string, req CreateWebhookRequest) (*models.DeploymentWebhook, error)                                         // webhook を作成する
	GetWebhook(ctx context.Context, userID string, deploymentID string) (*models.DeploymentWebhook, error)                                                                     // webhook を取得する
	DeleteWebhook(ctx context.Context, userID string, webhookID string) error                                                                                                  // webhook を削除する
	TriggerBuildByWebhook(ctx context.Context, deploymentID string, secret string, commitMessage string, author string) (*models.DeploymentBuild, error)                      // シークレット認証でビルドをトリガーする
	GetBuildByWebhook(ctx context.Context, deploymentID string, secret string, buildID string) (*models.DeploymentBuild, error)                                               // シークレット認証でビルド状態を確認する
	ApplyByWebhook(ctx context.Context, deploymentID string, secret string) (*ApplyResult, error)                                                                             // シークレット認証で Apply を実行する
	UpdateImageAndApplyByWebhook(ctx context.Context, deploymentID string, secret string, imageURL string) (*ApplyResult, error)                                              // シークレット認証で image_url を更新して Apply を実行する
}

// ErrInvalidSignature はシークレットが不正な場合のエラー
var ErrInvalidSignature = errors.New("invalid signature") // 署名不正エラーを定義する

// ErrWebhookInactive は Webhook が無効な場合のエラー
var ErrWebhookInactive = errors.New("webhook is inactive") // Webhook 無効エラーを定義する

// CreateWebhookRequest は POST /deployments/:id/webhooks のリクエスト構造体
type CreateWebhookRequest struct {
	GithubRepoURL string `json:"github_repo_url"` // GitHub リポジトリ URL
}

// webhookServiceImpl は WebhookService の実装
type webhookServiceImpl struct {
	webhookRepo    repository.WebhookRepository    // webhook リポジトリ
	deploymentRepo repository.DeploymentRepository // deployment リポジトリ（認可チェックに使用する）
	projectRepo    repository.ProjectRepository    // project リポジトリ（認可チェックに使用する）
	imageRepo      repository.ImageRepository       // image リポジトリ（image_url 更新時の Image レコード作成用）
	applyService   ApplyServiceInterface           // apply サービス（Webhook 経由の Apply 実行に使用する）
	buildService   BuildService                    // build サービス（Webhook 経由のビルドトリガーに使用する）
}

// NewWebhookService は WebhookService の実装を返す
func NewWebhookService(
	webhookRepo repository.WebhookRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	imageRepo repository.ImageRepository,
	applyService ApplyServiceInterface,
	buildService BuildService,
) WebhookService {
	return &webhookServiceImpl{
		webhookRepo:    webhookRepo,    // webhook リポジトリを注入する
		deploymentRepo: deploymentRepo, // deployment リポジトリを注入する
		projectRepo:    projectRepo,    // project リポジトリを注入する
		imageRepo:      imageRepo,      // image リポジトリを注入する
		applyService:   applyService,   // apply サービスを注入する
		buildService:   buildService,   // build サービスを注入する
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

// getDeploymentOwner は deploymentID に対応する Project を返す
func (svc *webhookServiceImpl) getDeploymentOwner(ctx context.Context, deploymentID string) (*models.Project, error) {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return projectData, nil // project を返す
}

// verifyWebhookSecret は deploymentID に紐づく Webhook のシークレットを検証する
func (svc *webhookServiceImpl) verifyWebhookSecret(ctx context.Context, deploymentID string, secret string) error {
	webhookData, err := svc.webhookRepo.FindByDeploymentID(ctx, deploymentID) // webhook を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if !webhookData.IsActive { // Webhook が無効な場合はエラーを返す
		return ErrWebhookInactive
	}
	if !hmac.Equal([]byte(secret), []byte(webhookData.Secret)) { // シークレットが一致しない場合はエラーを返す
		return ErrInvalidSignature
	}
	return nil // 検証成功を返す
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

// TriggerBuildByWebhook はシークレット認証でビルドをトリガーする
func (svc *webhookServiceImpl) TriggerBuildByWebhook(ctx context.Context, deploymentID string, secret string, commitMessage string, author string) (*models.DeploymentBuild, error) {
	if err := svc.verifyWebhookSecret(ctx, deploymentID, secret); err != nil { // シークレットを検証する
		return nil, err // 検証エラーを返す
	}
	projectData, err := svc.getDeploymentOwner(ctx, deploymentID) // deployment の所有者 project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return svc.buildService.TriggerBuild(ctx, projectData.UserID, deploymentID, commitMessage, author) // ビルドをトリガーする
}

// GetBuildByWebhook はシークレット認証でビルド状態を確認する
func (svc *webhookServiceImpl) GetBuildByWebhook(ctx context.Context, deploymentID string, secret string, buildID string) (*models.DeploymentBuild, error) {
	if err := svc.verifyWebhookSecret(ctx, deploymentID, secret); err != nil { // シークレットを検証する
		return nil, err // 検証エラーを返す
	}
	projectData, err := svc.getDeploymentOwner(ctx, deploymentID) // deployment の所有者 project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return svc.buildService.GetBuild(ctx, projectData.UserID, buildID) // ビルド情報を取得する
}

// ApplyByWebhook はシークレット認証で Apply を実行する
func (svc *webhookServiceImpl) ApplyByWebhook(ctx context.Context, deploymentID string, secret string) (*ApplyResult, error) {
	if err := svc.verifyWebhookSecret(ctx, deploymentID, secret); err != nil { // シークレットを検証する
		return nil, err // 検証エラーを返す
	}
	projectData, err := svc.getDeploymentOwner(ctx, deploymentID) // deployment の所有者 project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return svc.applyService.Apply(ctx, projectData.UserID, deploymentID) // Apply を実行する
}

// UpdateImageAndApplyByWebhook はシークレット認証で image_url を pending に設定して Apply を実行する
func (svc *webhookServiceImpl) UpdateImageAndApplyByWebhook(ctx context.Context, deploymentID string, secret string, imageURL string) (*ApplyResult, error) {
	if err := svc.verifyWebhookSecret(ctx, deploymentID, secret); err != nil { // シークレットを検証する
		return nil, err // 検証エラーを返す
	}
	projectData, err := svc.getDeploymentOwner(ctx, deploymentID) // deployment の所有者 project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	imageData := &models.Image{ // イメージレコードを構築する（Webhook 経由の外部URL直接指定のため BuildID は nil）
		ProjectID: projectData.ID, // プロジェクト ID を設定する
		BuildID:   nil,            // ビルドを経由しない直接指定のため nil
		ImageURL:  imageURL,       // 指定された URL を設定する
	}
	if err := svc.imageRepo.Create(ctx, imageData); err != nil { // Image レコードを作成する
		return nil, err // 作成エラーを返す
	}
	if err := svc.deploymentRepo.UpdatePendingImageID(ctx, deploymentID, imageData.ID); err != nil { // pending_image_id を更新する
		return nil, err // 更新エラーを返す
	}
	return svc.applyService.Apply(ctx, projectData.UserID, deploymentID) // Apply を実行する
}
