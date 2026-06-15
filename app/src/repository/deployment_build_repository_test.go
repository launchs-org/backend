package repository

import (
	"app/models"
	"context"
	"testing"
)

// TestDeploymentBuildRepository_Create はビルドレコードが作成できることを確認する
func TestDeploymentBuildRepository_Create(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	// テスト用の Project と Deployment を作成する
	projectData := &models.Project{
		Name:      "test-project-build-create", // プロジェクト名を設定する
		Namespace: "ns-build-create",           // namespace を設定する
		UserID:    "user-1",                    // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,  // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // プロジェクト ID を設定する
		Name:      "test-deployment-build-create", // デプロイメント名を設定する
		Type:      models.DeploymentTypeDockerfile, // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil { // デプロイメントを作成する
		t.Fatalf("テスト用デプロイメントの作成に失敗しました: %v", err)
	}
	defer db.Delete(deploymentData) // テスト後にデプロイメントを削除する

	buildRepo := NewDeploymentBuildRepository(db) // リポジトリを生成する

	buildData := &models.DeploymentBuild{
		DeploymentID: deploymentData.ID,         // デプロイメント ID を設定する
		BuildType:    models.BuildTypeDockerfile, // ビルドタイプを設定する
		Status:       models.BuildStatusPending,  // ステータスを設定する
	}

	if err := buildRepo.Create(ctx, buildData); err != nil { // ビルドレコードを作成する
		t.Fatalf("Create() がエラーを返しました: %v", err)
	}
	defer db.Delete(buildData) // テスト後にビルドレコードを削除する

	if buildData.ID == "" { // ID が採番されたことを確認する
		t.Error("作成後にビルド ID が設定されていません")
	}
	if buildData.Status != models.BuildStatusPending { // ステータスが pending であることを確認する
		t.Errorf("期待するステータス %s、実際のステータス %s", models.BuildStatusPending, buildData.Status)
	}
}

// TestDeploymentBuildRepository_UpdateStatus は ステータスが更新できることを確認する
func TestDeploymentBuildRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	// テスト用の Project と Deployment と DeploymentBuild を作成する
	projectData := &models.Project{
		Name:      "test-project-build-status", // プロジェクト名を設定する
		Namespace: "ns-build-status",           // namespace を設定する
		UserID:    "user-1",                    // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,  // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,                 // プロジェクト ID を設定する
		Name:      "test-deployment-build-status", // デプロイメント名を設定する
		Type:      models.DeploymentTypeDockerfile, // タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,        // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil { // デプロイメントを作成する
		t.Fatalf("テスト用デプロイメントの作成に失敗しました: %v", err)
	}
	defer db.Delete(deploymentData) // テスト後にデプロイメントを削除する

	buildData := &models.DeploymentBuild{
		DeploymentID: deploymentData.ID,         // デプロイメント ID を設定する
		BuildType:    models.BuildTypeDockerfile, // ビルドタイプを設定する
		Status:       models.BuildStatusPending,  // 初期ステータスを pending に設定する
	}
	if err := db.Create(buildData).Error; err != nil { // ビルドレコードを作成する
		t.Fatalf("テスト用ビルドレコードの作成に失敗しました: %v", err)
	}
	defer db.Delete(buildData) // テスト後にビルドレコードを削除する

	buildRepo := NewDeploymentBuildRepository(db) // リポジトリを生成する

	if err := buildRepo.UpdateStatus(ctx, buildData.ID, models.BuildStatusBuilding); err != nil { // ステータスを更新する
		t.Fatalf("UpdateStatus() がエラーを返しました: %v", err)
	}

	// 更新後のレコードを取得して確認する
	var updatedBuild models.DeploymentBuild
	if err := db.First(&updatedBuild, "id = ?", buildData.ID).Error; err != nil { // 更新後のレコードを取得する
		t.Fatalf("更新後のビルドレコードの取得に失敗しました: %v", err)
	}
	if updatedBuild.Status != models.BuildStatusBuilding { // ステータスが更新されたことを確認する
		t.Errorf("期待するステータス %s、実際のステータス %s", models.BuildStatusBuilding, updatedBuild.Status)
	}
}
