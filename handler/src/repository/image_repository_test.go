package repository

import (
	"handler/models"
	"context"
	"testing"
)

// TestImageRepository_Create はイメージレコードが作成できることを確認する
func TestImageRepository_Create(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-create", // プロジェクト名を設定する
		Namespace: "ns-image-create",           // namespace を設定する
		UserID:    "user-1",                    // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,  // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	imageData := &models.Image{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		ImageURL:  "harbor.example.com/a/b:1", // イメージ URL を設定する
	}
	if err := imageRepo.Create(ctx, imageData); err != nil { // イメージレコードを作成する
		t.Fatalf("Create() がエラーを返しました: %v", err)
	}
	defer db.Delete(imageData) // テスト後にイメージレコードを削除する

	if imageData.ID == "" { // ID が採番されたことを確認する
		t.Error("作成後にイメージ ID が設定されていません")
	}
	if imageData.BuildID != nil { // BuildID が未設定の場合は nil であることを確認する
		t.Error("BuildID を指定していないのに nil ではありません")
	}
}

// TestImageRepository_FindByID は ID でイメージレコードが取得できることを確認する
func TestImageRepository_FindByID(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-find", // プロジェクト名を設定する
		Namespace: "ns-image-find",           // namespace を設定する
		UserID:    "user-1",                  // ユーザー ID を設定する
		Status:    models.ProjectStatusActive, // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	imageData := &models.Image{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		ImageURL:  "harbor.example.com/a/b:2", // イメージ URL を設定する
	}
	if err := db.Create(imageData).Error; err != nil { // イメージレコードを作成する
		t.Fatalf("テスト用イメージレコードの作成に失敗しました: %v", err)
	}
	defer db.Delete(imageData) // テスト後にイメージレコードを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	foundImage, err := imageRepo.FindByID(ctx, imageData.ID) // イメージレコードを取得する
	if err != nil {
		t.Fatalf("FindByID() がエラーを返しました: %v", err)
	}
	if foundImage.ImageURL != imageData.ImageURL { // イメージ URL が一致することを確認する
		t.Errorf("期待するイメージURL %s、実際のイメージURL %s", imageData.ImageURL, foundImage.ImageURL)
	}
}

// TestImageRepository_FindByID_NotFound は存在しない ID の場合にエラーが返ることを確認する
func TestImageRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	if _, err := imageRepo.FindByID(ctx, "00000000-0000-0000-0000-000000000000"); err == nil { // 存在しない ID で取得を試みる
		t.Error("存在しない ID の場合にエラーが返されませんでした")
	}
}

// TestImageRepository_FindByBuildID は buildID に紐づくイメージレコードが取得できることを確認する
func TestImageRepository_FindByBuildID(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-by-build", // プロジェクト名を設定する
		Namespace: "ns-image-by-build",           // namespace を設定する
		UserID:    "user-1",                      // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,    // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	buildData := &models.DeploymentBuild{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		BuildType: models.BuildTypeRailpack,   // ビルドタイプを設定する
		Status:    models.BuildStatusSucceeded, // ステータスを設定する
	}
	if err := db.Create(buildData).Error; err != nil { // ビルドレコードを作成する
		t.Fatalf("テスト用ビルドレコードの作成に失敗しました: %v", err)
	}
	defer db.Delete(buildData) // テスト後にビルドレコードを削除する

	imageData := &models.Image{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		BuildID:   &buildData.ID,              // ビルド ID を設定する
		ImageURL:  "harbor.example.com/a/b:3", // イメージ URL を設定する
	}
	if err := db.Create(imageData).Error; err != nil { // イメージレコードを作成する
		t.Fatalf("テスト用イメージレコードの作成に失敗しました: %v", err)
	}
	defer db.Delete(imageData) // テスト後にイメージレコードを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	foundImage, err := imageRepo.FindByBuildID(ctx, buildData.ID) // buildID でイメージレコードを取得する
	if err != nil {
		t.Fatalf("FindByBuildID() がエラーを返しました: %v", err)
	}
	if foundImage.ID != imageData.ID { // 取得したイメージが期待通りであることを確認する
		t.Errorf("期待するイメージID %s、実際のイメージID %s", imageData.ID, foundImage.ID)
	}
}

// TestImageRepository_FindAllByProjectID はプロジェクト単位のイメージ一覧が Build を含めて取得できることを確認する
func TestImageRepository_FindAllByProjectID(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-list", // プロジェクト名を設定する
		Namespace: "ns-image-list",           // namespace を設定する
		UserID:    "user-1",                  // ユーザー ID を設定する
		Status:    models.ProjectStatusActive, // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	buildData := &models.DeploymentBuild{
		ProjectID:     projectData.ID,             // プロジェクト ID を設定する
		BuildType:     models.BuildTypeRailpack,   // ビルドタイプを設定する
		Status:        models.BuildStatusSucceeded, // ステータスを設定する
		CommitMessage: "テストコミット",                // コミットメッセージを設定する
	}
	if err := db.Create(buildData).Error; err != nil { // ビルドレコードを作成する
		t.Fatalf("テスト用ビルドレコードの作成に失敗しました: %v", err)
	}
	defer db.Delete(buildData) // テスト後にビルドレコードを削除する

	imageData1 := &models.Image{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		BuildID:   &buildData.ID,              // ビルド ID を設定する（railpack 経由）
		ImageURL:  "harbor.example.com/a/b:4", // イメージ URL を設定する
	}
	imageData2 := &models.Image{
		ProjectID: projectData.ID,               // プロジェクト ID を設定する
		ImageURL:  "docker.io/library/nginx:latest", // 外部イメージ URL を設定する（BuildID は nil）
	}
	if err := db.Create(imageData1).Error; err != nil { // 1件目のイメージを作成する
		t.Fatalf("テスト用イメージレコード1の作成に失敗しました: %v", err)
	}
	defer db.Delete(imageData1) // テスト後にイメージレコードを削除する
	if err := db.Create(imageData2).Error; err != nil { // 2件目のイメージを作成する
		t.Fatalf("テスト用イメージレコード2の作成に失敗しました: %v", err)
	}
	defer db.Delete(imageData2) // テスト後にイメージレコードを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	images, err := imageRepo.FindAllByProjectID(ctx, projectData.ID) // プロジェクト単位のイメージ一覧を取得する
	if err != nil {
		t.Fatalf("FindAllByProjectID() がエラーを返しました: %v", err)
	}
	if len(images) != 2 { // イメージ件数を確認する
		t.Fatalf("期待するイメージ件数 2、実際の件数 %d", len(images))
	}

	// railpack 経由のイメージについて Build が Preload されていることを確認する
	var railpackImageFound bool
	for _, image := range images {
		if image.BuildID != nil && *image.BuildID == buildData.ID { // ビルド経由のイメージを特定する
			railpackImageFound = true
			if image.Build == nil { // Build が Preload されていることを確認する
				t.Error("Build が Preload されていません")
			} else if image.Build.CommitMessage != "テストコミット" { // Preload された Build の内容を確認する
				t.Errorf("期待するコミットメッセージ テストコミット、実際のメッセージ %s", image.Build.CommitMessage)
			}
		}
	}
	if !railpackImageFound { // railpack 経由のイメージが見つかったことを確認する
		t.Error("railpack 経由のイメージが一覧に含まれていません")
	}
}

// TestImageRepository_UpdateSizeBytes はイメージサイズが更新できることを確認する
func TestImageRepository_UpdateSizeBytes(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-size", // プロジェクト名を設定する
		Namespace: "ns-image-size",           // namespace を設定する
		UserID:    "user-1",                  // ユーザー ID を設定する
		Status:    models.ProjectStatusActive, // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	imageData := &models.Image{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		ImageURL:  "harbor.example.com/a/b:5", // イメージ URL を設定する
	}
	if err := db.Create(imageData).Error; err != nil { // イメージレコードを作成する
		t.Fatalf("テスト用イメージレコードの作成に失敗しました: %v", err)
	}
	defer db.Delete(imageData) // テスト後にイメージレコードを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	if err := imageRepo.UpdateSizeBytes(ctx, imageData.ID, 12345); err != nil { // サイズを更新する
		t.Fatalf("UpdateSizeBytes() がエラーを返しました: %v", err)
	}

	var updatedImage models.Image
	if err := db.First(&updatedImage, "id = ?", imageData.ID).Error; err != nil { // 更新後のレコードを取得する
		t.Fatalf("更新後のイメージレコードの取得に失敗しました: %v", err)
	}
	if updatedImage.SizeBytes != 12345 { // サイズが更新されたことを確認する
		t.Errorf("期待するサイズ 12345、実際のサイズ %d", updatedImage.SizeBytes)
	}
}

// TestImageRepository_UpdateSizeBytes_NotFound は存在しない ID の場合にエラーが返ることを確認する
func TestImageRepository_UpdateSizeBytes_NotFound(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	if err := imageRepo.UpdateSizeBytes(ctx, "00000000-0000-0000-0000-000000000000", 100); err == nil { // 存在しない ID で更新を試みる
		t.Error("存在しない ID の場合にエラーが返されませんでした")
	}
}

// TestImageRepository_Delete はイメージレコードが削除できることを確認する
func TestImageRepository_Delete(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-delete", // プロジェクト名を設定する
		Namespace: "ns-image-delete",           // namespace を設定する
		UserID:    "user-1",                    // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,  // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	imageData := &models.Image{
		ProjectID: projectData.ID,             // プロジェクト ID を設定する
		ImageURL:  "harbor.example.com/a/b:6", // イメージ URL を設定する
	}
	if err := db.Create(imageData).Error; err != nil { // イメージレコードを作成する
		t.Fatalf("テスト用イメージレコードの作成に失敗しました: %v", err)
	}

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	if err := imageRepo.Delete(ctx, imageData); err != nil { // イメージレコードを削除する
		t.Fatalf("Delete() がエラーを返しました: %v", err)
	}

	var count int64
	db.Model(&models.Image{}).Where("id = ?", imageData.ID).Count(&count) // 削除後の件数を確認する
	if count != 0 {                                                         // 件数が 0 であることを確認する
		t.Errorf("削除後のイメージ件数 0 を期待しましたが、実際の件数は %d です", count)
	}
}

// TestImageRepository_FindOrCreate_新規作成 は既存レコードがない場合に新規作成されることを確認する
func TestImageRepository_FindOrCreate_新規作成(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-foc-new", // プロジェクト名を設定する
		Namespace: "ns-image-foc-new",           // namespace を設定する
		UserID:    "user-1",                     // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,   // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	imageData, err := imageRepo.FindOrCreate(ctx, &models.Image{
		ProjectID: projectData.ID,               // プロジェクト ID を設定する
		ImageURL:  "harbor.example.com/a/b:foc", // イメージ URL を設定する
	})
	if err != nil {
		t.Fatalf("FindOrCreate() がエラーを返しました: %v", err)
	}
	defer db.Delete(imageData) // テスト後にイメージレコードを削除する

	if imageData.ID == "" { // ID が採番されたことを確認する
		t.Error("新規作成時にイメージ ID が設定されていません")
	}

	var count int64
	db.Model(&models.Image{}).Where("project_id = ? AND image_url = ?", projectData.ID, "harbor.example.com/a/b:foc").Count(&count) // 件数を確認する
	if count != 1 {                                                                                                                    // 1件のみ作成されたことを確認する
		t.Errorf("期待するイメージ件数 1、実際の件数 %d", count)
	}
}

// TestImageRepository_FindOrCreate_既存を再利用 は同一(project_id, image_url)で呼び出した場合に既存レコードが再利用され重複作成されないことを確認する
func TestImageRepository_FindOrCreate_既存を再利用(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	projectData := &models.Project{
		Name:      "test-project-image-foc-reuse", // プロジェクト名を設定する
		Namespace: "ns-image-foc-reuse",           // namespace を設定する
		UserID:    "user-1",                       // ユーザー ID を設定する
		Status:    models.ProjectStatusActive,     // ステータスを設定する
	}
	if err := db.Create(projectData).Error; err != nil { // プロジェクトを作成する
		t.Fatalf("テスト用プロジェクトの作成に失敗しました: %v", err)
	}
	defer db.Delete(projectData) // テスト後にプロジェクトを削除する

	imageRepo := NewImageRepository(db) // リポジトリを生成する
	imageURL := "harbor.example.com/a/b:foc-reuse"

	firstImage, err := imageRepo.FindOrCreate(ctx, &models.Image{ProjectID: projectData.ID, ImageURL: imageURL}) // 1回目の呼び出しで新規作成する
	if err != nil {
		t.Fatalf("1回目の FindOrCreate() がエラーを返しました: %v", err)
	}
	defer db.Delete(firstImage) // テスト後にイメージレコードを削除する

	secondImage, err := imageRepo.FindOrCreate(ctx, &models.Image{ProjectID: projectData.ID, ImageURL: imageURL}) // 2回目の呼び出しは既存を再利用するはず
	if err != nil {
		t.Fatalf("2回目の FindOrCreate() がエラーを返しました: %v", err)
	}

	if firstImage.ID != secondImage.ID { // 同じレコードが返されたことを確認する
		t.Errorf("既存レコードが再利用されず、別レコードが返されました: 1回目=%s, 2回目=%s", firstImage.ID, secondImage.ID)
	}

	var count int64
	db.Model(&models.Image{}).Where("project_id = ? AND image_url = ?", projectData.ID, imageURL).Count(&count) // 件数を確認する
	if count != 1 {                                                                                              // 重複作成されず1件のみであることを確認する
		t.Errorf("期待するイメージ件数 1、実際の件数 %d（重複作成が発生しています）", count)
	}
}

// TestImageRepository_FindByProjectIDAndURL_NotFound は存在しない組み合わせの場合にエラーが返ることを確認する
func TestImageRepository_FindByProjectIDAndURL_NotFound(t *testing.T) {
	db := setupTestDB(t) // テスト用 DB を準備する
	ctx := context.Background()

	imageRepo := NewImageRepository(db) // リポジトリを生成する

	if _, err := imageRepo.FindByProjectIDAndURL(ctx, "00000000-0000-0000-0000-000000000000", "no-such-url"); err == nil { // 存在しない組み合わせで検索する
		t.Error("存在しない組み合わせの場合にエラーが返されませんでした")
	}
}
