package workflow

import (
	"builder/activity"
	"testing"

	"app/shared/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/temporal"
)

// BuildWorkflowTestSuite は BuildWorkflow のテストスイート
type BuildWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	testEnv *testsuite.TestWorkflowEnvironment // テスト用 Temporal 環境
}

func TestBuildWorkflowSuite(t *testing.T) {
	suite.Run(t, new(BuildWorkflowTestSuite)) // テストスイートを実行する
}

// SetupTest は各テストの前にテスト環境を初期化する
func (testSuite *BuildWorkflowTestSuite) SetupTest() {
	testSuite.testEnv = testSuite.NewTestWorkflowEnvironment() // テスト環境を生成する
	// 文字列名で OnActivity をモックするには事前に Activity を登録する必要がある
	testSuite.testEnv.RegisterActivity(&activity.BuildActivities{}) // Activity を登録する
}

// AfterTest は各テスト終了後にアサーションを検証する
func (testSuite *BuildWorkflowTestSuite) AfterTest(suiteName, testName string) {
	testSuite.testEnv.AssertExpectations(testSuite.T()) // モックの期待値を検証する
}

// TestBuildWorkflow_正常にActivity連鎖が実行される は全5 Activity が正しい順序で実行されることを確認する
func (testSuite *BuildWorkflowTestSuite) TestBuildWorkflow_正常にActivity連鎖が実行される() {
	input := activity.BuildWorkflowInput{BuildID: "build-1"} // テスト用入力を定義する

	// builder workflow は Activity 名を文字列で登録するため文字列でモックする
	testSuite.testEnv.OnActivity("VerifyHarborCredentialActivity", mock.Anything, input).Return(nil)          // 1. Harbor 認証確認
	testSuite.testEnv.OnActivity("CreateBuildJobActivity", mock.Anything, input).Return(nil)                  // 2. Job 作成
	testSuite.testEnv.OnActivity("StreamBuildLogsActivity", mock.Anything, input).Return(nil)                 // 3. ログストリーム
	testSuite.testEnv.OnActivity("VerifyBuildResultActivity", mock.Anything, input).Return(nil)               // 3.5. ビルド Job の結果確認
	testSuite.testEnv.OnActivity("SetPendingImageActivity", mock.Anything, input).Return("image-1", nil) // 4. pending_image_id 更新（イメージ ID を返す）
	testSuite.testEnv.OnActivity("UpdateBuildStatusActivity", mock.Anything, input, models.BuildStatusSucceeded, "image-1").Return(nil) // 5. succeeded 更新（imageID を渡す）

	testSuite.testEnv.ExecuteWorkflow(BuildWorkflow, input)

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.NoError(testSuite.testEnv.GetWorkflowError()) // エラーがないことを確認する
}

// TestBuildWorkflow_VerifyHarborエラー時はfailedステータスになる は Harbor 認証失敗時にビルドステータスが failed になることを確認する
func (testSuite *BuildWorkflowTestSuite) TestBuildWorkflow_VerifyHarborエラー時はfailedステータスになる() {
	input := activity.BuildWorkflowInput{BuildID: "build-err"} // テスト用入力を定義する

	// 1. VerifyHarborCredentialActivity がエラーを返す
	testSuite.testEnv.OnActivity("VerifyHarborCredentialActivity", mock.Anything, input).
		Return(temporal.NewApplicationError("harbor error", "HarborError")) // エラーを返す

	// failed 更新のために UpdateBuildStatusActivity が呼ばれることを期待する（builtImageURL は空文字）
	testSuite.testEnv.OnActivity("UpdateBuildStatusActivity", mock.Anything, input, models.BuildStatusFailed, "").Return(nil)

	testSuite.testEnv.ExecuteWorkflow(BuildWorkflow, input)

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}

// TestBuildWorkflow_StreamBuildLogsエラー時はfailedステータスになる は ログストリーム失敗時にビルドステータスが failed になることを確認する
func (testSuite *BuildWorkflowTestSuite) TestBuildWorkflow_StreamBuildLogsエラー時はfailedステータスになる() {
	input := activity.BuildWorkflowInput{BuildID: "build-log-err"} // テスト用入力を定義する

	testSuite.testEnv.OnActivity("VerifyHarborCredentialActivity", mock.Anything, input).Return(nil)
	testSuite.testEnv.OnActivity("CreateBuildJobActivity", mock.Anything, input).Return(nil)
	testSuite.testEnv.OnActivity("StreamBuildLogsActivity", mock.Anything, input).
		Return(temporal.NewApplicationError("log stream failed", "StreamError")) // ログストリームがエラーを返す
	testSuite.testEnv.OnActivity("UpdateBuildStatusActivity", mock.Anything, input, models.BuildStatusFailed, "").Return(nil) // builtImageURL は空文字

	testSuite.testEnv.ExecuteWorkflow(BuildWorkflow, input)

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted()) // Workflow が完了したことを確認する
	testSuite.Error(testSuite.testEnv.GetWorkflowError())   // エラーが返ることを確認する
}

// TestBuildWorkflow_ビルドJobが失敗ならfailedステータスになりデプロイに進まない は、ログの転送が成功しても、
// ビルド Job が失敗(制限時間の超過など)していれば、ビルドを failed にし、成功の処理(pending_image 更新、succeeded)に進まないことを確認する。
func (testSuite *BuildWorkflowTestSuite) TestBuildWorkflow_ビルドJobが失敗ならfailedステータスになりデプロイに進まない() {
	input := activity.BuildWorkflowInput{BuildID: "build-job-failed"}

	testSuite.testEnv.OnActivity("VerifyHarborCredentialActivity", mock.Anything, input).Return(nil)
	testSuite.testEnv.OnActivity("CreateBuildJobActivity", mock.Anything, input).Return(nil)
	testSuite.testEnv.OnActivity("StreamBuildLogsActivity", mock.Anything, input).Return(nil) // ログの転送は成功する
	testSuite.testEnv.OnActivity("VerifyBuildResultActivity", mock.Anything, input).
		Return(temporal.NewNonRetryableApplicationError("ビルド Job が失敗しました: DeadlineExceeded", "BuildJobFailed", nil))
	testSuite.testEnv.OnActivity("UpdateBuildStatusActivity", mock.Anything, input, models.BuildStatusFailed, "").Return(nil) // failed に更新される
	// SetPendingImageActivity と、succeeded への更新は呼ばれない(モックを登録しないので、呼ばれればテストが失敗する)

	testSuite.testEnv.ExecuteWorkflow(BuildWorkflow, input)

	testSuite.True(testSuite.testEnv.IsWorkflowCompleted())
	testSuite.Error(testSuite.testEnv.GetWorkflowError())
}
