package workflow

import (
	"controller/activity"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// DeleteDeploymentWorkflowTestSuite は DeleteDeploymentWorkflow のテストスイート
type DeleteDeploymentWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	testEnv *testsuite.TestWorkflowEnvironment // テスト用 Temporal 環境
}

func TestDeleteDeploymentWorkflowSuite(t *testing.T) {
	suite.Run(t, new(DeleteDeploymentWorkflowTestSuite)) // テストスイートを実行する
}

// SetupTest は各テストの前にテスト環境を初期化する
func (testSuite *DeleteDeploymentWorkflowTestSuite) SetupTest() {
	testSuite.testEnv = testSuite.NewTestWorkflowEnvironment() // テスト環境を生成する
}

// AfterTest は各テスト終了後にアサーションを検証する
func (testSuite *DeleteDeploymentWorkflowTestSuite) AfterTest(suiteName, testName string) {
	testSuite.testEnv.AssertExpectations(testSuite.T()) // モックの期待値を検証する
}

// TestDeleteDeploymentWorkflow_正常にActivity連鎖が実行される は3つの Activity が正しい順序で実行されることを確認する
func (testSuite *DeleteDeploymentWorkflowTestSuite) TestDeleteDeploymentWorkflow_正常にActivity連鎖が実行される() {
	// 1. SetDeploymentDeletingActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.DeploymentActivities).SetDeploymentDeletingActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// 2. DeleteK8sDeploymentActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.DeploymentActivities).DeleteK8sDeploymentActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// 3. DeleteDeploymentRecordActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.DeploymentActivities).DeleteDeploymentRecordActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// Workflow を実行する
	testSuite.testEnv.ExecuteWorkflow(DeleteDeploymentWorkflow, DeleteDeploymentWorkflowInput{
		DeploymentID: "deployment-1",
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError()) // エラーがないことを確認する
}

// TestDeleteDeploymentWorkflow_SetDeletingエラー時はWorkflowがエラーを返す は最初の Activity でエラーが発生したとき Workflow もエラーを返すことを確認する
func (testSuite *DeleteDeploymentWorkflowTestSuite) TestDeleteDeploymentWorkflow_SetDeletingエラー時はWorkflowがエラーを返す() {
	// SetDeploymentDeletingActivity がエラーを返すようにモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.DeploymentActivities).SetDeploymentDeletingActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("db update failed")) // エラーを返す

	testSuite.testEnv.ExecuteWorkflow(DeleteDeploymentWorkflow, DeleteDeploymentWorkflowInput{
		DeploymentID: "deployment-err",
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了（エラーで）したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}
