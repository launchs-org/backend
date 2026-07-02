package models

type InstanceSize struct {
	Size          string `gorm:"primaryKey;type:varchar(16)"`
	CPURequest    string `gorm:"type:varchar(16);not null"`
	CPULimit      string `gorm:"type:varchar(16);not null"`
	MemoryRequest string `gorm:"type:varchar(16);not null"`
	MemoryLimit   string `gorm:"type:varchar(16);not null"`
}

func (InstanceSize) TableName() string { return "instance_sizes" }
