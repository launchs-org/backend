package service

import (
	"app/models"
	"app/repository"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrDuplicatePathPrefix は同一 IngressRoute 内で同じ path_prefix が既に存在する場合のエラー
var ErrDuplicatePathPrefix = errors.New("path_prefix already exists in this ingress route")

// IngressRouteService は IngressRoute・PathRule CRUD のビジネスロジックを定義するインターフェース
type IngressRouteService interface {
	ListIngressRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error)                        // ingress_route 一覧を取得する
	CreateIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error)                         // ingress_route を作成する（ホストは自動生成）
	DeleteIngressRoute(ctx context.Context, userID string, ingressRouteID string) error                                            // ingress_route を status=deleting にする
	ListPathRules(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error)                           // path_rule 一覧を取得する
	CreatePathRule(ctx context.Context, userID string, ingressRouteID string, req CreatePathRuleRequest) (*models.PathRule, error) // path_rule を作成する
	DeletePathRule(ctx context.Context, userID string, pathRuleID string) error                                                    // path_rule を status=deleting にする
}

// CreatePathRuleRequest は POST /ingress-routes/:id/path-rules のリクエスト構造体
type CreatePathRuleRequest struct {
	PathPrefix string `json:"path_prefix"` // ルーティング対象パス
	ServiceID  string `json:"service_id"`  // 対象 Service の ID
}

// ingressRouteServiceImpl は IngressRouteService の実装
type ingressRouteServiceImpl struct {
	ingressRouteRepo repository.IngressRouteRepository // ingress_route リポジトリ
	pathRuleRepo     repository.PathRuleRepository     // path_rule リポジトリ
	projectRepo      repository.ProjectRepository      // project リポジトリ（所有権チェック用）
	baseDomain       string                            // ホスト自動生成に使うベースドメイン
}

// NewIngressRouteService は IngressRouteService の実装を返す
func NewIngressRouteService(
	ingressRouteRepo repository.IngressRouteRepository,
	pathRuleRepo repository.PathRuleRepository,
	projectRepo repository.ProjectRepository,
	baseDomain string,
) IngressRouteService {
	return &ingressRouteServiceImpl{
		ingressRouteRepo: ingressRouteRepo, // ingress_route リポジトリを注入する
		pathRuleRepo:     pathRuleRepo,     // path_rule リポジトリを注入する
		projectRepo:      projectRepo,      // project リポジトリを注入する
		baseDomain:       baseDomain,       // ベースドメインを注入する
	}
}

// ListIngressRoutes は projectID に紐づく ingress_route 一覧を返す
func (svc *ingressRouteServiceImpl) ListIngressRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	return svc.ingressRouteRepo.FindAllByProjectID(ctx, projectID) // リポジトリ経由で ingress_route 一覧を取得する
}

// CreateIngressRoute は projectID に紐づく ingress_route を作成する
func (svc *ingressRouteServiceImpl) CreateIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}

	ingressRouteID := uuid.New().String()                                             // Go 側で UUID を生成する
	baseDomain := svc.baseDomain                                                      // ベースドメインを取得する
	if baseDomain == "" {                                                             // 環境変数が未設定の場合はフォールバックする
		baseDomain = os.Getenv("BASE_DOMAIN")
	}
	host := fmt.Sprintf("%s.%s", ingressRouteID, baseDomain)                         // {uuid}.{BASE_DOMAIN} 形式でホストを生成する

	ingressRouteData := &models.IngressRoute{
		ID:        ingressRouteID,                      // 生成した UUID を設定する
		ProjectID: projectID,                           // project ID を設定する
		Host:      host,                                // 自動生成したホストを設定する
		Status:    models.IngressRouteStatusPending,    // 初期ステータスを設定する
	}
	if err := svc.ingressRouteRepo.Create(ctx, nil, ingressRouteData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return ingressRouteData, nil // 作成した ingress_route を返す
}

// DeleteIngressRoute は ingress_route を status=deleting にする
func (svc *ingressRouteServiceImpl) DeleteIngressRoute(ctx context.Context, userID string, ingressRouteID string) error {
	ingressRouteData, err := svc.ingressRouteRepo.FindByID(ctx, ingressRouteID) // ingress_route を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkProjectOwnership(ctx, userID, ingressRouteData.ProjectID); err != nil { // 所有権を確認する
		return err
	}
	ingressRouteData.Status = models.IngressRouteStatusDeleting        // 削除待ち状態に変更する
	return svc.ingressRouteRepo.Update(ctx, nil, ingressRouteData)     // DB に保存する
}

// ListPathRules は ingressRouteID に紐づく path_rule 一覧を返す
func (svc *ingressRouteServiceImpl) ListPathRules(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error) {
	if err := svc.checkIngressRouteOwnership(ctx, userID, ingressRouteID); err != nil { // 所有権を確認する
		return nil, err
	}
	return svc.pathRuleRepo.FindByIngressRouteID(ctx, ingressRouteID) // リポジトリ経由で一覧を取得する
}

// CreatePathRule は ingressRouteID に紐づく path_rule を作成する
func (svc *ingressRouteServiceImpl) CreatePathRule(ctx context.Context, userID string, ingressRouteID string, req CreatePathRuleRequest) (*models.PathRule, error) {
	if err := svc.checkIngressRouteOwnership(ctx, userID, ingressRouteID); err != nil { // 所有権を確認する
		return nil, err
	}

	// 同一 IngressRoute 内で同じ path_prefix が既に存在するか確認する（deleting 中も含めて弾く）
	existingRules, err := svc.pathRuleRepo.FindByIngressRouteID(ctx, ingressRouteID) // 既存ルール一覧を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	for _, existingRule := range existingRules {
		if existingRule.PathPrefix == req.PathPrefix { // 同じパスが既に存在する場合はエラーを返す
			return nil, ErrDuplicatePathPrefix
		}
	}

	pathRuleData := &models.PathRule{
		IngressRouteID: ingressRouteID,               // ingress_route ID を設定する
		PathPrefix:     req.PathPrefix,               // パスプレフィックスを設定する
		ServiceID:      req.ServiceID,                // 対象 Service ID を設定する
		Status:         models.PathRuleStatusPending, // 初期ステータスを pending にする
	}
	if err := svc.pathRuleRepo.Create(ctx, nil, pathRuleData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return pathRuleData, nil // 作成した path_rule を返す
}

// DeletePathRule は path_rule を status=deleting にする
func (svc *ingressRouteServiceImpl) DeletePathRule(ctx context.Context, userID string, pathRuleID string) error {
	pathRuleData, err := svc.pathRuleRepo.FindByID(ctx, pathRuleID) // path_rule を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkIngressRouteOwnership(ctx, userID, pathRuleData.IngressRouteID); err != nil { // 所有権を確認する
		return err
	}
	return svc.pathRuleRepo.UpdateStatus(ctx, nil, pathRuleID, models.PathRuleStatusDeleting) // status を deleting に更新する
}

// checkProjectOwnership は projectID に対応するプロジェクトの所有者を確認する
func (svc *ingressRouteServiceImpl) checkProjectOwnership(ctx context.Context, userID string, projectID string) error {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}
	return nil // 正常終了
}

// checkIngressRouteOwnership は IngressRoute を辿って所有者を確認する
func (svc *ingressRouteServiceImpl) checkIngressRouteOwnership(ctx context.Context, userID string, ingressRouteID string) error {
	ingressRouteData, err := svc.ingressRouteRepo.FindByID(ctx, ingressRouteID) // ingress_route を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	return svc.checkProjectOwnership(ctx, userID, ingressRouteData.ProjectID) // プロジェクト所有権を確認する
}

// ErrIngressRouteNotFound は ingress_route が見つからない場合のエラー
var ErrIngressRouteNotFound = errors.New("ingress_route not found")

// ErrPathRuleNotFound は path_rule が見つからない場合のエラー
var ErrPathRuleNotFound = gorm.ErrRecordNotFound
