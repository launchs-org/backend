package workflow

import (
	"controller/activity"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// VolumeWorkflowTestSuite は CreateVolumeWorkflow / DeleteVolumeWorkflow のテストスイート
type VolumeWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	testEnv *testsuite.TestWorkflowEnvironment // テスト用 Temporal 環境
}

func TestVolumeWorkflowSuite(t *testing.T) {
	suite.Run(t, new(VolumeWorkflowTestSuite)) // テストスイートを実行する
}

// SetupTest は各テストの前にテスト環境を初期化する
func (testSuite *VolumeWorkflowTestSuite) SetupTest() {
	testSuite.testEnv = testSuite.NewTestWorkflowEnvironment() // テスト環境を生成する
}

// AfterTest は各テスト終了後にアサーションを検証する
func (testSuite *VolumeWorkflowTestSuite) AfterTest(suiteName, testName string) {
	testSuite.testEnv.AssertExpectations(testSuite.T()) // モックの期待値を検証する
}

// TestCreateVolumeWorkflow_正常にPVC作成Activityが実行される は CreateK8sPVCActivity が呼ばれることを確認する
func (testSuite *VolumeWorkflowTestSuite) TestCreateVolumeWorkflow_正常にPVC作成Activityが実行される() {
	// Temporal testsuite ではポインタレシーバーメソッドは receiver + ctx + input の順で引数をマッチさせる
	testSuite.testEnv.OnActivity((*activity.VolumeActivities).CreateK8sPVCActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	testSuite.testEnv.ExecuteWorkflow(CreateVolumeWorkflow, CreateVolumeWorkflowInput{VolumeID: "volume-1"})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError()) // エラーがないことを確認する
}

// TestCreateVolumeWorkflow_PVCエラー時はWorkflowがエラーを返す は PVC 作成エラー時 Workflow もエラーを返すことを確認する
func (testSuite *VolumeWorkflowTestSuite) TestCreateVolumeWorkflow_PVCエラー時はWorkflowがエラーを返す() {
	testSuite.testEnv.OnActivity((*activity.VolumeActivities).CreateK8sPVCActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("pvc creation failed")) // エラーを返す

	testSuite.testEnv.ExecuteWorkflow(CreateVolumeWorkflow, CreateVolumeWorkflowInput{VolumeID: "volume-err"})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了（エラーで）したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}

// TestDeleteVolumeWorkflow_正常に2つのActivity連鎖が実行される は PVC 削除後 DB 削除が実行されることを確認する
func (testSuite *VolumeWorkflowTestSuite) TestDeleteVolumeWorkflow_正常に2つのActivity連鎖が実行される() {
	// 1. DeleteK8sPVCActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.VolumeActivities).DeleteK8sPVCActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	// 2. DeleteVolumeRecordActivity をモックする（receiver + ctx + input）
	testSuite.testEnv.OnActivity((*activity.VolumeActivities).DeleteVolumeRecordActivity, mock.Anything, mock.Anything, mock.Anything).
		Return(nil) // 成功を返す

	testSuite.testEnv.ExecuteWorkflow(DeleteVolumeWorkflow, DeleteVolumeWorkflowInput{VolumeID: "volume-2"})

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError()) // エラーがないことを確認する
}
