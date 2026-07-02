package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// TemplateEnvVar は env_vars jsonb カラムの要素型
type TemplateEnvVar struct {
	Key          string `json:"key"           yaml:"key"`          // 環境変数キー
	Value        string `json:"value"         yaml:"value"`        // 環境変数の値
	IsSecret     bool   `json:"is_secret"     yaml:"is_secret"`    // シークレットとして扱うか
	AutoGenerate bool   `json:"auto_generate" yaml:"auto_generate"` // true の場合は起動時にランダム生成する
	Length       int    `json:"length"        yaml:"length"`       // AutoGenerate=true 時の文字数（0 の場合は 32 にフォールバック）
}

// TemplateVolume は volumes jsonb カラムの要素型
type TemplateVolume struct {
	Name      string `json:"name"       yaml:"name"`       // ボリューム名
	SizeMB    int    `json:"size_mb"    yaml:"size_mb"`    // ボリュームサイズ（MB）
	MountPath string `json:"mount_path" yaml:"mount_path"` // コンテナ内マウントパス
}

// TemplateYAML は YAML パース用の中間構造体
type TemplateYAML struct {
	Name         string           `yaml:"name"`          // テンプレート名
	Description  string           `yaml:"description"`   // テンプレートの説明
	ImageURL     string           `yaml:"image_url"`     // コンテナイメージ URL
	InstanceSize string           `yaml:"instance_size"` // インスタンスサイズ
	Replicas     int32            `yaml:"replicas"`      // レプリカ数
	Command      []string         `yaml:"command"`       // コンテナコマンド
	Args         []string         `yaml:"args"`          // コンテナ引数
	Service      *TemplateService `yaml:"service"`       // サービス設定（nil = サービス不要）
	EnvVars      []TemplateEnvVar `yaml:"env_vars"`      // 環境変数一覧
	Volumes      []TemplateVolume `yaml:"volumes"`       // ボリューム一覧
}

// TemplateService は YAML の service セクションに対応する型
type TemplateService struct {
	Port       int    `yaml:"port"`        // 公開ポート番号
	TargetPort int    `yaml:"target_port"` // コンテナ内ポート番号
	Type       string `yaml:"type"`        // ClusterIP | NodePort | LoadBalancer
}

// DeploymentTemplate はシステム共通のデプロイメント設定テンプレート
type DeploymentTemplate struct {
	ID          string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`          // UUID 主キー
	Name        string         `gorm:"type:varchar(63);not null;uniqueIndex"          json:"name"`        // テンプレート名（一意）
	Description string         `gorm:"type:text"                                      json:"description"` // テンプレートの説明
	Type        DeploymentType `gorm:"type:varchar(32);not null;default:'image_url'"  json:"type"`        // デプロイメントタイプ（現状 image_url 固定）

	ImageURL     string         `gorm:"type:text"                                     json:"image_url"`     // コンテナイメージ URL
	InstanceSize string         `gorm:"type:varchar(16);not null;default:'small'"     json:"instance_size"` // インスタンスサイズ
	Replicas     int32          `gorm:"not null;default:1"                            json:"replicas"`      // レプリカ数
	Command      pq.StringArray `gorm:"type:text[]"                                   json:"command"`       // コンテナコマンド（ENTRYPOINT 上書き）
	Args         pq.StringArray `gorm:"type:text[]"                                   json:"args"`          // コンテナ引数（CMD 上書き）

	ServicePort       int         `gorm:"type:int"        json:"service_port"`        // 公開ポート（0 = サービス不要）
	ServiceTargetPort int         `gorm:"type:int"        json:"service_target_port"` // コンテナ内ポート
	ServiceType       ServiceType `gorm:"type:varchar(32)" json:"service_type"`        // サービスタイプ

	EnvVars datatypes.JSON `gorm:"type:jsonb" json:"env_vars"` // 環境変数テンプレート一覧
	Volumes datatypes.JSON `gorm:"type:jsonb" json:"volumes"`  // ボリューム設定一覧

	CreatedBy string    `gorm:"type:varchar(255);not null" json:"created_by"` // 作成者のユーザー ID
	CreatedAt time.Time `json:"created_at"`                                   // 作成日時
	UpdatedAt time.Time `json:"updated_at"`                                   // 更新日時
}

// TableName はテーブル名を明示する
func (DeploymentTemplate) TableName() string {
	return "deployment_templates" // テーブル名を返す
}
