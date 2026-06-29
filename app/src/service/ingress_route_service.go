package service

import (
	"app/models"
	"app/repository"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// ErrDuplicatePathPrefix は同一 IngressRoute 内で同じ path_prefix が既に存在する場合のエラー
var ErrDuplicatePathPrefix = errors.New("path_prefix already exists in this ingress route")

// ErrDuplicateIngressRouteName はプロジェクト内で同一名の IngressRoute が存在する場合のエラー
var ErrDuplicateIngressRouteName = errors.New("ingress route name already exists in this project")

// ErrInvalidIngressRouteName は名前が DNS ラベルの形式に違反している場合のエラー
var ErrInvalidIngressRouteName = errors.New("invalid ingress route name: must be lowercase alphanumeric or hyphens, max 20 chars, no leading/trailing hyphens")

// ingressRouteNamePattern は有効な IngressRoute 名のパターン（英小文字・数字・ハイフン、先頭末尾はハイフン不可）
var ingressRouteNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,18}[a-z0-9])?$`)

// randomSuffixChars はサフィックス生成に使用する文字セット
const randomSuffixChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// IngressRouteService は IngressRoute・PathRule CRUD のビジネスロジックを定義するインターフェース
type IngressRouteService interface {
	ListIngressRoutes(ctx context.Context, userID string, projectID string) ([]*models.IngressRoute, error)                         // ingress_route 一覧を取得する
	CreateIngressRoute(ctx context.Context, userID string, projectID string, name string) (*models.IngressRoute, error)              // ingress_route を作成する（name 省略時は自動生成）
	UpdateIngressRouteName(ctx context.Context, userID string, ingressRouteID string, newName string) error                          // ingress_route の名前を変更する（pending_name に書き込み）
	DeleteIngressRoute(ctx context.Context, userID string, ingressRouteID string) error                                              // ingress_route を status=deleting にする
	ListPathRules(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error)                             // path_rule 一覧を取得する
	CreatePathRule(ctx context.Context, userID string, ingressRouteID string, req CreatePathRuleRequest) (*models.PathRule, error)   // path_rule を作成する
	DeletePathRule(ctx context.Context, userID string, pathRuleID string) error                                                      // path_rule を status=deleting にする
}

// CreatePathRuleRequest は POST /ingress-routes/:id/path-rules のリクエスト構造体
type CreatePathRuleRequest struct {
	PathPrefix  string `json:"path_prefix"`  // ルーティング対象パス
	ServiceID   string `json:"service_id"`   // 対象 Service の ID
	StripPrefix bool   `json:"strip_prefix"` // パスプレフィックスを strip するか
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
func (svc *ingressRouteServiceImpl) CreateIngressRoute(ctx context.Context, userID string, projectID string, name string) (*models.IngressRoute, error) {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // プロジェクトを取得して所有権確認と名前自動生成に使う
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // 所有者チェック
		return nil, ErrForbidden // 所有者でない場合は禁止エラーを返す
	}

	if name == "" { // 名前が未指定の場合はプロジェクト名からスラッグを生成する
		name = slugify(projectData.Name)
		if name == "" { // スラッグが空になる場合は "app" をデフォルトにする
			name = "app"
		}
	}

	if !ingressRouteNamePattern.MatchString(name) { // 名前のバリデーションを行う
		return nil, ErrInvalidIngressRouteName
	}

	nameExists, nameErr := svc.ingressRouteRepo.ExistsByNameInProject(ctx, nil, projectID, name) // プロジェクト内の名前重複チェック
	if nameErr != nil {
		return nil, nameErr // 確認エラーを返す
	}
	if nameExists { // 同名が既に存在する場合はエラーを返す
		return nil, ErrDuplicateIngressRouteName
	}

	baseDomain := svc.baseDomain          // ベースドメインを取得する
	if baseDomain == "" {                 // 環境変数が未設定の場合はフォールバックする
		baseDomain = os.Getenv("BASE_DOMAIN")
	}

	host, hostErr := svc.generateUniqueHost(ctx, name, baseDomain) // ユニークなホスト名を生成する
	if hostErr != nil {
		return nil, hostErr // 生成エラーを返す
	}

	ingressRouteData := &models.IngressRoute{
		ProjectID: projectID,                        // project ID を設定する
		Name:      name,                             // 名前を設定する
		Host:      host,                             // 自動生成したホストを設定する
		Status:    models.IngressRouteStatusPending, // 初期ステータスを設定する
	}
	if err := svc.ingressRouteRepo.Create(ctx, nil, ingressRouteData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return ingressRouteData, nil // 作成した ingress_route を返す
}

// UpdateIngressRouteName は ingress_route の名前を変更する（pending_name に書き込み、apply 時に反映）
func (svc *ingressRouteServiceImpl) UpdateIngressRouteName(ctx context.Context, userID string, ingressRouteID string, newName string) error {
	if !ingressRouteNamePattern.MatchString(newName) { // 名前のバリデーションを行う
		return ErrInvalidIngressRouteName
	}

	ingressRouteData, err := svc.ingressRouteRepo.FindByID(ctx, ingressRouteID) // ingress_route を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if err := svc.checkProjectOwnership(ctx, userID, ingressRouteData.ProjectID); err != nil { // 所有権を確認する
		return err
	}

	nameExists, nameErr := svc.ingressRouteRepo.ExistsByNameInProject(ctx, nil, ingressRouteData.ProjectID, newName) // プロジェクト内の名前重複チェック
	if nameErr != nil {
		return nameErr // 確認エラーを返す
	}
	if nameExists && newName != ingressRouteData.Name { // 現在の名前と異なるのに重複する場合はエラー
		return ErrDuplicateIngressRouteName
	}

	ingressRouteData.PendingName = newName                                      // 変更後の名前を pending_name に書き込む
	ingressRouteData.Status = models.IngressRouteStatusPending                  // apply 待ち状態にする
	return svc.ingressRouteRepo.Update(ctx, nil, ingressRouteData)              // DB に保存する
}

// generateUniqueHost はホスト衝突チェック付きで {name}-{8文字}.{baseDomain} を生成する（最大5回リトライ）
func (svc *ingressRouteServiceImpl) generateUniqueHost(ctx context.Context, name string, baseDomain string) (string, error) {
	for retryIndex := 0; retryIndex < 5; retryIndex++ { // 最大5回リトライする
		suffix, suffixErr := generateRandomSuffix(8) // 8文字のランダムサフィックスを生成する
		if suffixErr != nil {
			return "", suffixErr // 生成エラーを返す
		}
		host := fmt.Sprintf("%s-%s.%s", name, suffix, baseDomain) // {name}-{suffix}.{baseDomain} 形式でホストを生成する
		exists, existsErr := svc.ingressRouteRepo.ExistsByHost(ctx, nil, host) // ホストの重複チェック
		if existsErr != nil {
			return "", existsErr // 確認エラーを返す
		}
		if !exists { // 重複なしの場合は生成したホストを返す
			return host, nil
		}
	}
	return "", fmt.Errorf("ホスト名の生成に失敗しました（最大リトライ回数に達しました）") // リトライ上限に達した場合はエラーを返す
}

// generateRandomSuffix は指定された長さのランダムな英小文字・数字のサフィックスを生成する
func generateRandomSuffix(length int) (string, error) {
	result := make([]byte, length)                                               // バイトスライスを初期化する
	charsetLen := big.NewInt(int64(len(randomSuffixChars)))                      // 文字セットの長さを取得する
	for charIndex := range result {                                               // 各バイトにランダムな文字を設定する
		randomIndex, err := rand.Int(rand.Reader, charsetLen)                    // crypto/rand で安全な乱数を生成する
		if err != nil {
			return "", err // 乱数生成エラーを返す
		}
		result[charIndex] = randomSuffixChars[randomIndex.Int64()] // 文字セットからランダムに選択する
	}
	return string(result), nil // 生成したサフィックスを返す
}

// slugify はプロジェクト名を DNS ラベル互換のスラッグに変換する
func slugify(input string) string {
	lower := strings.ToLower(input)                                             // 小文字に変換する
	replaced := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(lower, "-")  // 英小文字・数字以外をハイフンに置換する
	trimmed := strings.Trim(replaced, "-")                                      // 先頭末尾のハイフンを除去する
	if len(trimmed) > 20 {                                                      // 最大20文字に切り詰める
		trimmed = strings.TrimRight(trimmed[:20], "-")                          // 切り詰め後に末尾のハイフンを除去する
	}
	return trimmed // スラッグを返す
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
		StripPrefix:    req.StripPrefix,              // strip_prefix フラグを設定する
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
