package models

import "time"

// Image はビルド成果物（Harbor に push されたコンテナイメージ）を表すモデル
// DeploymentBuild が「ビルド実行の記録」であるのに対し、Image は「成果物」として完全に分離する
type Image struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"                          json:"id"`
	ProjectID string `gorm:"type:uuid;not null;index;uniqueIndex:idx_images_project_id_image_url"    json:"project_id"` // 親プロジェクトID（Deployment削除後もImageを保持するため）。image_url との複合ユニークキー

	// BuildID は成果物を生んだビルドのID（railpack/dockerfileビルド経由の場合のみ設定）
	// image_url タイプ（外部イメージ直接指定）の Deployment 用に作られる Image は nil になる
	BuildID *string          `gorm:"type:uuid;index"    json:"build_id"`
	Build   *DeploymentBuild `gorm:"foreignKey:BuildID" json:"build,omitempty"`

	ImageURL  string `gorm:"type:text;not null;uniqueIndex:idx_images_project_id_image_url" json:"image_url"` // Harbor 上のイメージ URL、または外部指定URL（旧 BuiltImageURL）。project_id との複合ユニークキー
	SizeBytes int64  `gorm:"default:0"                                                      json:"size_bytes"` // Harbor に格納されたイメージサイズ（バイト単位、旧 ImageSizeBytes）

	CreatedAt time.Time `json:"created_at"`
}

func (Image) TableName() string { return "images" }
