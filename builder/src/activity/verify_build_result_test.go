package activity

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func verifyTestJob(buildID string, status batchv1.JobStatus) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "railpack-" + buildID, Namespace: buildkitNamespace}, Status: status}
}

// TestVerifyBuildResultActivity_Jobが失敗ならエラー は、ビルド Job が失敗(制限時間超過など)したとき、成功として扱わずにエラーを返すことを確認する。
func TestVerifyBuildResultActivity_Jobが失敗ならエラー(t *testing.T) {
	job := verifyTestJob("build-1", batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "DeadlineExceeded", Message: "Job was active longer than specified deadline"},
	}})
	activities := &BuildActivities{K8sClient: k8sfake.NewSimpleClientset(job)}

	err := activities.VerifyBuildResultActivity(context.Background(), BuildWorkflowInput{BuildID: "build-1"})

	if err == nil || !strings.Contains(err.Error(), "DeadlineExceeded") {
		t.Fatalf("DeadlineExceeded を含むエラーを期待しましたが、実際: %v", err)
	}
}

func TestVerifyBuildResultActivity_Jobが成功ならnil(t *testing.T) {
	job := verifyTestJob("build-2", batchv1.JobStatus{Succeeded: 1, Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}})
	activities := &BuildActivities{K8sClient: k8sfake.NewSimpleClientset(job)}

	if err := activities.VerifyBuildResultActivity(context.Background(), BuildWorkflowInput{BuildID: "build-2"}); err != nil {
		t.Fatalf("成功のとき nil を期待しましたが、実際: %v", err)
	}
}

func TestVerifyBuildResultActivity_Jobが存在しなければエラー(t *testing.T) {
	activities := &BuildActivities{K8sClient: k8sfake.NewSimpleClientset()}

	if err := activities.VerifyBuildResultActivity(context.Background(), BuildWorkflowInput{BuildID: "gone"}); err == nil {
		t.Fatal("Job が存在しない(削除・キャンセルされた)場合は、エラーを返すことを期待しました")
	}
}
