package repository

import (
	"context"
	"handler/models"
	"testing"

	"gorm.io/gorm"
)

// TestDeploymentApplyProgressRepository_InitializeSteps_初回作成 は初回呼び出しで9ステップが正しく作成されることを確認する
func TestDeploymentApplyProgressRepository_InitializeSteps_初回作成(t *testing.T) {
	db := setupTestDB(t)                    // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-progress-init",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)                                  // テスト用レコードを作成する
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) }) // テスト終了後にレコードを削除する

	repo := NewDeploymentApplyProgressRepository(db) // リポジトリを生成する
	workflowID := "apply-" + deploymentData.ID       // workflow_id を組み立てる
	skippedSteps := map[models.ApplyProgressStepName]bool{
		models.ApplyProgressStepVolume: true, // ボリュームなしを想定して skip する
	}

	err := repo.InitializeSteps(context.Background(), nil, workflowID, deploymentData.ID, skippedSteps) // 初期化する
	if err != nil {
		t.Fatalf("InitializeSteps がエラーを返しました: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Where("workflow_id = ?", workflowID).Delete(&models.DeploymentApplyProgress{}) }) // テスト終了後にレコードを削除する

	progressList, err := repo.FindAllByWorkflowID(context.Background(), workflowID) // 全ステップを取得する
	if err != nil {
		t.Fatalf("FindAllByWorkflowID がエラーを返しました: %v", err)
	}
	if len(progressList) != 9 { // 9ステップ作成されていることを確認する
		t.Fatalf("期待する件数: 9, 実際の件数: %d", len(progressList))
	}
	if progressList[0].StepName != models.ApplyProgressStepVolume { // 1番目がvolumeであることを確認する
		t.Errorf("期待するstep_name: volume, 実際のstep_name: %s", progressList[0].StepName)
	}
	if progressList[0].Status != models.ApplyProgressStepStatusSkipped { // skip指定したステップがskippedであることを確認する
		t.Errorf("期待するstatus: skipped, 実際のstatus: %s", progressList[0].Status)
	}
	if progressList[1].Status != models.ApplyProgressStepStatusPending { // skip指定していないステップがpendingであることを確認する
		t.Errorf("期待するstatus: pending, 実際のstatus: %s", progressList[1].Status)
	}
}

// TestDeploymentApplyProgressRepository_InitializeSteps_リトライ時はリセットされ重複しない は同一workflow_idの再実行で件数が増えないことを確認する
func TestDeploymentApplyProgressRepository_InitializeSteps_リトライ時はリセットされ重複しない(t *testing.T) {
	db := setupTestDB(t)                    // テスト用 DB を準備する
	projectData := createTestProject(t, db) // テスト用 Project を作成する

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-progress-retry",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	repo := NewDeploymentApplyProgressRepository(db)
	workflowID := "apply-" + deploymentData.ID
	t.Cleanup(func() { db.Unscoped().Where("workflow_id = ?", workflowID).Delete(&models.DeploymentApplyProgress{}) })

	if err := repo.InitializeSteps(context.Background(), nil, workflowID, deploymentData.ID, map[models.ApplyProgressStepName]bool{}); err != nil { // 1回目の初期化を実行する
		t.Fatalf("1回目の InitializeSteps がエラーを返しました: %v", err)
	}
	if err := repo.UpdateStepStatus(context.Background(), nil, workflowID, models.ApplyProgressStepContainer, models.ApplyProgressStepStatusInProgress, ""); err != nil { // 途中まで進める
		t.Fatalf("UpdateStepStatus がエラーを返しました: %v", err)
	}

	if err := repo.InitializeSteps(context.Background(), nil, workflowID, deploymentData.ID, map[models.ApplyProgressStepName]bool{}); err != nil { // リトライを模倣して再度初期化する
		t.Fatalf("2回目の InitializeSteps がエラーを返しました: %v", err)
	}

	progressList, err := repo.FindAllByWorkflowID(context.Background(), workflowID)
	if err != nil {
		t.Fatalf("FindAllByWorkflowID がエラーを返しました: %v", err)
	}
	if len(progressList) != 9 { // 重複作成されていないことを確認する
		t.Fatalf("期待する件数: 9, 実際の件数: %d", len(progressList))
	}
	for _, progressItem := range progressList {
		if progressItem.StepName == models.ApplyProgressStepContainer && progressItem.Status != models.ApplyProgressStepStatusPending { // リセットされpendingに戻っていることを確認する
			t.Errorf("期待するstatus: pending, 実際のstatus: %s", progressItem.Status)
		}
	}
}

// TestDeploymentApplyProgressRepository_UpdateStepStatus_状態遷移とタイムスタンプ は状態更新でタイムスタンプが正しく設定されることを確認する
func TestDeploymentApplyProgressRepository_UpdateStepStatus_状態遷移とタイムスタンプ(t *testing.T) {
	db := setupTestDB(t)
	projectData := createTestProject(t, db)

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-progress-update",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	repo := NewDeploymentApplyProgressRepository(db)
	workflowID := "apply-" + deploymentData.ID
	t.Cleanup(func() { db.Unscoped().Where("workflow_id = ?", workflowID).Delete(&models.DeploymentApplyProgress{}) })

	if err := repo.InitializeSteps(context.Background(), nil, workflowID, deploymentData.ID, map[models.ApplyProgressStepName]bool{}); err != nil {
		t.Fatalf("InitializeSteps がエラーを返しました: %v", err)
	}

	if err := repo.UpdateStepStatus(context.Background(), nil, workflowID, models.ApplyProgressStepImage, models.ApplyProgressStepStatusInProgress, ""); err != nil { // in_progress に更新する
		t.Fatalf("UpdateStepStatus(in_progress) がエラーを返しました: %v", err)
	}
	if err := repo.UpdateStepStatus(context.Background(), nil, workflowID, models.ApplyProgressStepImage, models.ApplyProgressStepStatusFailed, "image not found"); err != nil { // failed に更新する
		t.Fatalf("UpdateStepStatus(failed) がエラーを返しました: %v", err)
	}

	progressList, err := repo.FindAllByWorkflowID(context.Background(), workflowID)
	if err != nil {
		t.Fatalf("FindAllByWorkflowID がエラーを返しました: %v", err)
	}
	var imageStep *models.DeploymentApplyProgress
	for _, progressItem := range progressList {
		if progressItem.StepName == models.ApplyProgressStepImage {
			imageStep = progressItem
		}
	}
	if imageStep == nil {
		t.Fatalf("image ステップが見つかりません")
	}
	if imageStep.Status != models.ApplyProgressStepStatusFailed { // status が failed であることを確認する
		t.Errorf("期待するstatus: failed, 実際のstatus: %s", imageStep.Status)
	}
	if imageStep.ErrorMessage != "image not found" { // error_message が設定されていることを確認する
		t.Errorf("期待するerror_message: image not found, 実際のerror_message: %s", imageStep.ErrorMessage)
	}
	if imageStep.StartedAt == nil { // started_at が設定されていることを確認する
		t.Error("started_at が設定されていません")
	}
	if imageStep.FinishedAt == nil { // finished_at が設定されていることを確認する
		t.Error("finished_at が設定されていません")
	}
}

// TestDeploymentApplyProgressRepository_FindLatestWorkflowIDByDeploymentID_最新を返す は複数workflowがある場合に最新のworkflow_idが返ることを確認する
func TestDeploymentApplyProgressRepository_FindLatestWorkflowIDByDeploymentID_最新を返す(t *testing.T) {
	db := setupTestDB(t)
	projectData := createTestProject(t, db)

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-progress-latest",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	repo := NewDeploymentApplyProgressRepository(db)
	oldWorkflowID := "apply-old-" + deploymentData.ID
	newWorkflowID := "apply-new-" + deploymentData.ID
	t.Cleanup(func() {
		db.Unscoped().Where("workflow_id IN ?", []string{oldWorkflowID, newWorkflowID}).Delete(&models.DeploymentApplyProgress{})
	})

	if err := repo.InitializeSteps(context.Background(), nil, oldWorkflowID, deploymentData.ID, map[models.ApplyProgressStepName]bool{}); err != nil { // 古いworkflowを先に作成する
		t.Fatalf("古いworkflowのInitializeStepsがエラーを返しました: %v", err)
	}
	if err := repo.InitializeSteps(context.Background(), nil, newWorkflowID, deploymentData.ID, map[models.ApplyProgressStepName]bool{}); err != nil { // 新しいworkflowを後から作成する
		t.Fatalf("新しいworkflowのInitializeStepsがエラーを返しました: %v", err)
	}

	latestWorkflowID, err := repo.FindLatestWorkflowIDByDeploymentID(context.Background(), deploymentData.ID) // 最新のworkflow_idを取得する
	if err != nil {
		t.Fatalf("FindLatestWorkflowIDByDeploymentID がエラーを返しました: %v", err)
	}
	if latestWorkflowID != newWorkflowID { // 新しいworkflow_idが返ることを確認する
		t.Errorf("期待するworkflow_id: %s, 実際のworkflow_id: %s", newWorkflowID, latestWorkflowID)
	}
}

// TestDeploymentApplyProgressRepository_FindLatestByDeploymentID_最新workflowの9ステップを返す は最新workflowの全ステップが取得できることを確認する
func TestDeploymentApplyProgressRepository_FindLatestByDeploymentID_最新workflowの9ステップを返す(t *testing.T) {
	db := setupTestDB(t)
	projectData := createTestProject(t, db)

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-progress-latest-all",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	repo := NewDeploymentApplyProgressRepository(db)
	workflowID := "apply-" + deploymentData.ID
	t.Cleanup(func() { db.Unscoped().Where("workflow_id = ?", workflowID).Delete(&models.DeploymentApplyProgress{}) })

	if err := repo.InitializeSteps(context.Background(), nil, workflowID, deploymentData.ID, map[models.ApplyProgressStepName]bool{}); err != nil {
		t.Fatalf("InitializeSteps がエラーを返しました: %v", err)
	}

	progressList, err := repo.FindLatestByDeploymentID(context.Background(), deploymentData.ID) // 最新workflowの全ステップを取得する
	if err != nil {
		t.Fatalf("FindLatestByDeploymentID がエラーを返しました: %v", err)
	}
	if len(progressList) != 9 { // 9件返ることを確認する
		t.Fatalf("期待する件数: 9, 実際の件数: %d", len(progressList))
	}
	for stepIndex, progressItem := range progressList { // step_no昇順で並んでいることを確認する
		if progressItem.StepNo != stepIndex+1 {
			t.Errorf("期待するstep_no: %d, 実際のstep_no: %d", stepIndex+1, progressItem.StepNo)
		}
	}
}

// TestDeploymentApplyProgressRepository_FindLatestByDeploymentID_レコードなしはErrRecordNotFound は進捗が存在しない場合にエラーが返ることを確認する
func TestDeploymentApplyProgressRepository_FindLatestByDeploymentID_レコードなしはErrRecordNotFound(t *testing.T) {
	db := setupTestDB(t)
	projectData := createTestProject(t, db)

	deploymentData := &models.Deployment{
		ProjectID: projectData.ID,
		Name:      "test-progress-none",
		Type:      models.DeploymentTypeImageURL,
		Status:    models.DeploymentStatusPending,
		AppStatus: models.AppStatusPending,
	}
	db.Create(deploymentData)
	t.Cleanup(func() { db.Unscoped().Delete(deploymentData) })

	repo := NewDeploymentApplyProgressRepository(db)

	_, err := repo.FindLatestByDeploymentID(context.Background(), deploymentData.ID) // 進捗が存在しない状態で取得する
	if err != gorm.ErrRecordNotFound {                                               // レコードなしエラーが返ることを確認する
		t.Errorf("期待するエラー: %v, 実際のエラー: %v", gorm.ErrRecordNotFound, err)
	}
}
