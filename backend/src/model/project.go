package model

import (
	"backend/database" // データベースパッケージ
	"time"             // 時間
	"gorm.io/gorm"     // GORM
)

// Project はプロジェクトを表すモデルです
type Project struct {
	ID              string         `gorm:"primaryKey;type:varchar(255)" json:"id"`                // プロジェクトID
	Name            string         `gorm:"type:varchar(255);not null" json:"name"`                // プロジェクト名
	K8sResourceName string         `gorm:"type:varchar(255);not null" json:"k8s_resource_name"`   // K8sリソース名
	Namespace       string         `gorm:"type:varchar(255);not null" json:"namespace"`           // Namespace
	OwnerID         string         `gorm:"type:varchar(255);not null;index" json:"owner_id"`      // 所有者ID
	CreatedAt       time.Time      `json:"created_at"`                                            // 作成日時
	UpdatedAt       time.Time      `json:"updated_at"`                                            // 更新日時
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`                                        // 削除日時 (ソフトデリート)
}

// CreateProject はプロジェクトをデータベースに保存します
func CreateProject(project *Project) error {
	// プロジェクトレコードをDBに作成する
	return database.DB.Create(project).Error
}

// GetProjectByName は名前からプロジェクトを取得します (重複チェック用)
func GetProjectByName(name string) (*Project, error) {
	// 取得結果を格納する変数
	var project Project
	// 名前が一致するプロジェクトを1件取得する
	err := database.DB.Where("name = ?", name).First(&project).Error
	// エラーがある場合 (見つからない場合を含む)
	if err != nil {
		// nilとエラーを返す
		return nil, err
	}
	// 取得したプロジェクトを返す
	return &project, nil
}
// GetProjectByID はIDからプロジェクトを取得します
func GetProjectByID(id string) (*Project, error) {
	// 取得結果を格納する変数
	var project Project
	// IDが一致するプロジェクトを1件取得する
	err := database.DB.Where("id = ?", id).First(&project).Error
	// エラーがある場合 (見つからない場合を含む)
	if err != nil {
		// nilとエラーを返す
		return nil, err
	}
	// 取得したプロジェクトを返す
	return &project, nil
}

// GetProjectsByOwnerID は所有者IDからプロジェクト一覧を取得します
func GetProjectsByOwnerID(ownerID string) ([]Project, error) {
	// 取得結果を格納するスライス
	var projects []Project
	// 所有者IDが一致するプロジェクトを全件取得する (ソフトデリートされていないもの)
	err := database.DB.Where("owner_id = ?", ownerID).Find(&projects).Error
	// エラーがある場合
	if err != nil {
		// 空のスライスとエラーを返す
		return nil, err
	}
	// 取得したプロジェクト一覧を返す
	return projects, nil
}

// プロジェクトを削除する関数
func DeleteProject(id string) error {
	// プロジェクトを削除
	return database.DB.Where("id = ?", id).Delete(&Project{}).Error
}