package repository

import (
	"handler/models"
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

	deploymentIDValue := deploymentData.ID // ポインタ用変数を宣言する
	buildData := &models.DeploymentBuild{
		ProjectID:    projectData.ID,            // プロジェクト ID を設定する
		DeploymentID: &deploymentIDValue,        // デプロイメント ID をポインタで設定する
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

	deploymentIDValue := deploymentData.ID // ポインタ用変数を宣言する
	buildData := &models.DeploymentBuild{
		ProjectID:    projectData.ID,            // プロジェクト ID を設定する
		DeploymentID: &deploymentIDValue,        // デプロイメント ID をポインタで設定する
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

// TestDeploymentBuildRepository_FindAllByProjectID はプロジェクト単位のビルド一覧が取得できることを確認する
func TestDeploymentBuildRepository_FindAllByProjectID(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-build-list-prj", // プロジェクト名を設定する
		Namespace: "ns-build-list-prj",           // namespace を設定する
		UserID:    "user-1",                      // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,    // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	// 同プロジェクトに2件のビルドを作成する
	buildData1 := &models.DeploymentBuild{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		BuildType: models.BuildTypeDockerfile, // ビルドタイプを設定する
		Status:    models.BuildStatusPending,  // ステータスを設定する
	}
	buildData2 := &models.DeploymentBuild{
		ProjectID: projectData.ID,            // プロジェクト ID を設定する
		BuildType: models.BuildTypeRailpack,  // ビルドタイプを設定する
		Status:    models.BuildStatusFailed,  // ステータスを設定する
	}
	if err := db.Create(buildData1).Error; err != nil { // 1件目のビルドを作成する
		t.Fatalf("テスト用ビルドレコード1の作成に失敗しました: %v", err)
	}
	defer db.Delete(buildData1) // テスト後にビルドレコードを削除する
	if err := db.Create(buildData2).Error; err != nil { // 2件目のビルドを作成する
		t.Fatalf("テスト用ビルドレコード2の作成に失敗しました: %v", err)
	}
	defer db.Delete(buildData2) // テスト後にビルドレコードを削除する

	buildRepo := NewDeploymentBuildRepository(db) // リポジトリを生成する

	builds, err := buildRepo.FindAllByProjectID(ctx, projectData.ID) // プロジェクト単位のビルド一覧を取得する
	if err != nil {                                                    // エラーが返った場合はテスト失敗
		t.Fatalf("FindAllByProjectID() がエラーを返しました: %v", err)
	}
	if len(builds) != 2 { // ビルド件数を確認する
		t.Errorf("期待するビルド件数 2、実際の件数 %d", len(builds))
	}
}

// TestDeploymentBuildRepository_DeleteAllByProjectID はプロジェクト単位でビルドを全件削除できることを確認する
func TestDeploymentBuildRepository_DeleteAllByProjectID(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-build-del-prj", // プロジェクト名を設定する
		Namespace: "ns-build-del-prj",           // namespace を設定する
		UserID:    "user-1",                     // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,   // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	buildData := &models.DeploymentBuild{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		BuildType: models.BuildTypeDockerfile, // ビルドタイプを設定する
		Status:    models.BuildStatusPending,  // ステータスを設定する
	}
	if err := db.Create(buildData).Error; err != nil { // ビルドレコードを作成する
		t.Fatalf("テスト用ビルドレコードの作成に失敗しました: %v", err)
	}

	buildRepo := NewDeploymentBuildRepository(db) // リポジトリを生成する

	if err := buildRepo.DeleteAllByProjectID(ctx, db, projectData.ID); err != nil { // プロジェクト単位で全削除する
		t.Fatalf("DeleteAllByProjectID() がエラーを返しました: %v", err)
	}

	var count int64
	db.Model(&models.DeploymentBuild{}).Where("project_id = ?", projectData.ID).Count(&count) // 削除後の件数を確認する
	if count != 0 {                                                                             // 件数が 0 であることを確認する
		t.Errorf("削除後のビルド件数 0 を期待しましたが、実際の件数は %d です", count)
	}
}

// TestDeploymentBuildRepository_ArchiveFields はArchiveFileName/ArchiveSizeBytesがCreate/FindByIDで往復できることを確認する
func TestDeploymentBuildRepository_ArchiveFields(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-build-archive", // プロジェクト名を設定する
		Namespace: "ns-build-archive",           // namespace を設定する
		UserID:    "user-1",                     // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,   // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,              // プロジェクト ID を設定する
		Name:      "test-deployment-build-archive", // デプロイメント名を設定する
		Type:      models.DeploymentTypeArchive, // archive タイプを設定する
		Status:    models.DeploymentStatusPending, // ステータスを設定する
		AppStatus: models.AppStatusPending,       // アプリステータスを設定する
	}
	if err := db.Create(deploymentData).Error; err != nil { // デプロイメントを作成する
		t.Fatalf("テスト用デプロイメントの作成に失敗しました: %v", err)
	}
	defer db.Delete(deploymentData) // テスト後にデプロイメントを削除する

	buildRepo := NewDeploymentBuildRepository(db) // リポジトリを生成する

	deploymentIDValue := deploymentData.ID // ポインタ用変数を宣言する
	buildData := &models.DeploymentBuild{
		ProjectID:        projectData.ID,           // プロジェクト ID を設定する
		DeploymentID:     &deploymentIDValue,        // デプロイメント ID をポインタで設定する
		BuildType:        models.BuildTypeRailpack,  // ビルドタイプを設定する
		Status:           models.BuildStatusPending, // ステータスを設定する
		ArchiveFileName:  "source.tar.gz",           // アーカイブファイル名を設定する
		ArchiveSizeBytes: 12345,                     // アーカイブサイズを設定する
	}

	if err := buildRepo.Create(ctx, buildData); err != nil { // ビルドレコードを作成する
		t.Fatalf("Create() がエラーを返しました: %v", err)
	}
	defer db.Delete(buildData) // テスト後にビルドレコードを削除する

	foundBuild, err := buildRepo.FindByID(ctx, buildData.ID) // ビルドレコードを取得する
	if err != nil {
		t.Fatalf("FindByID() がエラーを返しました: %v", err)
	}
	if foundBuild.ArchiveFileName != "source.tar.gz" { // アーカイブファイル名が往復できることを確認する
		t.Errorf("期待するArchiveFileName %s、実際の値 %s", "source.tar.gz", foundBuild.ArchiveFileName)
	}
	if foundBuild.ArchiveSizeBytes != 12345 { // アーカイブサイズが往復できることを確認する
		t.Errorf("期待するArchiveSizeBytes %d、実際の値 %d", 12345, foundBuild.ArchiveSizeBytes)
	}
}
