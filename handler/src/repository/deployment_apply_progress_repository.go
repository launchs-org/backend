package repository

import (
	"context"
	"handler/models"
	"time"

	"gorm.io/gorm"
)

// DeploymentApplyProgressRepository は deployment_apply_progress テーブルへのアクセスを定義するインターフェース
type DeploymentApplyProgressRepository interface {
	InitializeSteps(ctx context.Context, tx *gorm.DB, workflowID string, deploymentID string, skippedSteps map[models.ApplyProgressStepName]bool) error                            // workflowID に対応する9ステップを冪等に初期化する
	UpdateStepStatus(ctx context.Context, tx *gorm.DB, workflowID string, stepName models.ApplyProgressStepName, status models.ApplyProgressStepStatus, errorMessage string) error // 指定ステップの状態を更新する
	FindAllByWorkflowID(ctx context.Context, workflowID string) ([]*models.DeploymentApplyProgress, error)                                                                         // workflowID に紐づく全ステップを step_no 昇順で取得する
	FindLatestWorkflowIDByDeploymentID(ctx context.Context, deploymentID string) (string, error)                                                                                   // deploymentID の最新workflow_idを取得する
	FindLatestByDeploymentID(ctx context.Context, deploymentID string) ([]*models.DeploymentApplyProgress, error)                                                                  // deploymentID に紐づく最新workflowの全ステップを取得する
}

// deploymentApplyProgressRepositoryImpl は DeploymentApplyProgressRepository の GORM 実装
type deploymentApplyProgressRepositoryImpl struct {
	db *gorm.DB // データベース接続
}

// NewDeploymentApplyProgressRepository は DeploymentApplyProgressRepository の実装を返す
func NewDeploymentApplyProgressRepository(db *gorm.DB) DeploymentApplyProgressRepository {
	return &deploymentApplyProgressRepositoryImpl{db: db} // 実装を生成して返す
}

// resolveExecutor は tx が指定されていれば tx を、なければ db を使う（controller/watcher 両方から呼ばれるための共通ヘルパー）
func (repo *deploymentApplyProgressRepositoryImpl) resolveExecutor(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil { // トランザクション内呼び出しの場合は tx を使う
		return tx.WithContext(ctx)
	}
	return repo.db.WithContext(ctx) // トランザクション外の場合は db を直接使う
}

// InitializeSteps は workflowID に対して9ステップ分のレコードを冪等に初期化する
// 既にレコードが存在する場合（Activity リトライ時）は該当ステップを pending/skipped にリセットしてから使う
func (repo *deploymentApplyProgressRepositoryImpl) InitializeSteps(ctx context.Context, tx *gorm.DB, workflowID string, deploymentID string, skippedSteps map[models.ApplyProgressStepName]bool) error {
	executor := repo.resolveExecutor(ctx, tx) // 実行対象の DB ハンドルを決定する

	for stepIndex, stepName := range models.ApplyProgressStepOrder { // 定義順にステップを初期化する
		initialStatus := models.ApplyProgressStepStatusPending // デフォルトは pending とする
		if skippedSteps[stepName] {                            // 対象リソースがないステップは skipped にする
			initialStatus = models.ApplyProgressStepStatusSkipped
		}

		var existing models.DeploymentApplyProgress // 既存レコードを格納する変数を定義する
		findErr := executor.Where("workflow_id = ? AND step_name = ?", workflowID, stepName).First(&existing).Error
		if findErr == nil { // 既存レコードがある場合はリセットする（Activity リトライ時）
			resetErr := executor.Model(&models.DeploymentApplyProgress{}).
				Where("id = ?", existing.ID).
				Updates(map[string]interface{}{
					"status":        initialStatus,
					"error_message": "",
					"started_at":    nil,
					"finished_at":   nil,
				}).Error
			if resetErr != nil {
				return resetErr // 更新エラーを返す
			}
			continue // 次のステップへ進む
		}
		if findErr != gorm.ErrRecordNotFound { // レコードなし以外のエラーは異常なので返す
			return findErr
		}

		progressRecord := &models.DeploymentApplyProgress{ // 新規レコードを生成する
			WorkflowID:   workflowID,
			DeploymentID: deploymentID,
			StepNo:       stepIndex + 1,
			StepName:     stepName,
			Status:       initialStatus,
		}
		if createErr := executor.Create(progressRecord).Error; createErr != nil { // レコードを作成する
			return createErr // 作成エラーを返す
		}
	}
	return nil // 正常終了
}

// UpdateStepStatus は指定ステップの状態を更新する（in_progress時はstartedAtを、終端状態時はfinishedAtを設定する）
func (repo *deploymentApplyProgressRepositoryImpl) UpdateStepStatus(ctx context.Context, tx *gorm.DB, workflowID string, stepName models.ApplyProgressStepName, status models.ApplyProgressStepStatus, errorMessage string) error {
	nowTime := time.Now() // 現在時刻を取得する
	updates := map[string]interface{}{
		"status":        status,       // ステータスを更新する
		"error_message": errorMessage, // エラーメッセージを更新する
	}
	if status == models.ApplyProgressStepStatusInProgress { // 実行中に遷移する場合は開始時刻を記録する
		updates["started_at"] = &nowTime
	}
	if status == models.ApplyProgressStepStatusDone || status == models.ApplyProgressStepStatusFailed || status == models.ApplyProgressStepStatusSkipped { // 終端状態に遷移する場合は終了時刻を記録する
		updates["finished_at"] = &nowTime
	}
	return repo.resolveExecutor(ctx, tx).Model(&models.DeploymentApplyProgress{}).
		Where("workflow_id = ? AND step_name = ?", workflowID, stepName).
		Updates(updates).Error // ステップの状態を更新する
}

// FindAllByWorkflowID は workflowID に紐づく9ステップを step_no 昇順で取得する
func (repo *deploymentApplyProgressRepositoryImpl) FindAllByWorkflowID(ctx context.Context, workflowID string) ([]*models.DeploymentApplyProgress, error) {
	var progressList []*models.DeploymentApplyProgress // 結果を格納するスライスを定義する
	err := repo.db.WithContext(ctx).Where("workflow_id = ?", workflowID).Order("step_no ASC").Find(&progressList).Error
	return progressList, err // 結果とエラーを返す
}

// FindLatestWorkflowIDByDeploymentID は deploymentID に対応する最新の workflow_id を取得する
func (repo *deploymentApplyProgressRepositoryImpl) FindLatestWorkflowIDByDeploymentID(ctx context.Context, deploymentID string) (string, error) {
	var latestRecord models.DeploymentApplyProgress // 最新レコードを格納する変数を定義する
	err := repo.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("created_at DESC").
		First(&latestRecord).Error
	if err != nil {
		return "", err // 取得エラーを返す
	}
	return latestRecord.WorkflowID, nil // 最新の workflow_id を返す
}

// FindLatestByDeploymentID は deploymentID に紐づく最新workflowの9ステップを step_no 昇順で取得する
func (repo *deploymentApplyProgressRepositoryImpl) FindLatestByDeploymentID(ctx context.Context, deploymentID string) ([]*models.DeploymentApplyProgress, error) {
	latestWorkflowID, err := repo.FindLatestWorkflowIDByDeploymentID(ctx, deploymentID) // 最新の workflow_id を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return repo.FindAllByWorkflowID(ctx, latestWorkflowID) // 最新workflowの全ステップを取得する
}
