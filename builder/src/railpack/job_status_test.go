package railpack

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func newJobWithStatus(jobID string, status batchv1.JobStatus) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "railpack-" + jobID, Namespace: "buildkit"},
		Status:     status,
	}
}

// TestJobStatus_DeadlineExceededはfailedカウントが0でも失敗と判定する は、制限時間超過で Job が失敗したとき、
// status.failed が 0 のままでも、Failed の condition から失敗と判定できることを確認する(これが漏れると、失敗が成功として記録される)。
func TestJobStatus_DeadlineExceededはfailedカウントが0でも失敗と判定する(t *testing.T) {
	job := newJobWithStatus("b1", batchv1.JobStatus{
		Failed: 0,
		Conditions: []batchv1.JobCondition{
			{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "DeadlineExceeded"},
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "DeadlineExceeded", Message: "Job was active longer than specified deadline"},
		},
	})
	clientset := k8sfake.NewSimpleClientset(job)

	status, reason, err := JobStatus(context.Background(), clientset, "buildkit", "b1")

	if err != nil || status != StatusFailed {
		t.Fatalf("失敗と判定されることを期待しましたが、実際: status=%s err=%v", status, err)
	}
	if !strings.Contains(reason, "DeadlineExceeded") {
		t.Errorf("失敗の理由に DeadlineExceeded が含まれることを期待しましたが、実際: %q", reason)
	}
}

func TestJobStatus_Completeのconditionは成功と判定する(t *testing.T) {
	job := newJobWithStatus("b2", batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}})
	status, _, err := JobStatus(context.Background(), k8sfake.NewSimpleClientset(job), "buildkit", "b2")
	if err != nil || status != StatusComplete {
		t.Fatalf("成功と判定されることを期待しましたが、実際: status=%s err=%v", status, err)
	}
}

func TestJobStatus_conditionがFalseの場合は無視する(t *testing.T) {
	job := newJobWithStatus("b3", batchv1.JobStatus{Active: 1, Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionFalse}}})
	status, _, _ := JobStatus(context.Background(), k8sfake.NewSimpleClientset(job), "buildkit", "b3")
	if status != StatusRunning {
		t.Fatalf("実行中と判定されることを期待しましたが、実際: %s", status)
	}
}

func TestJobStatus_カウントでも判定できる(t *testing.T) {
	cases := []struct {
		name   string
		status batchv1.JobStatus
		want   BuildStatus
	}{
		{"成功した Pod がある", batchv1.JobStatus{Succeeded: 1}, StatusComplete},
		{"失敗した Pod がある", batchv1.JobStatus{Failed: 1}, StatusFailed},
		{"実行中", batchv1.JobStatus{Active: 1}, StatusRunning},
		{"まだ何もない", batchv1.JobStatus{}, StatusInit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := JobStatus(context.Background(), k8sfake.NewSimpleClientset(newJobWithStatus("b4", c.status)), "buildkit", "b4")
			if err != nil || got != c.want {
				t.Errorf("期待 %s, 実際 %s (err=%v)", c.want, got, err)
			}
		})
	}
}

func TestJobStatus_Jobが存在しなければエラーを返す(t *testing.T) {
	_, _, err := JobStatus(context.Background(), k8sfake.NewSimpleClientset(), "buildkit", "nothing")
	if err == nil {
		t.Fatal("Job が存在しない場合は、エラーを返すことを期待しました")
	}
}
