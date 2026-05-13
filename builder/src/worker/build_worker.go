package worker

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	// "strings"
	"time"

	"builder/harbor"
	"builder/railpack"
	"launchs/shared/database"
	"launchs/shared/job_queue/jobs"
	"launchs/shared/model"

	"github.com/riverqueue/river"
	"k8s.io/client-go/kubernetes"
)

var _ river.Worker[jobs.BuildJobArgs] = (*BuildWorker)(nil)

type BuildWorker struct {
	river.WorkerDefaults[jobs.BuildJobArgs]
}

func (w *BuildWorker) Timeout(*river.Job[jobs.BuildJobArgs]) time.Duration {
	return 40 * time.Minute
}

func (w *BuildWorker) Work(ctx context.Context, job *river.Job[jobs.BuildJobArgs]) error {
	payload := job.Args

	fmt.Printf("[build-worker] processing job %d (build_job_id: %s)\n", job.ID, payload.BuildJobID)

	if err := processBuildTask(ctx, payload); err != nil {
		fmt.Printf("[build-worker] job %d failed: %v\n", job.ID, err)
		return err
	}
	return nil
}

func processBuildTask(ctx context.Context, payload jobs.BuildJobArgs) error {
	failJob := func(err error) error {
		model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
			"status":      "Failed",
			"finished_at": time.Now(),
		})
		model.UpdateContainerStatus(payload.ContainerID, "Failed")
		return err
	}

	model.UpdateContainerStatus(payload.ContainerID, "Building")
	model.UpdateBuildJobStatus(payload.BuildJobID, map[string]interface{}{
		"status":     "Running",
		"started_at": time.Now(),
		"image_id":   payload.ImageID,
	})

	// Harbor から projectID に対応する robot クレデンシャルを取得（なければ作成）
	cred, err := resolveHarborCredential(ctx, payload.ProjectID)
	if err != nil {
		return failJob(fmt.Errorf("Harbor クレデンシャルの取得に失敗: %w", err))
	}

	buildNamespace := os.Getenv("BUILD_NAMESPACE")
	if buildNamespace == "" {
		buildNamespace = "buildkit"
	}

	registryHost := os.Getenv("REGISTRY_HOST")
	if registryHost == "" {
		registryHost = "172.33.0.1"
	}

	clientset := database.K8sClientset.(*kubernetes.Clientset)

	client, err := railpack.New(clientset, railpack.BuildConfig{
		GitRepo:          payload.RepositoryURL,
		GitBranch:        payload.Branch,
		Subdir:           payload.Directory,
		ImageName:        payload.ContainerID,
		ImageTag:         payload.ImageID,
		RegistryHost:     registryHost,
		RegistryProject:  payload.ProjectID,
		RegistryUsername: cred.RobotName,
		RegistryPassword: cred.RobotSecret,
		RegistryInsecure: false,
		Namespace:        buildNamespace,
		JobID:            payload.BuildJobID,
		Timeout:          35 * time.Minute,
	})
	if err != nil {
		return failJob(fmt.Errorf("railpack クライアントの作成に失敗: %w", err))
	}

	jobID, err := client.Build(ctx)
	if err != nil {
		return failJob(fmt.Errorf("K8s ビルドジョブの作成に失敗: %w", err))
	}

	log.Printf("[railpack] ジョブを開始しました: %s", jobID)

	// ── ログをチャンネルで受け取って出力 ────────────────────
	logCh, errCh := client.StreamLogs(ctx, jobID)
	go func() {
		for line := range logCh {
			log.Println("[build]", line)
		}
		if err := <-errCh; err != nil {
			log.Printf("[railpack] ログストリームエラー: %v", err)
		}
	}()

	// ── 完了まで待機 ─────────────────────────────────────────
	status, err := client.Wait(ctx, jobID)
	if err != nil {
		return fmt.Errorf("ビルド待機中にエラー: %w", err)
	}

	if status == railpack.StatusComplete {
		log.Printf("✓ ビルド成功")
		return nil
	}

	fmt.Printf("[build-worker] K8s Job created (jobID: %s)\n", jobID)

	return errors.New("Failed To Build Container Status: ")
}

// resolveHarborCredential は projectID に対応する Harbor robot クレデンシャルを返します。
// DB に存在しない場合は Harbor 上にプロジェクトと robot を作成して DB に保存します。
func resolveHarborCredential(ctx context.Context, projectID string) (*model.HarborCredential, error) {
	cred, err := model.GetHarborCredentialByProjectID(projectID)
	if err != nil {
		return nil, err
	}
	if cred != nil {
		return cred, nil
	}

	harborURL := os.Getenv("HARBOR_URL")
	if harborURL == "" {
		harborURL = "https://172.33.0.1"
	}
	harborDecodeUser,err := base64.StdEncoding.DecodeString(os.Getenv("HARBOR_USERNAME"))

	// エラー処理
	if err != nil {
		log.Fatal(err)
	}
	harborUser := string(harborDecodeUser)
	
	// harborUser := "robot$launchs-org"
	harborPass := os.Getenv("HARBOR_PASSWORD")

	// パスワードとユーザー名を表示
	fmt.Printf("[build-worker] Harbor URL: %s\n", harborURL)
	fmt.Printf("[build-worker] Harbor User: %s\n", harborUser)
	fmt.Printf("[build-worker] Harbor Password: %s\n", harborPass)

	insecure := true //os.Getenv("REGISTRY_INSECURE") == "true"
	hc := harbor.NewClient(harbor.Config{
		BaseURL:  harborURL,
		Username: harborUser,
		Password: harborPass,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
			},
		},
	})

	if err := hc.CreateProject(ctx, projectID, true); err != nil {
		return nil, fmt.Errorf("Harbor プロジェクト作成失敗: %w", err)
	}

	robot, err := hc.CreateProjectRobot(ctx, projectID, "buildkit", -1)
	if err != nil {
		return nil, fmt.Errorf("Harbor robot 作成失敗: %w", err)
	}

	cred = &model.HarborCredential{
		ProjectID:     projectID,
		HarborProject: projectID,
		RobotID:       robot.ID,
		RobotName:     robot.Name,
		RobotSecret:   robot.Secret,
	}
	if err := model.SaveHarborCredential(cred); err != nil {
		return nil, fmt.Errorf("クレデンシャルの保存に失敗: %w", err)
	}

	fmt.Printf("[build-worker] Harbor robot '%s' を作成しました (project: %s)\n", robot.Name, projectID)
	return cred, nil
}
