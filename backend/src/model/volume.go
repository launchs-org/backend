package model

import (
	"backend/database" // データベースパッケージ
	"time"             // 時間
)

// Volume は永続化ボリュームを表すモデルです
type Volume struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`          // ボリュームID
	ProjectID   string    `gorm:"index;type:varchar(255);not null" json:"project_id"` // プロジェクトID
	ContainerID string    `gorm:"index;type:varchar(255)" json:"container_id"`       // マウント先コンテナID (NULL可)
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`          // ボリューム名
	SizeMB      int       `gorm:"not null" json:"size_mb"`                         // サイズ (MB)
	MountPath   string    `gorm:"type:varchar(255);not null" json:"mount_path"`    // コンテナ内のマウントパス
	Status      string    `gorm:"type:varchar(50);default:'Available'" json:"status"` // ステータス (Available, Deleting)
	CreatedAt   time.Time `json:"created_at"`                                      // 作成日時
	UpdatedAt   time.Time `json:"updated_at"`                                      // 更新日時
}

// CreateVolume はボリュームをデータベースに保存します
func CreateVolume(volume *Volume) error {
	// ボリュームレコードをDBに作成する
	return database.DB.Create(volume).Error
}

// GetVolumesByProjectID はプロジェクトIDに紐づくボリューム一覧を取得します
func GetVolumesByProjectID(projectID string) ([]Volume, error) {
	// 取得結果を格納するスライス
	var volumes []Volume
	// プロジェクトIDが一致するボリュームを全件取得する
	err := database.DB.Where("project_id = ?", projectID).Find(&volumes).Error
	// エラーを返す
	return volumes, err
}

// GetVolumesByContainerID はコンテナIDに紐づくボリューム一覧を取得します
func GetVolumesByContainerID(containerID string) ([]Volume, error) {
	// 取得結果を格納するスライス
	var volumes []Volume
	// コンテナIDが一致するボリュームを全件取得する
	err := database.DB.Where("container_id = ?", containerID).Find(&volumes).Error
	// エラーを返す
	return volumes, err
}

// GetVolumeByID はIDからボリュームを取得します
func GetVolumeByID(id string) (*Volume, error) {
	// 取得結果を格納する変数
	var volume Volume
	// IDが一致するボリュームを1件取得する
	err := database.DB.Where("id = ?", id).First(&volume).Error
	// エラーがある場合
	if err != nil {
		// nilとエラーを返す
		return nil, err
	}
	// 取得したボリュームを返す
	return &volume, nil
}

// UpdateVolume はボリューム情報を更新します
func UpdateVolume(volume *Volume) error {
	// ボリュームレコードを更新する
	return database.DB.Save(volume).Error
}

// DeleteVolume はボリュームを削除します
func DeleteVolume(id string) error {
	// IDが一致するボリュームを削除する
	return database.DB.Where("id = ?", id).Delete(&Volume{}).Error
}
