package service

import (
	"backend/database"
	"backend/model"
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetProjectByID_PreloadVolumes(t *testing.T) {
	// 1. テスト用のDB初期化 (インメモリSQLite)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	// マイグレーション実行
	db.AutoMigrate(&model.Project{}, &model.Container{}, &model.Service{}, &model.Ingress{}, &model.Volume{})
	database.DB = db

	// 2. テストデータの作成
	projectID := uuid.New().String()
	containerID := uuid.New().String()
	userID := "test-user"

	project := &model.Project{
		ID:        projectID,
		Name:      "test-project",
		OwnerID:   userID,
		Namespace: "ns-test",
	}
	db.Create(project)

	container := &model.Container{
		ID:        containerID,
		ProjectID: projectID,
		Name:      "test-container",
	}
	db.Create(container)

	volume := &model.Volume{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		ContainerID: containerID,
		Name:        "test-volume",
		MountPath:   "/data",
		SizeMB:      100,
	}
	db.Create(volume)

	// 3. テストの実行
	got, err := GetProjectByID(context.Background(), projectID, userID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}

	// 4. 検証
	if len(got.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got.Containers))
	}

	if len(got.Containers[0].Volumes) != 1 {
		t.Fatalf("expected 1 volume in container, got %d", len(got.Containers[0].Volumes))
	}

	if got.Containers[0].Volumes[0].Name != "test-volume" {
		t.Errorf("expected volume name 'test-volume', got '%s'", got.Containers[0].Volumes[0].Name)
	}
}
