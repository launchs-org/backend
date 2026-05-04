package model

import (
	"time"
	"backend/database"
)

// Ingress はK8s Ingress設定を表すモデルです
type Ingress struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`                        // Ingress ID
	ContainerID string    `gorm:"uniqueIndex;type:varchar(255)" json:"container_id"`             // コンテナID
	Subdomain   string    `json:"subdomain"`                                   // サブドメイン
	HttpPort    int       `json:"http_port"`                                   // HTTPポート
	TlsEnabled  bool      `json:"tls_enabled"`                                 // TLS有効フラグ
	CustomDomain        string    `json:"custom_domain"`                                // カスタムドメイン
	CustomDomainEnabled bool      `json:"custom_domain_enabled"`                        // カスタムドメイン有効フラグ
	CreatedAt           time.Time `json:"created_at"`                                   // 作成日時
	UpdatedAt    time.Time `json:"updated_at"`                                   // 更新日時
}

// GetIngressByContainerID はコンテナIDからIngress設定を取得します
func GetIngressByContainerID(containerID string) (*Ingress, error) {
	// Ingress変数を宣言
	var ingress Ingress
	// データベースから取得
	err := database.DB.Where("container_id = ?", containerID).First(&ingress).Error
	// エラーがある場合
	if err != nil {
		// nilとエラーを返す
		return nil, err
	}
	// Ingressを返す
	return &ingress, nil
}

// CreateIngress はIngress設定を保存します
func CreateIngress(ingress *Ingress) error {
	// データベースに作成
	return database.DB.Create(ingress).Error
}

// UpdateIngress はIngress設定を更新します
func UpdateIngress(ingress *Ingress) error {
	// データベースのレコードを更新
	return database.DB.Save(ingress).Error
}

// DeleteIngress はIngress設定を削除します
func DeleteIngress(containerID string) error {
	// コンテナIDに紐づくレコードを削除
	return database.DB.Where("container_id = ?", containerID).Delete(&Ingress{}).Error
}
