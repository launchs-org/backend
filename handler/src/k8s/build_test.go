package k8s

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateBuildJob_ReturnsUnimplementedError(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する
	ctx := context.Background()             // テスト用コンテキストを生成する

	jobName, err := CreateBuildJob(ctx, fakeClient, "build-id-001", "test-namespace") // dockerfile ビルダーを呼び出す

	if err == nil { // エラーが返らない場合はテスト失敗
		t.Fatal("dockerfile ビルダーは未実装エラーを返すべきですが、エラーが返りませんでした")
	}
	if jobName != "" { // ジョブ名が空でない場合はテスト失敗
		t.Errorf("エラー時のジョブ名は空であるべきですが、%q が返りました", jobName)
	}
}

func TestDeleteBuildJob_DeletesJobSuccessfully(t *testing.T) {
	ctx := context.Background() // テスト用コンテキストを生成する
	namespace := "test-namespace"
	jobName := "build-abc123"

	existingJob := &batchv1.Job{ // テスト用 Job を定義する
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,   // Job 名を設定する
			Namespace: namespace, // namespace を設定する
		},
	}
	fakeClient := fake.NewSimpleClientset(existingJob) // フェイク k8s クライアントに Job を登録する

	err := DeleteBuildJob(ctx, fakeClient, namespace, jobName) // Job を削除する
	if err != nil {                                            // 削除エラーが発生した場合はテスト失敗
		t.Fatalf("DeleteBuildJob が予期しないエラーを返しました: %v", err)
	}

	// Job が削除されていることを確認する
	jobList, listErr := fakeClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if listErr != nil {
		t.Fatalf("Job 一覧の取得に失敗しました: %v", listErr)
	}
	if len(jobList.Items) != 0 { // Job が残っている場合はテスト失敗
		t.Errorf("Job が削除されるべきですが、%d 件残っています", len(jobList.Items))
	}
}

func TestDeleteBuildJob_ReturnsErrorForNonExistentJob(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // フェイク k8s クライアントを生成する（Job なし）
	ctx := context.Background()             // テスト用コンテキストを生成する

	err := DeleteBuildJob(ctx, fakeClient, "test-namespace", "nonexistent-job") // 存在しない Job を削除する
	if err == nil {                                                               // エラーが返らない場合はテスト失敗
		t.Fatal("存在しない Job の削除はエラーを返すべきですが、エラーが返りませんでした")
	}
}
