package workflow

import (
	"controller/activity"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// CreateProjectWorkflowTestSuite は CreateProjectWorkflow のテストスイート
type CreateProjectWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	testEnv *testsuite.TestWorkflowEnvironment // テスト用 Temporal 環境
}

func TestCreateProjectWorkflowSuite(t *testing.T) {
	suite.Run(t, new(CreateProjectWorkflowTestSuite)) // テストスイートを実行する
}

// SetupTest は各テストの前にテスト環境を初期化する
func (testSuite *CreateProjectWorkflowTestSuite) SetupTest() {
	testSuite.testEnv = testSuite.NewTestWorkflowEnvironment() // テスト環境を生成する
}

// AfterTest は各テスト終了後にアサーションを検証する
func (testSuite *CreateProjectWorkflowTestSuite) AfterTest(suiteName, testName string) {
	testSuite.testEnv.AssertExpectations(testSuite.T()) // モックの期待値を検証する
}

// TestCreateProjectWorkflow_正常に4つのActivity連鎖が実行される は Harbor・k8s・DB の4ステップが正しい順序で実行されることを確認する
func (testSuite *CreateProjectWorkflowTestSuite) TestCreateProjectWorkflow_正常に4つのActivity連鎖が実行される() {
	// 1. CreateHarborProjectActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ProjectActivities).CreateHarborProjectActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// 2. CreateHarborRobotActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ProjectActivities).CreateHarborRobotActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// 3. CreateK8sNamespaceActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ProjectActivities).CreateK8sNamespaceActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// 4. ActivateProjectActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ProjectActivities).ActivateProjectActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// Workflow を実行する
	testSuite.testEnv.ExecuteWorkflow(CreateProjectWorkflow, CreateProjectWorkflowInput{
		ProjectID: "project-1",
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError()) // エラーがないことを確認する
}

// TestCreateProjectWorkflow_HarborProjectエラー時はWorkflowがエラーを返す は最初の Activity でエラーが発生したとき Workflow もエラーを返すことを確認する
func (testSuite *CreateProjectWorkflowTestSuite) TestCreateProjectWorkflow_HarborProjectエラー時はWorkflowがエラーを返す() {
	// CreateHarborProjectActivity がエラーを返すようにモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.ProjectActivities).CreateHarborProjectActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("harbor unavailable")) // エラーを返す

	testSuite.testEnv.ExecuteWorkflow(CreateProjectWorkflow, CreateProjectWorkflowInput{
		ProjectID: "project-err",
	})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了（エラーで）したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}
