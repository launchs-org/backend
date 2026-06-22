package service

import (
	"app/models"
	"app/repository"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"gorm.io/gorm"
)

// base62Chars は auto_generate で使用する文字セット（記号なし・アプリ互換）
const base62Chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// DeploymentTemplateService はテンプレート CRUD とテンプレートからのデプロイメント作成を定義するインターフェース
type DeploymentTemplateService interface {
	ListTemplates(ctx context.Context) ([]*models.DeploymentTemplate, error)                                                   // テンプレート一覧を取得する
	GetTemplate(ctx context.Context, templateID string) (*models.DeploymentTemplate, error)                                    // テンプレートを取得する
	CreateTemplate(ctx context.Context, createdBy string, req CreateTemplateRequest) (*models.DeploymentTemplate, error)       // テンプレートを作成する（管理者専用）
	UpdateTemplate(ctx context.Context, templateID string, req UpdateTemplateRequest) (*models.DeploymentTemplate, error)      // テンプレートを更新する（管理者専用）
	DeleteTemplate(ctx context.Context, templateID string) error                                                               // テンプレートを削除する（管理者専用）
	CreateDeploymentFromTemplate(ctx context.Context, userID string, req CreateDeploymentFromTemplateRequest) (*models.Deployment, error) // テンプレートからデプロイメントを作成する
}

// CreateTemplateRequest は POST /deployment-templates のリクエスト構造体
type CreateTemplateRequest struct {
	Name              string                   `json:"name"`                // テンプレート名
	Description       string                   `json:"description"`         // テンプレートの説明
	ImageURL          string                   `json:"image_url"`           // コンテナイメージ URL
	InstanceSize      string                   `json:"instance_size"`       // インスタンスサイズ
	Replicas          int32                    `json:"replicas"`            // レプリカ数
	Command           []string                 `json:"command"`             // コンテナコマンド
	Args              []string                 `json:"args"`                // コンテナ引数
	ServicePort       int                      `json:"service_port"`        // 公開ポート
	ServiceTargetPort int                      `json:"service_target_port"` // コンテナ内ポート
	ServiceType       string                   `json:"service_type"`        // サービスタイプ
	EnvVars           []models.TemplateEnvVar  `json:"env_vars"`            // 環境変数一覧
	Volumes           []models.TemplateVolume  `json:"volumes"`             // ボリューム一覧
}

// UpdateTemplateRequest は PUT /deployment-templates/:id のリクエスト構造体
type UpdateTemplateRequest struct {
	Name              *string                  `json:"name"`                // nil の場合は更新しない
	Description       *string                  `json:"description"`         // nil の場合は更新しない
	ImageURL          *string                  `json:"image_url"`           // nil の場合は更新しない
	InstanceSize      *string                  `json:"instance_size"`       // nil の場合は更新しない
	Replicas          *int32                   `json:"replicas"`            // nil の場合は更新しない
	Command           []string                 `json:"command"`             // nil の場合は更新しない
	Args              []string                 `json:"args"`                // nil の場合は更新しない
	ServicePort       *int                     `json:"service_port"`        // nil の場合は更新しない
	ServiceTargetPort *int                     `json:"service_target_port"` // nil の場合は更新しない
	ServiceType       *string                  `json:"service_type"`        // nil の場合は更新しない
	EnvVars           []models.TemplateEnvVar  `json:"env_vars"`            // nil の場合は更新しない
	Volumes           []models.TemplateVolume  `json:"volumes"`             // nil の場合は更新しない
}

// ExtraEnvVar はテンプレート適用時に追加する環境変数
type ExtraEnvVar struct {
	Key      string `json:"key"`       // 環境変数のキー
	Value    string `json:"value"`     // 環境変数の値
	IsSecret bool   `json:"is_secret"` // シークレットフラグ
}

// CreateDeploymentFromTemplateRequest は POST /projects/:id/deployments/from-template のリクエスト構造体
type CreateDeploymentFromTemplateRequest struct {
	ProjectID         string        `json:"-"`                  // パスパラメータから取得する
	TemplateID        string        `json:"template_id"`        // 使用するテンプレートの ID
	Name              string        `json:"name"`               // デプロイメント名（必須）
	ImageURL          *string       `json:"image_url"`          // nil の場合はテンプレート値を使用する
	InstanceSize      *string       `json:"instance_size"`      // nil の場合はテンプレート値を使用する
	Replicas          *int32        `json:"replicas"`           // nil の場合はテンプレート値を使用する
	SkipVolumeNames   []string      `json:"skip_volume_names"`  // 作成をスキップするボリューム名の一覧
	OverrideEnvVars   []ExtraEnvVar `json:"override_env_vars"`  // テンプレートのenv_varの値を上書きする（同キーのテンプレートenv_varに適用）
	ExtraEnvVars      []ExtraEnvVar `json:"extra_env_vars"`     // テンプレートに追加する環境変数（新規キーのみ）
}

// deploymentTemplateServiceImpl は DeploymentTemplateService の実装
type deploymentTemplateServiceImpl struct {
	db                  *gorm.DB                                  // トランザクション管理用 DB
	templateRepo        repository.DeploymentTemplateRepository   // テンプレートリポジトリ
	deploymentRepo      repository.DeploymentRepository           // デプロイメントリポジトリ
	serviceRepo         repository.ServiceRepository              // サービスリポジトリ
	envVarRepo          repository.EnvVarRepository               // 環境変数リポジトリ
	envVarMountRepo     repository.EnvVarMountRepository          // 環境変数マウントリポジトリ
	volumeRepo          repository.VolumeRepository               // ボリュームリポジトリ
	volumeMountRepo     repository.VolumeMountRepository          // ボリュームマウントリポジトリ
	projectRepo         repository.ProjectRepository              // プロジェクトリポジトリ（所有権チェック用）
	userQuotaRepo       repository.UserQuotaRepository            // クォータリポジトリ
}

// NewDeploymentTemplateService は DeploymentTemplateService の実装を返す
func NewDeploymentTemplateService(
	db *gorm.DB,
	templateRepo repository.DeploymentTemplateRepository,
	deploymentRepo repository.DeploymentRepository,
	serviceRepo repository.ServiceRepository,
	envVarRepo repository.EnvVarRepository,
	envVarMountRepo repository.EnvVarMountRepository,
	volumeRepo repository.VolumeRepository,
	volumeMountRepo repository.VolumeMountRepository,
	projectRepo repository.ProjectRepository,
	userQuotaRepo repository.UserQuotaRepository,
) DeploymentTemplateService {
	return &deploymentTemplateServiceImpl{
		db:              db,              // DB 接続を注入する
		templateRepo:    templateRepo,    // テンプレートリポジトリを注入する
		deploymentRepo:  deploymentRepo,  // デプロイメントリポジトリを注入する
		serviceRepo:     serviceRepo,     // サービスリポジトリを注入する
		envVarRepo:      envVarRepo,      // 環境変数リポジトリを注入する
		envVarMountRepo: envVarMountRepo, // 環境変数マウントリポジトリを注入する
		volumeRepo:      volumeRepo,      // ボリュームリポジトリを注入する
		volumeMountRepo: volumeMountRepo, // ボリュームマウントリポジトリを注入する
		projectRepo:     projectRepo,     // プロジェクトリポジトリを注入する
		userQuotaRepo:   userQuotaRepo,   // クォータリポジトリを注入する
	}
}

// ListTemplates はテンプレート一覧を返す
func (svc *deploymentTemplateServiceImpl) ListTemplates(ctx context.Context) ([]*models.DeploymentTemplate, error) {
	return svc.templateRepo.FindAll(ctx) // リポジトリ経由で取得する
}

// GetTemplate は templateID に対応するテンプレートを返す
func (svc *deploymentTemplateServiceImpl) GetTemplate(ctx context.Context, templateID string) (*models.DeploymentTemplate, error) {
	return svc.templateRepo.FindByID(ctx, templateID) // リポジトリ経由で取得する
}

// CreateTemplate はテンプレートを作成する（管理者専用）
func (svc *deploymentTemplateServiceImpl) CreateTemplate(ctx context.Context, createdBy string, req CreateTemplateRequest) (*models.DeploymentTemplate, error) {
	envVarsJSON, err := marshalTemplateEnvVars(req.EnvVars) // 環境変数を JSON に変換する
	if err != nil {
		return nil, err // 変換エラーを返す
	}
	volumesJSON, err := marshalTemplateVolumes(req.Volumes) // ボリュームを JSON に変換する
	if err != nil {
		return nil, err // 変換エラーを返す
	}

	// デフォルト値を設定する
	instanceSize := req.InstanceSize
	if instanceSize == "" {
		instanceSize = "small" // インスタンスサイズのデフォルトを設定する
	}
	replicas := req.Replicas
	if replicas == 0 {
		replicas = 1 // レプリカ数のデフォルトを設定する
	}

	templateData := &models.DeploymentTemplate{
		Name:              req.Name,                                        // テンプレート名を設定する
		Description:       req.Description,                                 // 説明を設定する
		Type:              models.DeploymentTypeImageURL,                   // image_url 固定
		ImageURL:          req.ImageURL,                                    // イメージ URL を設定する
		InstanceSize:      instanceSize,                                    // インスタンスサイズを設定する
		Replicas:          replicas,                                        // レプリカ数を設定する
		Command:           req.Command,                                     // コマンドを設定する
		Args:              req.Args,                                        // 引数を設定する
		ServicePort:       req.ServicePort,                                 // サービスポートを設定する
		ServiceTargetPort: req.ServiceTargetPort,                           // サービスターゲットポートを設定する
		ServiceType:       models.ServiceType(req.ServiceType),             // サービスタイプを設定する
		EnvVars:           envVarsJSON,                                     // 環境変数 JSON を設定する
		Volumes:           volumesJSON,                                     // ボリューム JSON を設定する
		CreatedBy:         createdBy,                                       // 作成者を設定する
	}

	if err := svc.templateRepo.Create(ctx, templateData); err != nil { // リポジトリ経由で作成する
		return nil, err // 作成エラーを返す
	}
	return templateData, nil // 作成したテンプレートを返す
}

// UpdateTemplate はテンプレートを更新する（管理者専用）
func (svc *deploymentTemplateServiceImpl) UpdateTemplate(ctx context.Context, templateID string, req UpdateTemplateRequest) (*models.DeploymentTemplate, error) {
	templateData, err := svc.templateRepo.FindByID(ctx, templateID) // テンプレートを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	// 各フィールドを更新する（nil でない場合のみ）
	if req.Name != nil {
		templateData.Name = *req.Name // テンプレート名を更新する
	}
	if req.Description != nil {
		templateData.Description = *req.Description // 説明を更新する
	}
	if req.ImageURL != nil {
		templateData.ImageURL = *req.ImageURL // イメージ URL を更新する
	}
	if req.InstanceSize != nil {
		templateData.InstanceSize = *req.InstanceSize // インスタンスサイズを更新する
	}
	if req.Replicas != nil {
		templateData.Replicas = *req.Replicas // レプリカ数を更新する
	}
	if req.Command != nil {
		templateData.Command = req.Command // コマンドを更新する
	}
	if req.Args != nil {
		templateData.Args = req.Args // 引数を更新する
	}
	if req.ServicePort != nil {
		templateData.ServicePort = *req.ServicePort // サービスポートを更新する
	}
	if req.ServiceTargetPort != nil {
		templateData.ServiceTargetPort = *req.ServiceTargetPort // サービスターゲットポートを更新する
	}
	if req.ServiceType != nil {
		templateData.ServiceType = models.ServiceType(*req.ServiceType) // サービスタイプを更新する
	}
	if req.EnvVars != nil {
		envVarsJSON, err := marshalTemplateEnvVars(req.EnvVars) // 環境変数を JSON に変換する
		if err != nil {
			return nil, err // 変換エラーを返す
		}
		templateData.EnvVars = envVarsJSON // 環境変数 JSON を更新する
	}
	if req.Volumes != nil {
		volumesJSON, err := marshalTemplateVolumes(req.Volumes) // ボリュームを JSON に変換する
		if err != nil {
			return nil, err // 変換エラーを返す
		}
		templateData.Volumes = volumesJSON // ボリューム JSON を更新する
	}

	if err := svc.templateRepo.Update(ctx, templateData); err != nil { // リポジトリ経由で更新する
		return nil, err // 更新エラーを返す
	}
	return templateData, nil // 更新したテンプレートを返す
}

// DeleteTemplate はテンプレートを削除する（管理者専用）
func (svc *deploymentTemplateServiceImpl) DeleteTemplate(ctx context.Context, templateID string) error {
	return svc.templateRepo.Delete(ctx, templateID) // リポジトリ経由で削除する
}

// CreateDeploymentFromTemplate はテンプレートからデプロイメントを作成する
func (svc *deploymentTemplateServiceImpl) CreateDeploymentFromTemplate(ctx context.Context, userID string, req CreateDeploymentFromTemplateRequest) (*models.Deployment, error) {
	templateData, err := svc.templateRepo.FindByID(ctx, req.TemplateID) // テンプレートを取得する
	if err != nil {
		return nil, err // テンプレートが見つからない場合はエラーを返す
	}

	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, req.ProjectID) // プロジェクトを取得して所有権を確認する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // 所有権チェックを行う
		return nil, ErrForbidden // 所有者以外のアクセスはエラーにする
	}

	if err := CheckDeploymentQuota(ctx, svc.userQuotaRepo, userID); err != nil { // デプロイメント数のクォータチェックを行う
		return nil, err // クォータ超過エラーを返す
	}

	// リクエストで上書きされた値を解決する（nil の場合はテンプレート値を使用する）
	imageURL := templateData.ImageURL // テンプレート値をデフォルトに設定する
	if req.ImageURL != nil {
		imageURL = *req.ImageURL // リクエスト値で上書きする
	}
	instanceSize := templateData.InstanceSize // テンプレート値をデフォルトに設定する
	if req.InstanceSize != nil {
		instanceSize = *req.InstanceSize // リクエスト値で上書きする
	}
	replicas := templateData.Replicas // テンプレート値をデフォルトに設定する
	if req.Replicas != nil {
		replicas = *req.Replicas // リクエスト値で上書きする
	}

	// テンプレートの環境変数とボリュームを展開する
	envVarList, err := unmarshalTemplateEnvVars(templateData.EnvVars) // 環境変数 JSON を展開する
	if err != nil {
		return nil, err // 展開エラーを返す
	}
	volumeList, err := unmarshalTemplateVolumes(templateData.Volumes) // ボリューム JSON を展開する
	if err != nil {
		return nil, err // 展開エラーを返す
	}

	var deploymentData *models.Deployment                                                                   // 作成したデプロイメントを格納する変数
	err = svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {                                    // トランザクションを開始する
		deploymentData = &models.Deployment{                                                               // デプロイメントレコードを構築する
			ProjectID:       req.ProjectID,                     // プロジェクト ID を設定する
			Name:            req.Name,                          // デプロイメント名を設定する
			Type:            models.DeploymentTypeImageURL,     // image_url タイプ固定
			Status:          models.DeploymentStatusPending,    // pending ステータスで作成する
			AppStatus:       models.AppStatusPending,           // 初期アプリステータスを設定する
			PendingImageURL: imageURL,                          // イメージ URL を pending に設定する
			InstanceSize:    instanceSize,                      // インスタンスサイズを設定する
			Replicas:        replicas,                          // レプリカ数を設定する
			Command:         templateData.Command,              // コマンドをテンプレートから引き継ぐ
			Args:            templateData.Args,                 // 引数をテンプレートから引き継ぐ
		}
		if err := svc.deploymentRepo.CreateWithTx(ctx, tx, deploymentData); err != nil { // デプロイメントをトランザクション内で作成する
			return err // 作成エラーを返してロールバックする
		}

		if templateData.ServicePort != 0 { // サービスポートが設定されている場合はサービスを作成する
			serviceData := &models.Service{
				DeploymentID:      deploymentData.ID,                           // デプロイメント ID を設定する
				PendingPort:       templateData.ServicePort,                    // pending ポートを設定する
				PendingTargetPort: templateData.ServiceTargetPort,              // pending ターゲットポートを設定する
				Type:              templateData.ServiceType,                    // サービスタイプを設定する
				Status:            models.ServiceStatusPending,                 // pending ステータスで作成する
			}
			if err := svc.serviceRepo.CreateWithTx(ctx, tx, serviceData); err != nil { // サービスをトランザクション内で作成する
				return err // 作成エラーを返してロールバックする
			}
		}

		overrideEnvVarMap := make(map[string]string, len(req.OverrideEnvVars)) // オーバーライドマップを構築する
		for _, overrideEnvVar := range req.OverrideEnvVars {
			overrideEnvVarMap[overrideEnvVar.Key] = overrideEnvVar.Value // キーに対する上書き値を登録する
		}

		for _, envVarDef := range envVarList { // 環境変数をループして作成する
			envVarValue := envVarDef.Value // デフォルト値を設定する
			if overriddenValue, hasOverride := overrideEnvVarMap[envVarDef.Key]; hasOverride && !envVarDef.AutoGenerate { // オーバーライドが指定されていてauto_generateでない場合
				envVarValue = overriddenValue // オーバーライド値を使用する
			}
			if envVarDef.AutoGenerate {    // auto_generate=true の場合はランダム値を生成する
				length := envVarDef.Length // 生成文字数を取得する
				if length == 0 {
					length = 32 // デフォルトの生成文字数を設定する
				}
				generated, err := generateRandomString(length) // ランダム文字列を生成する
				if err != nil {
					return err // 生成エラーを返してロールバックする
				}
				envVarValue = generated // 生成した値を使用する
			}
			envVarData := &models.EnvVar{
				ProjectID: req.ProjectID,    // プロジェクト ID を設定する
				Key:       envVarDef.Key,    // キーを設定する
				Value:     envVarValue,      // 値を設定する
				IsSecret:  envVarDef.IsSecret, // シークレットフラグを設定する
			}
			if err := svc.envVarRepo.Create(ctx, tx, envVarData); err != nil { // 環境変数を作成する
				return err // 作成エラーを返してロールバックする
			}
			envVarMountData := &models.EnvVarMount{
				EnvVarID:     envVarData.ID,     // 環境変数 ID を設定する
				DeploymentID: deploymentData.ID, // デプロイメント ID を設定する
				Status:       models.EnvVarMountStatusPending, // pending ステータスで作成する
			}
			if err := svc.envVarMountRepo.Create(ctx, tx, envVarMountData); err != nil { // マウント設定を作成する
				return err // 作成エラーを返してロールバックする
			}
		}

		skipVolumeSet := make(map[string]bool, len(req.SkipVolumeNames)) // スキップするボリューム名のセットを構築する
		for _, skipName := range req.SkipVolumeNames {
			skipVolumeSet[skipName] = true // スキップ対象として登録する
		}

		for _, volDef := range volumeList { // ボリュームをループして作成する
			if skipVolumeSet[volDef.Name] { // スキップ対象のボリュームは作成しない
				continue
			}
			volumeData := &models.Volume{
				ProjectID: req.ProjectID, // プロジェクト ID を設定する
				Name:      volDef.Name,   // ボリューム名を設定する
				SizeMB:    volDef.SizeMB, // ボリュームサイズを設定する
				Status:    models.VolumeStatusPending, // pending ステータスで作成する
			}
			if err := svc.volumeRepo.Create(ctx, tx, volumeData); err != nil { // ボリュームを作成する
				return err // 作成エラーを返してロールバックする
			}
			volumeMountData := &models.VolumeMount{
				VolumeID:     volumeData.ID,     // ボリューム ID を設定する
				DeploymentID: deploymentData.ID, // デプロイメント ID を設定する
				MountPath:    volDef.MountPath,  // マウントパスを設定する
				Status:       models.VolumeMountStatusPending, // pending ステータスで作成する
			}
			if err := svc.volumeMountRepo.Create(ctx, tx, volumeMountData); err != nil { // マウント設定を作成する
				return err // 作成エラーを返してロールバックする
			}
		}

		for _, extraEnvVar := range req.ExtraEnvVars { // 追加の環境変数をループして作成する
			extraEnvVarData := &models.EnvVar{
				ProjectID: req.ProjectID,       // プロジェクト ID を設定する
				Key:       extraEnvVar.Key,     // キーを設定する
				Value:     extraEnvVar.Value,   // 値を設定する
				IsSecret:  extraEnvVar.IsSecret, // シークレットフラグを設定する
			}
			if err := svc.envVarRepo.Create(ctx, tx, extraEnvVarData); err != nil { // 追加環境変数を作成する
				return err // 作成エラーを返してロールバックする
			}
			extraMountData := &models.EnvVarMount{
				EnvVarID:     extraEnvVarData.ID, // 環境変数 ID を設定する
				DeploymentID: deploymentData.ID,  // デプロイメント ID を設定する
				Status:       models.EnvVarMountStatusPending, // pending ステータスで作成する
			}
			if err := svc.envVarMountRepo.Create(ctx, tx, extraMountData); err != nil { // マウント設定を作成する
				return err // 作成エラーを返してロールバックする
			}
		}

		return nil // 全処理成功でコミットする
	})
	if err != nil {
		return nil, err // トランザクションエラーを返す
	}
	return deploymentData, nil // 作成したデプロイメントを返す
}

// generateRandomString は crypto/rand を使って length 文字のランダム文字列を生成する（base62）
func generateRandomString(length int) (string, error) {
	result := make([]byte, length)                                               // 結果バッファを確保する
	charsetLen := big.NewInt(int64(len(base62Chars)))                            // 文字セットの長さを設定する
	for charIndex := 0; charIndex < length; charIndex++ {                        // 各文字を生成する
		randomIndex, err := rand.Int(rand.Reader, charsetLen)                    // 乱数インデックスを生成する
		if err != nil {
			return "", fmt.Errorf("ランダム文字列の生成に失敗しました: %w", err) // 生成エラーを返す
		}
		result[charIndex] = base62Chars[randomIndex.Int64()] // 文字を設定する
	}
	return string(result), nil // 生成した文字列を返す
}

// marshalTemplateEnvVars は TemplateEnvVar スライスを datatypes.JSON に変換する
func marshalTemplateEnvVars(envVars []models.TemplateEnvVar) ([]byte, error) {
	if len(envVars) == 0 {
		return []byte("[]"), nil // 空スライスの場合は空配列 JSON を返す
	}
	data, err := json.Marshal(envVars) // JSON に変換する
	if err != nil {
		return nil, fmt.Errorf("環境変数の JSON 変換に失敗しました: %w", err) // 変換エラーを返す
	}
	return data, nil // JSON を返す
}

// marshalTemplateVolumes は TemplateVolume スライスを datatypes.JSON に変換する
func marshalTemplateVolumes(volumes []models.TemplateVolume) ([]byte, error) {
	if len(volumes) == 0 {
		return []byte("[]"), nil // 空スライスの場合は空配列 JSON を返す
	}
	data, err := json.Marshal(volumes) // JSON に変換する
	if err != nil {
		return nil, fmt.Errorf("ボリュームの JSON 変換に失敗しました: %w", err) // 変換エラーを返す
	}
	return data, nil // JSON を返す
}

// unmarshalTemplateEnvVars は datatypes.JSON を TemplateEnvVar スライスに展開する
func unmarshalTemplateEnvVars(data []byte) ([]models.TemplateEnvVar, error) {
	if len(data) == 0 {
		return nil, nil // データが空の場合は nil を返す
	}
	var envVarList []models.TemplateEnvVar                                                     // 展開先スライスを定義する
	if err := json.Unmarshal(data, &envVarList); err != nil {                                  // JSON を展開する
		return nil, fmt.Errorf("環境変数 JSON の展開に失敗しました: %w", err) // 展開エラーを返す
	}
	return envVarList, nil // 展開した一覧を返す
}

// unmarshalTemplateVolumes は datatypes.JSON を TemplateVolume スライスに展開する
func unmarshalTemplateVolumes(data []byte) ([]models.TemplateVolume, error) {
	if len(data) == 0 {
		return nil, nil // データが空の場合は nil を返す
	}
	var volumeList []models.TemplateVolume                                                      // 展開先スライスを定義する
	if err := json.Unmarshal(data, &volumeList); err != nil {                                   // JSON を展開する
		return nil, fmt.Errorf("ボリューム JSON の展開に失敗しました: %w", err) // 展開エラーを返す
	}
	return volumeList, nil // 展開した一覧を返す
}
