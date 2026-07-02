package service

import (
	"handler/models"
	"handler/repository"
	"context"
	"errors"
	"gorm.io/gorm"
)

// ErrDuplicateEnvVarKey は同一プロジェクト内でキーが重複している場合のエラー
var ErrDuplicateEnvVarKey = errors.New("同じキーの環境変数がすでに存在します")

// EnvVarService は環境変数 CRUD のビジネスロジックを定義するインターフェース
type EnvVarService interface {
	ListEnvVars(ctx context.Context, userID string, projectID string) ([]*models.EnvVar, error)                          // 環境変数一覧を取得する
	CreateEnvVar(ctx context.Context, userID string, projectID string, req CreateEnvVarRequest) (*models.EnvVar, error) // 環境変数を作成する
	UpdateEnvVar(ctx context.Context, userID string, envVarID string, req UpdateEnvVarRequest) (*models.EnvVar, error)  // 環境変数を更新する
	DeleteEnvVar(ctx context.Context, userID string, envVarID string) error                                              // 環境変数を削除する
}

// CreateEnvVarRequest は POST /projects/:id/env-vars のリクエスト構造体
type CreateEnvVarRequest struct {
	Key      string `json:"key"`       // 環境変数のキー
	Value    string `json:"value"`     // 環境変数の値
	IsSecret bool   `json:"is_secret"` // true の場合は k8s Secret に格納する
}

// UpdateEnvVarRequest は PUT /env-vars/:id のリクエスト構造体
type UpdateEnvVarRequest struct {
	Key      *string `json:"key"`       // nil の場合は更新しない
	Value    *string `json:"value"`     // nil の場合は更新しない
	IsSecret *bool   `json:"is_secret"` // nil の場合は更新しない
}

// envVarServiceImpl は EnvVarService の実装
type envVarServiceImpl struct {
	db          *gorm.DB                       // データベース接続（トランザクション開始に使用する）
	envVarRepo  repository.EnvVarRepository    // env_var リポジトリ
	projectRepo repository.ProjectRepository   // project リポジトリ（認可チェックに使用する）
}

// NewEnvVarService は EnvVarService の実装を返す
func NewEnvVarService(
	db *gorm.DB,
	envVarRepo repository.EnvVarRepository,
	projectRepo repository.ProjectRepository,
) EnvVarService {
	return &envVarServiceImpl{
		db:          db,          // DB 接続を注入する
		envVarRepo:  envVarRepo,  // env_var リポジトリを注入する
		projectRepo: projectRepo, // project リポジトリを注入する
	}
}

// checkProjectOwner は projectID に対応する project の UserID と userID を比較し、不一致の場合は ErrForbidden を返す
func (svc *envVarServiceImpl) checkProjectOwner(ctx context.Context, userID string, projectID string) (*models.Project, error) {
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // ユーザー ID が一致しない場合は forbidden を返す
		return nil, ErrForbidden // アクセス拒否エラーを返す
	}
	return projectData, nil // project を返す
}

// ListEnvVars は projectID に紐づく env_var 一覧を返す
func (svc *envVarServiceImpl) ListEnvVars(ctx context.Context, userID string, projectID string) ([]*models.EnvVar, error) {
	if _, err := svc.checkProjectOwner(ctx, userID, projectID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}
	return svc.envVarRepo.FindAllByProjectID(ctx, projectID) // リポジトリ経由で一覧を取得する
}

// CreateEnvVar は env_var を作成する
func (svc *envVarServiceImpl) CreateEnvVar(ctx context.Context, userID string, projectID string, req CreateEnvVarRequest) (*models.EnvVar, error) {
	if _, err := svc.checkProjectOwner(ctx, userID, projectID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}

	exists, err := svc.envVarRepo.ExistsByProjectIDAndKey(ctx, projectID, req.Key, "") // 同一プロジェクト内でキーが重複しているか確認する
	if err != nil {
		return nil, err // チェックエラーを返す
	}
	if exists { // 重複が存在する場合はエラーを返す
		return nil, ErrDuplicateEnvVarKey // 重複エラーを返す
	}

	var createdEnvVar *models.EnvVar                                               // 結果格納用変数を宣言する
	err = svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {            // トランザクションを開始する
		envVarData := &models.EnvVar{
			ProjectID: projectID,   // プロジェクト ID を設定する
			Key:       req.Key,     // キーを設定する
			Value:     req.Value,   // 値を設定する
			IsSecret:  req.IsSecret, // シークレットフラグを設定する
		}
		if err := svc.envVarRepo.Create(ctx, tx, envVarData); err != nil { // tx を渡してリポジトリに委譲する
			return err // エラーでロールバックする
		}
		createdEnvVar = envVarData // 作成した env_var を格納する
		return nil                 // コミットする
	})
	return createdEnvVar, err // 結果を返す
}

// UpdateEnvVar は envVarID に対応する env_var を更新する
func (svc *envVarServiceImpl) UpdateEnvVar(ctx context.Context, userID string, envVarID string, req UpdateEnvVarRequest) (*models.EnvVar, error) {
	// 認可チェック用に先にレコードを取得する（ProjectID が必要）
	envVarSnapshot, err := svc.envVarRepo.FindByID(ctx, envVarID) // env_var を取得する（トランザクション外）
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if _, err := svc.checkProjectOwner(ctx, userID, envVarSnapshot.ProjectID); err != nil { // 認可チェックを行う
		return nil, err // エラーを返す
	}

	var updatedEnvVar *models.EnvVar                                             // 結果格納用変数を宣言する
	txErr := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {       // トランザクションを開始する
		envVarData, err := svc.envVarRepo.FindByIDForUpdate(ctx, tx, envVarID) // FOR UPDATE ロックで再取得する
		if err != nil {
			return err // 取得エラーでロールバックする
		}

		if req.Key != nil {
			keyExists, keyCheckErr := svc.envVarRepo.ExistsByProjectIDAndKey(ctx, envVarData.ProjectID, *req.Key, envVarData.ID) // 更新後のキーが重複しないか確認する（自己除外あり）
			if keyCheckErr != nil {
				return keyCheckErr // チェックエラーでロールバックする
			}
			if keyExists { // 重複が存在する場合はエラーを返す
				return ErrDuplicateEnvVarKey // 重複エラーでロールバックする
			}
			envVarData.Key = *req.Key // キーを更新する
		}
		if req.Value != nil {
			envVarData.Value = *req.Value // 値を更新する
		}
		if req.IsSecret != nil {
			envVarData.IsSecret = *req.IsSecret // シークレットフラグを更新する
		}

		if err := svc.envVarRepo.Update(ctx, tx, envVarData); err != nil { // tx を渡してリポジトリに委譲する
			return err // 保存エラーでロールバックする
		}
		updatedEnvVar = envVarData // 更新した env_var を格納する
		return nil                 // コミットする
	})
	return updatedEnvVar, txErr // 結果を返す
}

// DeleteEnvVar は envVarID に対応する env_var を削除する
func (svc *envVarServiceImpl) DeleteEnvVar(ctx context.Context, userID string, envVarID string) error {
	envVarData, err := svc.envVarRepo.FindByID(ctx, envVarID) // env_var を取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	if _, err := svc.checkProjectOwner(ctx, userID, envVarData.ProjectID); err != nil { // 認可チェックを行う
		return err // エラーを返す
	}

	return svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // トランザクションを開始する
		return svc.envVarRepo.Delete(ctx, tx, envVarData) // tx を渡してリポジトリに委譲する
	})
}
