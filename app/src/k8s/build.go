package k8s

import (
	"app/logger"
	"app/models"
	"app/repository"
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// CreateBuildJob は dockerfile ビルダーによる k8s Job を作成する
// dockerfile ビルダーは設計検討中のため現時点では未実装
func CreateBuildJob(
	ctx context.Context,
	client kubernetes.Interface,
	buildID string,
	namespace string,
) (string, error) {
	return "", fmt.Errorf("dockerfile ビルダーは未実装です（ISSUE-051 で実装予定）") // dockerfile ビルダーは未実装のためエラーを返す
}

// DeleteBuildJob は jobName に対応する k8s Job を削除する
func DeleteBuildJob(ctx context.Context, client kubernetes.Interface, namespace, jobName string) error {
	propagationPolicy := metav1.DeletePropagationForeground                                                                    // Pod も連動して削除する
	return client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}) // Job を削除する
}

// WatchBuildJobs は全 Namespace の Build Job 変化を監視して DB を自動更新する
func WatchBuildJobs(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	buildRepo repository.DeploymentBuildRepository,
	logChunkRepo repository.BuildLogChunkRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
) {
	// 起動時リカバリ: building 状態のビルドに対してログストリームを再開する
	buildingList, err := buildRepo.FindAllBuilding(ctx) // building 状態のビルドを全件取得する
	if err != nil {
		logger.PrintErr("WatchBuildJobs: building ビルドの取得に失敗しました: " + err.Error()) // エラーをログ出力する
	} else {
		for buildIndex := range buildingList { // 各ビルドに対してログストリームを再開する
			buildData := &buildingList[buildIndex]                                                                          // ポインタを取得する
			namespace, namespaceErr := resolveNamespace(ctx, buildData, deploymentRepo, projectRepo) // namespace を取得する
			if namespaceErr != nil {
				logger.PrintErr("WatchBuildJobs: namespace 解決に失敗しました（buildID=" + buildData.ID + "）: " + namespaceErr.Error()) // エラーをログ出力する
				continue                                                                                                               // 次のビルドに進む
			}
			go streamAndSaveLogs(ctx, k8sClient, buildData, namespace, logChunkRepo) // ログストリームを goroutine で再開する
		}
	}

	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, watchErr := k8sClient.BatchV1().Jobs("").Watch(ctx, metav1.ListOptions{
			LabelSelector: "build-job-id", // build-job-id ラベルを持つ Job のみ監視する
		}) // Watch を開始する
		if watchErr != nil {
			logger.PrintErr("WatchBuildJobs: Watch 開始に失敗しました: " + watchErr.Error()) // エラーをログ出力する
			continue                                                                        // 再試行する
		}

		logger.Println("WatchBuildJobs: 監視を開始しました") // 監視開始ログを出力する

		watchBuildJobLoop(ctx, watcher, k8sClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo) // イベントループを実行する

		logger.Println("WatchBuildJobs: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// watchBuildJobLoop は Build Job Watch イベントチャネルを処理するループ
func watchBuildJobLoop(
	ctx context.Context,
	watcher watch.Interface,
	k8sClient kubernetes.Interface,
	buildRepo repository.DeploymentBuildRepository,
	logChunkRepo repository.BuildLogChunkRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
) {
	defer watcher.Stop() // 終了時に Watch を停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case event, ok := <-watcher.ResultChan(): // イベントを受信する
			if !ok { // チャネルが閉じられた場合はループを抜ける
				return
			}
			handleBuildJobEvent(ctx, event, k8sClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo) // イベントを処理する
		}
	}
}

// handleBuildJobEvent は Build Job の Watch イベントを処理する
func handleBuildJobEvent(
	ctx context.Context,
	event watch.Event,
	k8sClient kubernetes.Interface,
	buildRepo repository.DeploymentBuildRepository,
	logChunkRepo repository.BuildLogChunkRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
) {
	k8sJob, ok := event.Object.(*batchv1.Job) // イベントオブジェクトを Job にキャストする
	if !ok {                                   // キャストに失敗した場合はスキップする
		return
	}

	if event.Type != watch.Added && event.Type != watch.Modified { // Added/Modified 以外はスキップする
		return
	}

	buildID, exists := k8sJob.Labels["build-job-id"] // build-job-id ラベルを取得する
	if !exists || buildID == "" {                     // ラベルが存在しない場合はスキップする
		return
	}

	buildData, err := buildRepo.FindByID(ctx, buildID) // ビルドレコードを取得する
	if err != nil {
		logger.PrintErr("WatchBuildJobs: ビルドレコード取得に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
		return
	}

	namespace := k8sJob.Namespace // Job の namespace を取得する

	switch {
	case k8sJob.Status.Active > 0 && buildData.Status == models.BuildStatusPending: // Job が Active かつ DB が pending の場合
		if err := buildRepo.UpdateStatus(ctx, buildID, models.BuildStatusBuilding); err != nil { // ステータスを building に更新する
			logger.PrintErr("WatchBuildJobs: ステータス更新に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}
		buildData.Status = models.BuildStatusBuilding                      // ローカルのステータスを更新する
		go streamAndSaveLogs(ctx, k8sClient, buildData, namespace, logChunkRepo) // ログストリームを goroutine で開始する
		logger.Println("WatchBuildJobs: ビルドを building に更新してログストリームを開始しました: " + buildID) // 更新ログを出力する

	case k8sJob.Status.Succeeded > 0 && buildData.Status == models.BuildStatusBuilding: // Job が成功かつ DB が building の場合
		builtImageURL, urlErr := buildBuiltImageURL(ctx, buildData, deploymentRepo, projectRepo, harborCredentialRepo) // BuiltImageURL を組み立てる
		if urlErr != nil {
			logger.PrintErr("WatchBuildJobs: BuiltImageURL の組み立てに失敗しました（buildID=" + buildID + "）: " + urlErr.Error()) // エラーをログ出力する
			return
		}
		finishedAt := time.Now()                                                                                                  // 完了時刻を取得する
		if err := buildRepo.UpdateBuildResult(ctx, buildID, models.BuildStatusSucceeded, builtImageURL, finishedAt); err != nil { // ビルド結果を更新する
			logger.PrintErr("WatchBuildJobs: ビルド結果更新に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}
		if err := deploymentRepo.UpdatePendingImageURL(ctx, buildData.DeploymentID, builtImageURL); err != nil { // pending_image_url を更新する
			logger.PrintErr("WatchBuildJobs: pending_image_url 更新に失敗しました（deploymentID=" + buildData.DeploymentID + "）: " + err.Error()) // エラーをログ出力する
			return
		}
		logger.Println("WatchBuildJobs: ビルド成功。succeeded に更新し pending_image_url をセットしました: " + buildID) // 成功ログを出力する

	case k8sJob.Status.Failed > 0 && buildData.Status == models.BuildStatusBuilding: // Job が失敗かつ DB が building の場合
		finishedAt := time.Now()                                                                                               // 完了時刻を取得する
		if err := buildRepo.UpdateBuildResult(ctx, buildID, models.BuildStatusFailed, "", finishedAt); err != nil { // ビルド結果を failed で更新する
			logger.PrintErr("WatchBuildJobs: ビルド失敗結果更新に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}
		logger.Println("WatchBuildJobs: ビルド失敗。failed に更新しました: " + buildID) // 失敗ログを出力する
	}
}

// streamAndSaveLogs は指定されたビルドの Job に属する全 Pod のログをリアルタイムで取得し、chunk 単位で DB に保存する
func streamAndSaveLogs(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	buildData *models.DeploymentBuild,
	namespace string,
	logChunkRepo repository.BuildLogChunkRepository,
) {
	if buildData.BuildType == models.BuildTypeDockerfile { // Dockerfile ビルドはログ取得未実装のためスキップする
		return
	}

	logCh := collectJobLogs(ctx, k8sClient, namespace, buildData.ID) // Job に属する全 Pod のログをチャンネルで取得する

	ticker := time.NewTicker(3 * time.Second) // 3秒ごとにバッファをフラッシュするタイマーを生成する
	defer ticker.Stop()                       // 終了時にタイマーを停止する

	var buf strings.Builder // ログバッファを生成する

	flush := func() { // バッファをフラッシュして DB に保存する関数
		if buf.Len() == 0 { // バッファが空の場合はスキップする
			return
		}
		chunk := &models.BuildLogChunk{ // ログチャンクレコードを生成する
			BuildID: buildData.ID, // ビルドIDを設定する
			Content: buf.String(), // バッファの内容を設定する
		}
		if err := logChunkRepo.Create(ctx, chunk); err != nil { // DB に保存する
			logger.PrintErr("WatchBuildJobs: ログチャンク保存に失敗しました（buildID=" + buildData.ID + "）: " + err.Error()) // エラーをログ出力する
		}
		buf.Reset() // バッファをリセットする
	}

	for {
		select {
		case line, ok := <-logCh: // ログ行を受信する
			if !ok { // チャネルが閉じられた場合はフラッシュして終了する
				flush()
				return
			}
			buf.WriteString(line + "\n") // ログ行をバッファに追記する
			if buf.Len() > 4096 {        // バッファが 4096 バイトを超えたら即時フラッシュする
				flush()
			}
		case <-ticker.C: // 3秒タイマーが発火したらフラッシュする
			flush()
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		}
	}
}

// collectJobLogs は build-job-id ラベルに対応する全 Pod を取得し、各 Pod の全コンテナログをチャンネルで返す
func collectJobLogs(ctx context.Context, k8sClient kubernetes.Interface, namespace, buildID string) <-chan string {
	logCh := make(chan string, 100) // ログ行を送るチャンネルを生成する

	go func() {
		defer close(logCh) // 終了時にチャンネルをクローズする

		podList, err := waitForJobPods(ctx, k8sClient, namespace, buildID) // Job に属する Pod が起動するまで待機する
		if err != nil {
			logger.PrintErr("WatchBuildJobs: Pod 取得に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}

		for podIndex := range podList { // 各 Pod のログを取得する
			pod := &podList[podIndex]

			// initContainers と containers の全コンテナ名を収集する
			containerNames := make([]string, 0) // コンテナ名一覧を初期化する
			for initIndex := range pod.Spec.InitContainers {
				containerNames = append(containerNames, pod.Spec.InitContainers[initIndex].Name) // initContainer 名を追加する
			}
			for containerIndex := range pod.Spec.Containers {
				containerNames = append(containerNames, pod.Spec.Containers[containerIndex].Name) // container 名を追加する
			}

			for _, containerName := range containerNames { // 各コンテナのログをストリームする
				if err := streamPodContainerLogs(ctx, k8sClient, namespace, pod.Name, containerName, logCh); err != nil { // コンテナのログをストリームする
					if ctx.Err() == nil { // コンテキストキャンセル以外のエラーをログ出力する
						logger.PrintErr("WatchBuildJobs: コンテナログ取得に失敗しました（pod=" + pod.Name + ", container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
					}
				}
			}
		}
	}()

	return logCh // チャンネルを返す
}

// waitForJobPods は build-job-id ラベルに対応する Pod が少なくとも1つ起動するまで最大 60 秒待機して返す
func waitForJobPods(ctx context.Context, k8sClient kubernetes.Interface, namespace, buildID string) ([]corev1.Pod, error) {
	labelSelector := "build-job-id=" + buildID // build-job-id ラベルでフィルタするセレクタを生成する
	for retryIndex := range make([]struct{}, 60) {                              // 最大 60 回リトライする
		_ = retryIndex
		podList, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector, // build-job-id ラベルで Pod を絞り込む
		}) // Pod 一覧を取得する
		if err == nil && len(podList.Items) > 0 { // Pod が1つ以上存在する場合は返す
			return podList.Items, nil
		}
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return nil, ctx.Err()
		case <-time.After(time.Second): // 1秒待機してリトライする
		}
	}
	return nil, fmt.Errorf("build-job-id=%s に対応する Pod が 60 秒以内に起動しませんでした", buildID) // タイムアウトエラーを返す
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

	scanner := bufio.NewScanner(stream) // スキャナを生成する
	for scanner.Scan() {                // ログを1行ずつ読み取る
		line := fmt.Sprintf("[%s] %s", containerName, scanner.Text()) // コンテナ名プレフィックスを付与する
		select {
		case logCh <- line: // ログ行をチャンネルへ送信する
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return ctx.Err()
		}
	}
	return nil // 正常終了を返す
}

// buildBuiltImageURL はビルド成功時のイメージURLを組み立てる
func buildBuiltImageURL(
	ctx context.Context,
	buildData *models.DeploymentBuild,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
) (string, error) {
	deploymentData, err := deploymentRepo.FindByID(ctx, buildData.DeploymentID) // deployment を取得する
	if err != nil {
		return "", fmt.Errorf("deployment の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	projectData, err := projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return "", fmt.Errorf("project の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	harborCredential, err := harborCredentialRepo.FindByProjectIDNoTx(ctx, deploymentData.ProjectID) // harbor credential を取得する
	if err != nil {
		return "", fmt.Errorf("harbor credential の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	imageURL := fmt.Sprintf("%s/%s/%s:%s",
		harborCredential.HarborEndpoint, // Harbor エンドポイントを設定する
		projectData.Name,                // プロジェクト名を設定する
		deploymentData.Name,             // デプロイメント名を設定する
		buildData.ID,                    // ビルドIDをタグに使用する
	) // イメージURLを組み立てる

	return imageURL, nil // イメージURLを返す
}

// resolveNamespace はビルドデータから namespace を解決する
func resolveNamespace(
	ctx context.Context,
	buildData *models.DeploymentBuild,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
) (string, error) {
	deploymentData, err := deploymentRepo.FindByID(ctx, buildData.DeploymentID) // deployment を取得する
	if err != nil {
		return "", fmt.Errorf("deployment の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	projectData, err := projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return "", fmt.Errorf("project の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	return projectData.Namespace, nil // namespace を返す
}
