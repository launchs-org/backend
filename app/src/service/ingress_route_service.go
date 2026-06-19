package service

import (
	"app/models"
	"app/repository"
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrIngressRouteNotFound は ingress_route が見つからない場合のエラー
var ErrIngressRouteNotFound = errors.New("ingress_route not found")

// ErrRouteNotFound はルートエントリが見つからない場合のエラー
var ErrRouteNotFound = errors.New("ingress_route_route not found")

// ErrDeploymentNotBelongToProject は DeploymentID がプロジェクトに属さない場合のエラー
var ErrDeploymentNotBelongToProject = errors.New("deployment does not belong to project")

// IngressRouteService は IngressRoute CRUD のビジネスロジックを定義するインターフェース
type IngressRouteService interface {
	GetIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error)                                               // ingress_route を取得する
	CreateIngressRoute(ctx context.Context, userID string, projectID string, req CreateIngressRouteRequest) (*models.IngressRoute, error)             // ingress_route を作成する
	UpdateIngressRoute(ctx context.Context, userID string, projectID string, req UpdateIngressRouteRequest) (*models.IngressRoute, error)             // ingress_route の pending フィールドを更新する
	DeleteIngressRoute(ctx context.Context, userID string, projectID string) error                                                                    // ingress_route と関連ルートを削除する
	ListRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRouteRoute, error)                                             // ルートエントリ一覧を取得する
	AddRoute(ctx context.Context, userID string, projectID string, req AddRouteRequest) (*models.IngressRouteRoute, error)                            // ルートエントリを追加する（pending 状態）
	UpdateRoute(ctx context.Context, userID string, projectID string, routeID string, req UpdateRouteRequest) (*models.IngressRouteRoute, error)      // ルートエントリを更新する（pending 状態に戻す）
	DeleteRoute(ctx context.Context, userID string, projectID string, routeID string) error                                                           // ルートエントリを deleting 状態にする
}

// CreateIngressRouteRequest は POST /projects/:id/ingress-route のリクエスト構造体
type CreateIngressRouteRequest struct {
	Host string `json:"host"` // ホスト名（省略時は自動生成）
}

// UpdateIngressRouteRequest は PUT /projects/:id/ingress-route のリクエスト構造体
type UpdateIngressRouteRequest struct {
	Host *string `json:"host"` // nil の場合は更新しない
}

// AddRouteRequest は POST /projects/:id/ingress-route/routes のリクエスト構造体
type AddRouteRequest struct {
	DeploymentID string `json:"deployment_id"` // ルーティング先 Deployment の ID
	PathPrefix   string `json:"path_prefix"`   // パスプレフィックス（例: /api）
	Port         int    `json:"port"`          // 転送先ポート番号
}

// UpdateRouteRequest は PUT /projects/:id/ingress-route/routes/:routeId のリクエスト構造体
type UpdateRouteRequest struct {
	DeploymentID *string `json:"deployment_id"` // nil の場合は更新しない
	PathPrefix   *string `json:"path_prefix"`   // nil の場合は更新しない
	Port         *int    `json:"port"`          // nil の場合は更新しない
}

// ingressRouteServiceImpl は IngressRouteService の実装
type ingressRouteServiceImpl struct {
	ingressRouteRepo      repository.IngressRouteRepository      // ingress_route リポジトリ
	ingressRouteRouteRepo repository.IngressRouteRouteRepository // ingress_route_route リポジトリ
	projectRepo           repository.ProjectRepository           // project リポジトリ（所有権チェック用）
	deploymentRepo        repository.DeploymentRepository        // deployment リポジトリ（DeploymentID 検証用）
	baseDomain            string                                 // ホスト自動生成に使うベースドメイン
}

// NewIngressRouteService は IngressRouteService の実装を返す
func NewIngressRouteService(
	ingressRouteRepo repository.IngressRouteRepository,
	ingressRouteRouteRepo repository.IngressRouteRouteRepository,
	projectRepo repository.ProjectRepository,
	deploymentRepo repository.DeploymentRepository,
	baseDomain string,
) IngressRouteService {
	return &ingressRouteServiceImpl{
		ingressRouteRepo:      ingressRouteRepo,      // ingress_route リポジトリを注入する
		ingressRouteRouteRepo: ingressRouteRouteRepo, // ingress_route_route リポジトリを注入する
		projectRepo:           projectRepo,           // project リポジトリを注入する
		deploymentRepo:        deploymentRepo,        // deployment リポジトリを注入する
		baseDomain:            baseDomain,            // ベースドメインを注入する
	}
}

// checkProjectOwnership は project の UserID と userID が一致するか確認する
func (svc *ingressRouteServiceImpl) checkProjectOwnership(ctx context.Context, userID string, projectID string) error {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}
	return nil // 所有権確認 OK
}

// checkDeploymentBelongsToProject は DeploymentID が指定プロジェクトに属するか確認する
func (svc *ingressRouteServiceImpl) checkDeploymentBelongsToProject(ctx context.Context, deploymentID string, projectID string) error {
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // deployment が見つからない場合
			return ErrDeploymentNotBelongToProject
		}
		return err // 取得エラーを返す
	}
	if deploymentData.ProjectID != projectID { // プロジェクトが一致しない場合はエラーを返す
		return ErrDeploymentNotBelongToProject
	}
	return nil // 検証 OK
}

// GetIngressRoute は projectID に紐づく ingress_route を返す
func (svc *ingressRouteServiceImpl) GetIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByProjectID(ctx, projectID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return ingressRouteData, nil // ingress_route を返す
}

// CreateIngressRoute は projectID に紐づく ingress_route を作成する
func (svc *ingressRouteServiceImpl) CreateIngressRoute(ctx context.Context, userID string, projectID string, req CreateIngressRouteRequest) (*models.IngressRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	host := req.Host                                               // リクエストのホスト名を使う
	if host == "" && svc.baseDomain != "" {                        // ホストが未指定かつ baseDomain が設定されている場合は自動生成する
		hostPrefix := projectID                                    // プロジェクト ID をプレフィックスとして使う
		if len(hostPrefix) > 8 {                                   // 長い場合は先頭 8 文字に切り詰める
			hostPrefix = hostPrefix[:8]
		}
		host = hostPrefix + "." + svc.baseDomain                   // {projectID[:8]}.baseDomain 形式で生成する
	}
	ingressRouteData := &models.IngressRoute{
		ProjectID: projectID,                        // プロジェクト ID を設定する
		Host:      host,                             // ホスト名を設定する
		Status:    models.IngressRouteStatusPending, // 初期ステータスを設定する
	}
	if err := svc.ingressRouteRepo.Create(ctx, ingressRouteData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return ingressRouteData, nil // 作成した ingress_route を返す
}

// UpdateIngressRoute は送られてきたフィールドのみ pending_* を更新する
func (svc *ingressRouteServiceImpl) UpdateIngressRoute(ctx context.Context, userID string, projectID string, req UpdateIngressRouteRequest) (*models.IngressRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByProjectID(ctx, projectID) // リポジトリ経由で取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if req.Host != nil {
		ingressRouteData.PendingHost = *req.Host // pending_host を更新する
	}
	if err := svc.ingressRouteRepo.Update(ctx, ingressRouteData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return ingressRouteData, nil // 更新後の ingress_route を返す
}

// DeleteIngressRoute は projectID に紐づく ingress_route と関連ルートエントリを削除する
func (svc *ingressRouteServiceImpl) DeleteIngressRoute(ctx context.Context, userID string, projectID string) error {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return err
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByProjectID(ctx, projectID) // リポジトリ経由で取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.ingressRouteRouteRepo.DeleteByIngressRouteID(ctx, nil, ingressRouteData.ID); err != nil { // 関連ルートエントリを全削除する
		return err // 削除エラーを返す
	}
	return svc.ingressRouteRepo.Delete(ctx, ingressRouteData.ID) // ingress_route レコードを削除する
}

// ListRoutes は projectID に紐づく ingress_route のルートエントリ一覧を返す
func (svc *ingressRouteServiceImpl) ListRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRouteRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByProjectID(ctx, projectID) // ingress_route を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return svc.ingressRouteRouteRepo.FindByIngressRouteID(ctx, ingressRouteData.ID) // ルートエントリ一覧を返す
}

// AddRoute は projectID に紐づく ingress_route にルートエントリを追加する（pending 状態）
func (svc *ingressRouteServiceImpl) AddRoute(ctx context.Context, userID string, projectID string, req AddRouteRequest) (*models.IngressRouteRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	if err := svc.checkDeploymentBelongsToProject(ctx, req.DeploymentID, projectID); err != nil { // DeploymentID の検証を行う
		return nil, err
	}
	ingressRouteData, err := svc.ingressRouteRepo.FindByProjectID(ctx, projectID) // ingress_route を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	routeData := &models.IngressRouteRoute{
		IngressRouteID: ingressRouteData.ID,             // 親 IngressRoute の ID を設定する
		DeploymentID:   req.DeploymentID,                // ルーティング先 Deployment の ID を設定する
		PathPrefix:     req.PathPrefix,                  // パスプレフィックスを設定する
		Port:           req.Port,                        // 転送先ポート番号を設定する
		Status:         models.IngressRouteRouteStatusPending, // 初期ステータスを pending に設定する
	}
	if err := svc.ingressRouteRouteRepo.Create(ctx, nil, routeData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return routeData, nil // 作成したルートエントリを返す
}

// UpdateRoute はルートエントリを更新し、ステータスを pending に戻す
func (svc *ingressRouteServiceImpl) UpdateRoute(ctx context.Context, userID string, projectID string, routeID string, req UpdateRouteRequest) (*models.IngressRouteRoute, error) {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return nil, err
	}
	routeData, err := svc.ingressRouteRouteRepo.FindByID(ctx, routeID) // ルートエントリを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if req.DeploymentID != nil { // DeploymentID が指定されている場合は検証して更新する
		if err := svc.checkDeploymentBelongsToProject(ctx, *req.DeploymentID, projectID); err != nil { // DeploymentID の検証を行う
			return nil, err
		}
		routeData.DeploymentID = *req.DeploymentID // DeploymentID を更新する
	}
	if req.PathPrefix != nil {
		routeData.PathPrefix = *req.PathPrefix // パスプレフィックスを更新する
	}
	if req.Port != nil {
		routeData.Port = *req.Port // 転送先ポート番号を更新する
	}
	routeData.Status = models.IngressRouteRouteStatusPending // 編集されたので pending に戻す
	if err := svc.ingressRouteRouteRepo.Update(ctx, routeData); err != nil { // リポジトリ経由で保存する
		return nil, err // 保存エラーを返す
	}
	return routeData, nil // 更新後のルートエントリを返す
}

// DeleteRoute はルートエントリのステータスを deleting に変更する（apply 後に物理削除される）
func (svc *ingressRouteServiceImpl) DeleteRoute(ctx context.Context, userID string, projectID string, routeID string) error {
	if err := svc.checkProjectOwnership(ctx, userID, projectID); err != nil { // 所有権を確認する
		return err
	}
	routeData, err := svc.ingressRouteRouteRepo.FindByID(ctx, routeID) // ルートエントリを取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	routeData.Status = models.IngressRouteRouteStatusDeleting              // ステータスを deleting に変更する
	return svc.ingressRouteRouteRepo.Update(ctx, routeData)                 // リポジトリ経由で保存する
}
