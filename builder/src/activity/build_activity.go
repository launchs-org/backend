package activity

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"app/shared/logger"
	"app/shared/models"
	"app/shared/repository"
	"builder/railpack"

	"go.temporal.io/sdk/activity"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const buildkitNamespace = "buildkit" // ビルドジョブを作成する専用 namespace

// BuildActivities はビルド関連の Temporal Activity をまとめた構造体
type BuildActivities struct {
	K8sClient            kubernetes.Interface                   // k8s クライアント
	BuildRepo            repository.DeploymentBuildRepository  // build リポジトリ
	DeploymentRepo       repository.DeploymentRepository       // deployment リポジトリ
	ProjectRepo          repository.ProjectRepository          // project リポジトリ
	HarborCredentialRepo repository.HarborCredentialRepository // harbor credential リポジトリ
	LogChunkRepo         repository.BuildLogChunkRepository    // ビルドログチャンクリポジトリ
	RegistryHost         string                                // ビルドジョブが使う Harbor ホスト名（クラスタ内 DNS 名）
}

// BuildWorkflowInput は BuildWorkflow への入力
type BuildWorkflowInput struct {
	BuildID string // 対象ビルドの ID（あらかじめ DB に pending 状態で作成済み）
}

// VerifyHarborCredentialActivity は Harbor 認証情報の存在を確認する
func (act *BuildActivities) VerifyHarborCredentialActivity(ctx context.Context, input BuildWorkflowInput) error {
	buildData, err := act.BuildRepo.FindByID(ctx, input.BuildID) // ビルドレコードを取得する
	if err != nil {
		return fmt.Errorf("ビルドレコードの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	_, err = act.HarborCredentialRepo.FindByProjectIDNoTx(ctx, buildData.ProjectID) // Harbor 認証情報の存在を確認する
	if err != nil {
		return fmt.Errorf("Harbor 認証情報の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	return nil // 認証情報確認成功を返す
}

// CreateBuildJobActivity は K8s Job を作成してジョブ名を DB に保存する
func (act *BuildActivities) CreateBuildJobActivity(ctx context.Context, input BuildWorkflowInput) error {
	buildData, err := act.BuildRepo.FindByID(ctx, input.BuildID) // ビルドレコードを取得する
	if err != nil {
		return fmt.Errorf("ビルドレコードの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	projectData, err := act.ProjectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を取得する
	if err != nil {
		return fmt.Errorf("プロジェクトの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	harborCredential, err := act.HarborCredentialRepo.FindByProjectIDNoTx(ctx, buildData.ProjectID) // Harbor 認証情報を取得する
	if err != nil {
		return fmt.Errorf("Harbor 認証情報の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	// ビルドタイプに応じてイメージ名を設定する
	imageName := buildData.ID // デフォルトはビルド ID を使う
	if buildData.DeploymentID != nil {
		imageName = *buildData.DeploymentID // Deployment ID が存在する場合はそれを使う
	}

	railpackClient, err := railpack.New(act.K8sClient, railpack.BuildConfig{ // railpack クライアントを生成する
		JobID:            buildData.ID,        // ビルド ID をジョブ ID に使う
		GitRepo:          buildData.GithubRepoURL, // Git リポジトリ URL を設定する
		GitBranch:        buildData.Branch,    // ブランチを設定する
		Subdir:           buildData.Directory, // ビルドサブディレクトリを設定する
		ImageName:        imageName,           // イメージ名を設定する
		ImageTag:         buildData.ID,        // タグにビルド ID を使う
		RegistryHost:     act.RegistryHost,    // クラスタ内 DNS 名を使用する
		RegistryProject:  projectData.ID,      // Harbor プロジェクト名にプロジェクト ID を使う
		RegistryUsername: harborCredential.RobotName,   // robot アカウント名を設定する
		RegistryPassword: harborCredential.RobotSecret, // robot シークレットを設定する
		Namespace:        buildkitNamespace,   // ビルド専用 namespace を設定する
	})
	if err != nil {
		return fmt.Errorf("railpack クライアントの生成に失敗しました: %w", err) // 生成エラーを返す
	}

	jobID, err := railpackClient.Build(ctx) // ビルドジョブを起動する
	if err != nil {
		return fmt.Errorf("ビルドジョブの起動に失敗しました: %w", err) // 起動エラーを返す
	}

	jobName := "railpack-" + jobID                                                          // railpack の命名規則に合わせる
	if err := act.BuildRepo.UpdateK8sJobName(ctx, buildData.ID, jobName); err != nil { // Job 名を DB に保存する
		return fmt.Errorf("ジョブ名の保存に失敗しました: %w", err) // 保存エラーを返す
	}

	if err := act.BuildRepo.UpdateStatus(ctx, buildData.ID, models.BuildStatusBuilding); err != nil { // ステータスを building に更新する
		return fmt.Errorf("ステータスの更新に失敗しました: %w", err) // 更新エラーを返す
	}

	return nil // ジョブ作成成功を返す
}

// StreamBuildLogsActivity はビルドジョブのログをストリームして DB に保存する（Heartbeat 付き）
func (act *BuildActivities) StreamBuildLogsActivity(ctx context.Context, input BuildWorkflowInput) error {
	buildData, err := act.BuildRepo.FindByID(ctx, input.BuildID) // ビルドレコードを取得する
	if err != nil {
		return fmt.Errorf("ビルドレコードの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	logCh := collectJobLogs(ctx, act.K8sClient, buildkitNamespace, buildData.ID) // ビルドジョブは buildkit namespace に作成されるためそちらを使う

	ticker := time.NewTicker(3 * time.Second) // 3秒ごとにバッファをフラッシュするタイマーを生成する
	defer ticker.Stop()                       // 終了時にタイマーを停止する

	heartbeatTicker := time.NewTicker(10 * time.Second) // 10秒ごとに Heartbeat を送信するタイマーを生成する
	defer heartbeatTicker.Stop()                         // 終了時にタイマーを停止する

	var buf strings.Builder // ログバッファを生成する

	flush := func() { // バッファをフラッシュして DB に保存する関数
		if buf.Len() == 0 { // バッファが空の場合はスキップする
			return
		}
		chunk := &models.BuildLogChunk{ // ログチャンクレコードを生成する
			BuildID: buildData.ID, // ビルドIDを設定する
			Content: buf.String(), // バッファの内容を設定する
		}
		if saveErr := act.LogChunkRepo.Create(ctx, chunk); saveErr != nil { // DB に保存する
			logger.PrintErr("StreamBuildLogsActivity: ログチャンク保存に失敗しました（buildID=" + buildData.ID + "）: " + saveErr.Error()) // エラーをログ出力する
		}
		buf.Reset() // バッファをリセットする
	}

	for {
		select {
		case line, ok := <-logCh: // ログ行を受信する
			if !ok { // チャネルが閉じられた場合はフラッシュして終了する
				flush()
				return nil // ログ収集完了を返す
			}
			buf.WriteString(line + "\n") // ログ行をバッファに追記する
			if buf.Len() > 4096 {        // バッファが 4096 バイトを超えたら即時フラッシュする
				flush()
			}
		case <-ticker.C: // 3秒タイマーが発火したらフラッシュする
			flush()
		case <-heartbeatTicker.C: // 10秒タイマーが発火したら Heartbeat を送信する
			activity.RecordHeartbeat(ctx, "streaming logs for build "+buildData.ID) // Heartbeat を送信する
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return ctx.Err() // キャンセルエラーを返す
		}
	}
}

// SetPendingImageURLActivity はビルド成功時に pending_image_url と pending_github_* フィールドを更新し、組み立てたイメージ URL を返す
func (act *BuildActivities) SetPendingImageURLActivity(ctx context.Context, input BuildWorkflowInput) (string, error) {
	buildData, err := act.BuildRepo.FindByID(ctx, input.BuildID) // ビルドレコードを取得する
	if err != nil {
		return "", fmt.Errorf("ビルドレコードの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	if buildData.DeploymentID == nil { // DeploymentID が nil の場合（Deployment 削除済み）はスキップする
		return "", nil // スキップして正常終了を返す
	}

	projectData, err := act.ProjectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を取得する
	if err != nil {
		return "", fmt.Errorf("プロジェクトの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	imageName := *buildData.DeploymentID // イメージ名に Deployment ID を使う
	builtImageURL := fmt.Sprintf("%s/%s/%s:%s",
		act.RegistryHost, // クラスタ内 DNS 名を使用する
		projectData.ID,   // Harbor プロジェクト名にプロジェクト ID を使う
		imageName,        // イメージ名を設定する
		buildData.ID,     // ビルド ID をタグに使用する
	) // イメージURLを組み立てる

	deploymentIDValue := *buildData.DeploymentID                                                              // pointer をデリファレンスする
	if err := act.DeploymentRepo.UpdatePendingImageURL(ctx, deploymentIDValue, builtImageURL); err != nil { // pending_image_url を更新する
		return "", fmt.Errorf("pending_image_url の更新に失敗しました: %w", err) // 更新エラーを返す
	}

	deploymentData, err := act.DeploymentRepo.FindByID(ctx, deploymentIDValue) // pending_github_* 更新用に deployment を取得する
	if err != nil {
		return "", fmt.Errorf("Deployment の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	if err := act.DeploymentRepo.UpdatePendingGithubBuildFields(ctx, deploymentIDValue, deploymentData.GithubRepoURL, buildData.Branch, buildData.CommitSHA, buildData.Directory); err != nil { // pending_github_* フィールドを更新する
		return "", fmt.Errorf("pending_github_* フィールドの更新に失敗しました: %w", err) // 更新エラーを返す
	}

	return builtImageURL, nil // イメージ URL を返す
}

// UpdateBuildStatusActivity はビルド結果のステータスと Deployment の状態を更新する
// builtImageURL はビルド成功時のみ値が入り、失敗時は空文字を渡す
func (act *BuildActivities) UpdateBuildStatusActivity(ctx context.Context, input BuildWorkflowInput, buildStatus models.BuildStatus, builtImageURL string) error {
	buildData, err := act.BuildRepo.FindByID(ctx, input.BuildID) // ビルドレコードを取得する
	if err != nil {
		return fmt.Errorf("ビルドレコードの取得に失敗しました: %w", err) // 取得エラーを返す
	}

	finishedAt := time.Now()                                                                                              // 完了時刻を取得する
	if err := act.BuildRepo.UpdateBuildResult(ctx, input.BuildID, buildStatus, builtImageURL, 0, finishedAt); err != nil { // ビルド結果を更新する（成功時のみ builtImageURL に値が入る）
		return fmt.Errorf("ビルド結果の更新に失敗しました: %w", err) // 更新エラーを返す
	}

	if buildStatus == models.BuildStatusSucceeded && buildData.DeploymentID != nil { // 成功かつ Deployment が存在する場合に Deployment ステータスを更新する
		if err := act.DeploymentRepo.UpdateDeploymentStatus(ctx, *buildData.DeploymentID, models.DeploymentStatusPending); err != nil { // not_init → pending に遷移する
			return fmt.Errorf("Deployment ステータスの更新に失敗しました: %w", err) // 更新エラーを返す
		}
	}

	return nil // 更新成功を返す
}

// collectJobLogs は build-job-id ラベルに対応する Pod を取得し、全コンテナのログをチャンネルで返す
func collectJobLogs(ctx context.Context, k8sClient kubernetes.Interface, namespace, buildID string) <-chan string {
	logCh := make(chan string, 100) // ログ行を送るチャンネルを生成する

	go func() {
		defer close(logCh) // 終了時にチャンネルをクローズする

		pod, err := waitForJobPod(ctx, k8sClient, namespace, buildID) // Job に属する Pod が起動するまで待機する
		if err != nil {
			logger.PrintErr("collectJobLogs: Pod 取得に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}

		// initContainer 名のリストを収集する（実行順を保持）
		initContainerNames := make([]string, 0, len(pod.Spec.InitContainers)) // initContainer 名リストを初期化する
		for initIndex := range pod.Spec.InitContainers {
			initContainerNames = append(initContainerNames, pod.Spec.InitContainers[initIndex].Name) // initContainer 名を追加する
		}

		// main container 名のリストを収集する
		mainContainerNames := make([]string, 0, len(pod.Spec.Containers)) // main container 名リストを初期化する
		for containerIndex := range pod.Spec.Containers {
			mainContainerNames = append(mainContainerNames, pod.Spec.Containers[containerIndex].Name) // container 名を追加する
		}

		for _, containerName := range initContainerNames { // initContainer のログを順番に取得する
			if err := waitForContainerRunning(ctx, k8sClient, namespace, pod.Name, containerName); err != nil { // コンテナが起動するまで待機する
				if ctx.Err() != nil {
					return
				}
				logger.PrintErr("collectJobLogs: initContainer 起動待機に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
				continue
			}
			if err := streamPodContainerLogs(ctx, k8sClient, namespace, pod.Name, containerName, logCh); err != nil { // コンテナのログをストリームする
				if ctx.Err() != nil {
					return
				}
				logger.PrintErr("collectJobLogs: initContainer ログ取得に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
			}
		}

		for _, containerName := range mainContainerNames { // main container のログを取得する
			if err := waitForContainerRunning(ctx, k8sClient, namespace, pod.Name, containerName); err != nil { // コンテナが起動するまで待機する
				if ctx.Err() != nil {
					return
				}
				logger.PrintErr("collectJobLogs: main container 起動待機に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
				continue
			}
			if err := streamPodContainerLogs(ctx, k8sClient, namespace, pod.Name, containerName, logCh); err != nil { // コンテナのログをストリームする
				if ctx.Err() != nil {
					return
				}
				logger.PrintErr("collectJobLogs: main container ログ取得に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
			}
		}
	}()

	return logCh // チャンネルを返す
}

// waitForJobPod は build-job-id ラベルに対応する Pod が少なくとも1つ起動するまで最大 120 秒待機する
func waitForJobPod(ctx context.Context, k8sClient kubernetes.Interface, namespace, buildID string) (*corev1.Pod, error) {
	labelSelector := "build-job-id=" + buildID // セレクタを生成する
	for retryIndex := range make([]struct{}, 120) {
		_ = retryIndex
		podList, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector, // build-job-id ラベルで Pod を絞り込む
		})
		if err == nil && len(podList.Items) > 0 {
			return &podList.Items[0], nil // 最初の Pod を返す
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("build-job-id=%s に対応する Pod が 120 秒以内に起動しませんでした", buildID) // タイムアウトエラーを返す
}

// waitForContainerRunning は指定コンテナが Running または Terminated になるまで最大 300 秒待機する
func waitForContainerRunning(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName, containerName string) error {
	for retryIndex := range make([]struct{}, 300) {
		_ = retryIndex
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{}) // Pod の最新状態を取得する
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
				continue
			}
		}

		for statusIndex := range pod.Status.InitContainerStatuses { // initContainer のステータスを確認する
			status := &pod.Status.InitContainerStatuses[statusIndex]
			if status.Name != containerName {
				continue
			}
			if status.State.Running != nil || status.State.Terminated != nil {
				return nil // Running または Terminated であればログ取得可能
			}
		}

		for statusIndex := range pod.Status.ContainerStatuses { // main container のステータスを確認する
			status := &pod.Status.ContainerStatuses[statusIndex]
			if status.Name != containerName {
				continue
			}
			if status.State.Running != nil || status.State.Terminated != nil {
				return nil // Running または Terminated であればログ取得可能
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("コンテナ %s が 300 秒以内に起動しませんでした", containerName) // タイムアウトエラーを返す
}

// streamPodContainerLogs は指定した Pod/コンテナのログを Follow で読み取り、logCh へ送信する
func streamPodContainerLogs(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName, containerName string, logCh chan<- string) error {
	logOptions := &corev1.PodLogOptions{
		Container: containerName, // 対象コンテナ名を設定する
		Follow:    true,          // ログをリアルタイムで追従する
	}
	req := k8sClient.CoreV1().Pods(namespace).GetLogs(podName, logOptions) // ログ取得リクエストを生成する
	stream, err := req.Stream(ctx)                                          // ログストリームを開始する
	if err != nil {
		return err // ストリーム開始エラーを返す
	}
	defer stream.Close() // 終了時にストリームをクローズする

	scanner := bufio.NewScanner(stream)                     // スキャナを生成する
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)      // バッファを拡大する（buildctl の出力に対応）
	for scanner.Scan() {
		line := fmt.Sprintf("[%s] %s", containerName, scanner.Text()) // コンテナ名プレフィックスを付与する
		select {
		case logCh <- line:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil // 正常終了を返す
}
