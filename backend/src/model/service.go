package model

import (
	"time"
	"backend/database"
)

// Service はK8s Service設定を表すモデルです
type Service struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`                        // サービスID
	ContainerID string    `gorm:"uniqueIndex;type:varchar(255)" json:"container_id"`             // コンテナID
	Type        string    `json:"type"`                                        // Serviceタイプ
	Ports       string    `gorm:"type:text" json:"ports"`                      // ポート設定(JSON)
	IsActive    bool      `json:"is_active"`                                   // 有効フラグ
	InternalIP  string    `json:"internal_ip"`                                 // 内部IP
	ExternalIP  string    `json:"external_ip"`                                 // 外部IP
	CreatedAt   time.Time `json:"created_at"`                                  // 作成日時
	UpdatedAt   time.Time `json:"updated_at"`                                  // 更新日時
}

// GetServiceByContainerID はコンテナIDからサービス設定を取得します
func GetServiceByContainerID(containerID string) (*Service, error) {
	// サービス変数を宣言
	var service Service
	// データベースから取得
	err := database.DB.Where("container_id = ?", containerID).First(&service).Error
	// エラーがある場合
	if err != nil {
		// nilとエラーを返す
		return nil, err
	}
	// サービスを返す
	return &service, nil
}

// UpdateService はサービス設定を更新します
func UpdateService(service *Service) error {
	// 更新日時をセット
	service.UpdatedAt = time.Now()
	// データベースを更新
	return database.DB.Save(service).Error
}
