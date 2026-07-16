package railpack

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateJobUsesArchiveFetchContainerForArchiveSource(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset() // fake clientsetを生成する

	config := applyDefaults(BuildConfig{
		SourceType:       "archive",
		ArchiveURL:       "https://file.io/xxxx",
		ArchiveEncKeyHex: "abcd1234",
		ArchiveSHA256Hex: "ef567890",
		RegistryUsername: "robot-user",
		Namespace:        "buildkit",
		ImageName:        "my-app",
		ImageTag:         "v1.0.0",
		JobID:            "test-job-archive",
		Timeout:          10 * time.Minute,
	})

	jobID, err := createJob(context.Background(), fakeClientset, "buildkit", config) // Jobを作成する
	if err != nil {
		t.Fatalf("createJobが失敗しました: %v", err)
	}

	createdJob, err := fakeClientset.BatchV1().Jobs("buildkit").Get(context.Background(), "railpack-"+jobID, metav1.GetOptions{}) // 作成されたJobを取得する
	if err != nil {
		t.Fatalf("作成されたJobの取得に失敗しました: %v", err)
	}

	containerNames := make(map[string]bool)
	for _, initContainer := range createdJob.Spec.Template.Spec.InitContainers {
		containerNames[initContainer.Name] = true
	}

	if !containerNames["archive-fetch"] {
		t.Errorf("archive-fetchコンテナがInitContainersに含まれていません")
	}
	if containerNames["git-clone"] {
		t.Errorf("archiveソースなのにgit-cloneコンテナが含まれています")
	}
}

func TestCreateJobUsesGitCloneContainerForGitSource(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()

	config := applyDefaults(BuildConfig{
		GitRepo:          "https://github.com/org/repo",
		RegistryUsername: "robot-user",
		Namespace:        "buildkit",
		ImageName:        "my-app",
		ImageTag:         "v1.0.0",
		JobID:            "test-job-git",
		Timeout:          10 * time.Minute,
	})

	jobID, err := createJob(context.Background(), fakeClientset, "buildkit", config)
	if err != nil {
		t.Fatalf("createJobが失敗しました: %v", err)
	}

	createdJob, err := fakeClientset.BatchV1().Jobs("buildkit").Get(context.Background(), "railpack-"+jobID, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("作成されたJobの取得に失敗しました: %v", err)
	}

	containerNames := make(map[string]bool)
	for _, initContainer := range createdJob.Spec.Template.Spec.InitContainers {
		containerNames[initContainer.Name] = true
	}

	if !containerNames["git-clone"] { // 既存gitケースの回帰がないことを確認する
		t.Errorf("git-cloneコンテナがInitContainersに含まれていません")
	}
	if containerNames["archive-fetch"] {
		t.Errorf("gitソースなのにarchive-fetchコンテナが含まれています")
	}
}
