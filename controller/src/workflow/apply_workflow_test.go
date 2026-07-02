package workflow

import (
	"controller/activity"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// ApplyWorkflowTestSuite は ApplyWorkflow のテストスイート
type ApplyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	testEnv *testsuite.TestWorkflowEnvironment // テスト用 Temporal 環境
}

func TestApplyWorkflowSuite(t *testing.T) {
	suite.Run(t, new(ApplyWorkflowTestSuite)) // テストスイートを実行する
}

// SetupTest は各テストの前にテスト環境を初期化する
func (testSuite *ApplyWorkflowTestSuite) SetupTest() {
	testSuite.testEnv = testSuite.NewTestWorkflowEnvironment() // テスト環境を生成する
}

// AfterTest は各テスト終了後にアサーションを検証する
func (testSuite *ApplyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	testSuite.testEnv.AssertExpectations(testSuite.T()) // モックの期待値を検証する
}

// TestApplyWorkflow_正常にActivity連鎖が実行される は ApplyWorkflow が正しい順序で Activity を実行することを確認する
func (testSuite *ApplyWorkflowTestSuite) TestApplyWorkflow_正常にActivity連鎖が実行される() {
	applyResult := &activity.ApplyResultData{
		ApplyHistoryID: "history-1", // テスト用 apply_history ID を設定する
	}

	// ExecuteApply Activity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ApplyActivities).ExecuteApply, mock.Anything, mock.Anything, mock.Anything).
		Return(applyResult, nil) // 成功を返す

	// ApplyIngressRoutes Activity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ApplyActivities).ApplyIngressRoutes, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// Workflow を実行する
	testSuite.testEnv.ExecuteWorkflow(ApplyWorkflow, ApplyWorkflowInput{
		DeploymentID: "deployment-1",
		BaseDomain:   "example.com",
		ProjectID:    "project-1",
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted())                // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError())                // エラーがないことを確認する
}

// TestApplyWorkflow_ProjectIDなしはIngressRoutesActivityをスキップする は ProjectID が空の場合 ApplyIngressRoutes をスキップすることを確認する
func (testSuite *ApplyWorkflowTestSuite) TestApplyWorkflow_ProjectIDなしはIngressRoutesActivityをスキップする() {
	applyResult := &activity.ApplyResultData{
		ApplyHistoryID: "history-2", // テスト用 apply_history ID を設定する
	}

	// ExecuteApply Activity のみモックする（IngressRoutes はスキップされるはず）（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ApplyActivities).ExecuteApply, mock.Anything, mock.Anything, mock.Anything).
		Return(applyResult, nil) // 成功を返す

	// Workflow を ProjectID なしで実行する
	testSuite.testEnv.ExecuteWorkflow(ApplyWorkflow, ApplyWorkflowInput{
		DeploymentID: "deployment-2",
		BaseDomain:   "example.com",
		ProjectID:    "", // ProjectID が空なので IngressRoutes はスキップされる
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted())  // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError())  // エラーがないことを確認する
}

// TestApplyWorkflow_ExecuteApplyエラー時はWorkflowがエラーを返す は ExecuteApply Activity がエラーを返した場合 Workflow もエラーを返すことを確認する
func (testSuite *ApplyWorkflowTestSuite) TestApplyWorkflow_ExecuteApplyエラー時はWorkflowがエラーを返す() {
	// ExecuteApply Activity がエラーを返すようにモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ApplyActivities).ExecuteApply, mock.Anything, mock.Anything, mock.Anything).
		Return((*activity.ApplyResultData)(nil), fmt.Errorf("k8s apply failed")) // エラーを返す

	testSuite.testEnv.ExecuteWorkflow(ApplyWorkflow, ApplyWorkflowInput{
		DeploymentID: "deployment-3",
		BaseDomain:   "example.com",
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了（エラーで）したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}
