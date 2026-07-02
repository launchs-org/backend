package workflow

import (
	"builder/activity"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// CancelBuildWorkflowTestSuite は CancelBuildWorkflow のテストスイート
type CancelBuildWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	testEnv *testsuite.TestWorkflowEnvironment // テスト用 Temporal 環境
}

func TestCancelBuildWorkflowSuite(t *testing.T) {
	suite.Run(t, new(CancelBuildWorkflowTestSuite)) // テストスイートを実行する
}

// SetupTest は各テストの前にテスト環境を初期化する
func (testSuite *CancelBuildWorkflowTestSuite) SetupTest() {
	testSuite.testEnv = testSuite.NewTestWorkflowEnvironment() // テスト環境を生成する
	// 文字列名で OnActivity をモックするには事前に Activity を登録する必要がある
	testSuite.testEnv.RegisterActivity(&activity.CancelBuildActivities{}) // CancelBuild Activity を登録する
}

// AfterTest は各テスト終了後にアサーションを検証する
func (testSuite *CancelBuildWorkflowTestSuite) AfterTest(suiteName, testName string) {
	testSuite.testEnv.AssertExpectations(testSuite.T()) // モックの期待値を検証する
}

// TestCancelBuildWorkflow_正常に2Activity連鎖が実行される は DeleteBuildJob → SetBuildCancelled の順で実行されることを確認する
func (testSuite *CancelBuildWorkflowTestSuite) TestCancelBuildWorkflow_正常に2Activity連鎖が実行される() {
	input := activity.CancelBuildWorkflowInput{BuildID: "build-cancel-1"} // テスト用入力を定義する

	testSuite.testEnv.OnActivity("DeleteBuildJobActivity", mock.Anything, input).Return(nil)   // 1. Job 削除
	testSuite.testEnv.OnActivity("SetBuildCancelledActivity", mock.Anything, input).Return(nil) // 2. cancelled 更新

	testSuite.testEnv.ExecuteWorkflow(CancelBuildWorkflow, input)

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError()) // エラーがないことを確認する
}

// TestCancelBuildWorkflow_DeleteJobエラー時はWorkflowがエラーを返す は Job 削除エラー時 Workflow もエラーを返すことを確認する
func (testSuite *CancelBuildWorkflowTestSuite) TestCancelBuildWorkflow_DeleteJobエラー時はWorkflowがエラーを返す() {
	input := activity.CancelBuildWorkflowInput{BuildID: "build-cancel-err"} // テスト用入力を定義する

	testSuite.testEnv.OnActivity("DeleteBuildJobActivity", mock.Anything, input).
		Return(fmt.Errorf("k8s job delete failed")) // エラーを返す

	testSuite.testEnv.ExecuteWorkflow(CancelBuildWorkflow, input)

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了（エラーで）したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}
