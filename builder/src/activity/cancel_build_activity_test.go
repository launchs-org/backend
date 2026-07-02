package activity

import (
	"app/shared/models"
	"app/shared/repository"
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// mockDeploymentBuildRepository は DeploymentBuildRepository のテスト用モック
type mockDeploymentBuildRepository struct {
	findByIDFunc      func(ctx context.Context, buildID string) (*models.DeploymentBuild, error)
	updateStatusFunc  func(ctx context.Context, buildID string, status models.BuildStatus) error
	updateK8sJobNameFunc func(ctx context.Context, buildID string, jobName string) error
}

func (mock *mockDeploymentBuildRepository) Create(ctx context.Context, build *models.DeploymentBuild) error { return nil }
func (mock *mockDeploymentBuildRepository) FindByID(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
	if mock.findByIDFunc != nil {
		return mock.findByIDFunc(ctx, buildID)
	}
	return nil, nil
}
func (mock *mockDeploymentBuildRepository) FindAllByDeploymentID(ctx context.Context, deploymentID string) ([]models.DeploymentBuild, error) { return nil, nil }
func (mock *mockDeploymentBuildRepository) FindAllByProjectID(ctx context.Context, projectID string) ([]models.DeploymentBuild, error) { return nil, nil }
func (mock *mockDeploymentBuildRepository) FindAllBuilding(ctx context.Context) ([]models.DeploymentBuild, error) { return nil, nil }
func (mock *mockDeploymentBuildRepository) UpdateStatus(ctx context.Context, buildID string, status models.BuildStatus) error {
	if mock.updateStatusFunc != nil {
		return mock.updateStatusFunc(ctx, buildID, status)
	}
	return nil
}
func (mock *mockDeploymentBuildRepository) UpdateK8sJobName(ctx context.Context, buildID string, jobName string) error {
	if mock.updateK8sJobNameFunc != nil {
		return mock.updateK8sJobNameFunc(ctx, buildID, jobName)
	}
	return nil
}
func (mock *mockDeploymentBuildRepository) UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, builtImageURL string, imageSizeBytes int64, finishedAt time.Time) error { return nil }
func (mock *mockDeploymentBuildRepository) Delete(ctx context.Context, build *models.DeploymentBuild) error { return nil }
func (mock *mockDeploymentBuildRepository) DeleteAllByDeploymentID(ctx context.Context, deploymentID string) error { return nil }
func (mock *mockDeploymentBuildRepository) DeleteAllByProjectID(ctx context.Context, db *gorm.DB, projectID string) error { return nil }

// インターフェース実装を静的に確認する
var _ repository.DeploymentBuildRepository = (*mockDeploymentBuildRepository)(nil)

// TestSetBuildCancelledActivity_正常にステータスがcancelledになる はビルドステータスが cancelled に更新されることを確認する
func TestSetBuildCancelledActivity_正常にステータスがcancelledになる(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	var updatedStatus models.BuildStatus // 更新されたステータスを記録する変数を定義する

	buildRepo := &mockDeploymentBuildRepository{
		updateStatusFunc: func(ctx context.Context, buildID string, status models.BuildStatus) error {
			updatedStatus = status // ステータスを記録する
			return nil             // 成功を返す
		},
	}

	activities := &CancelBuildActivities{
		K8sClient: k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		BuildRepo: buildRepo,                    // モックリポジトリを注入する
	}

	err := activities.SetBuildCancelledActivity(ctx, CancelBuildWorkflowInput{BuildID: "build-1"}) // Activity を実行する
	if err != nil {
		t.Fatalf("SetBuildCancelledActivity がエラーを返しました: %v", err) // エラーが発生した場合はテスト失敗
	}
	if updatedStatus != models.BuildStatusCancelled { // ステータスが cancelled になったことを確認する
		t.Errorf("期待するステータス %s、実際のステータス %s", models.BuildStatusCancelled, updatedStatus)
	}
}

// TestSetBuildCancelledActivity_リポジトリエラー時はエラーを返す はリポジトリエラー時にエラーを返すことを確認する
func TestSetBuildCancelledActivity_リポジトリエラー時はエラーを返す(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildRepo := &mockDeploymentBuildRepository{
		updateStatusFunc: func(ctx context.Context, buildID string, status models.BuildStatus) error {
			return errors.New("db update failed") // エラーを返す
		},
	}

	activities := &CancelBuildActivities{
		K8sClient: k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		BuildRepo: buildRepo,                    // モックリポジトリを注入する
	}

	err := activities.SetBuildCancelledActivity(ctx, CancelBuildWorkflowInput{BuildID: "build-err"}) // Activity を実行する
	if err == nil { // エラーが返ることを確認する
		t.Fatal("エラーが返されるべきですが、nil が返りました")
	}
}

// TestDeleteBuildJobActivity_JobNameが空の場合はスキップする は K8sJobName が空の場合 Job 削除をスキップすることを確認する
func TestDeleteBuildJobActivity_JobNameが空の場合はスキップする(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する

	buildRepo := &mockDeploymentBuildRepository{
		findByIDFunc: func(ctx context.Context, buildID string) (*models.DeploymentBuild, error) {
			return &models.DeploymentBuild{
				ID:         buildID,
				K8sJobName: "", // Job 名が空（pending 状態で Job 未作成）
			}, nil
		},
	}

	activities := &CancelBuildActivities{
		K8sClient: k8sfake.NewSimpleClientset(), // fake k8s クライアントを生成する
		BuildRepo: buildRepo,                    // モックリポジトリを注入する
	}

	err := activities.DeleteBuildJobActivity(ctx, CancelBuildWorkflowInput{BuildID: "build-no-job"}) // Activity を実行する
	if err != nil { // エラーが返らないことを確認する
		t.Fatalf("K8sJobName が空の場合はエラーが返らないはずですが、エラーが返りました: %v", err)
	}
}
