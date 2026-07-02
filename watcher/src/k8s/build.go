package k8s

import (
	"app/shared/logger"
	"app/shared/models"
	"app/shared/repository"
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


// WatchBuildJobs は全 Namespace の Build Job 変化を監視して DB を自動更新する
func WatchBuildJobs(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	buildRepo repository.DeploymentBuildRepository,
	logChunkRepo repository.BuildLogChunkRepository,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	harborClient *HarborClient,
	registryHost string,
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

		watchBuildJobLoop(ctx, watcher, k8sClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, harborClient, registryHost) // イベントループを実行する

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
	harborClient *HarborClient,
	registryHost string,
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
			handleBuildJobEvent(ctx, event, k8sClient, buildRepo, logChunkRepo, deploymentRepo, projectRepo, harborCredentialRepo, harborClient, registryHost) // イベントを処理する
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
	harborClient *HarborClient,
	registryHost string,
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
		builtImageURL, urlErr := buildBuiltImageURL(ctx, buildData, deploymentRepo, projectRepo, harborCredentialRepo, registryHost) // BuiltImageURL を組み立てる
		if urlErr != nil {
			logger.PrintErr("WatchBuildJobs: BuiltImageURL の組み立てに失敗しました（buildID=" + buildID + "）: " + urlErr.Error()) // エラーをログ出力する
			return
		}
		imageSizeBytes := fetchImageSizeBytes(ctx, buildData, harborClient, harborCredentialRepo) // イメージサイズを Harbor から取得する（失敗しても 0 で継続）
		finishedAt := time.Now()                                                                                                                    // 完了時刻を取得する
		if err := buildRepo.UpdateBuildResult(ctx, buildID, models.BuildStatusSucceeded, builtImageURL, imageSizeBytes, finishedAt); err != nil { // ビルド結果を更新する
			logger.PrintErr("WatchBuildJobs: ビルド結果更新に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}
		if buildData.DeploymentID != nil { // DeploymentID が存在する場合のみ Deployment を更新する（Deployment 削除済みの場合はスキップする）
			deploymentIDValue := *buildData.DeploymentID // pointer をデリファレンスしてローカル変数に格納する
			if err := deploymentRepo.UpdatePendingImageURL(ctx, deploymentIDValue, builtImageURL); err != nil { // pending_image_url を更新する
				logger.PrintErr("WatchBuildJobs: pending_image_url 更新に失敗しました（deploymentID=" + deploymentIDValue + "）: " + err.Error()) // エラーをログ出力する
				return
			}
			builtDeploymentData, findErr := deploymentRepo.FindByID(ctx, deploymentIDValue) // pending_github_* 更新用に deployment を取得する
			if findErr != nil {
				logger.PrintErr("WatchBuildJobs: deployment 取得に失敗しました（deploymentID=" + deploymentIDValue + "）: " + findErr.Error()) // エラーをログ出力する
				return
			}
			if err := deploymentRepo.UpdatePendingGithubBuildFields(ctx, deploymentIDValue, builtDeploymentData.GithubRepoURL, buildData.Branch, buildData.CommitSHA, buildData.Directory); err != nil { // pending_github_* フィールドを現在のビルド情報で更新する
				logger.PrintErr("WatchBuildJobs: pending_github_* 更新に失敗しました（deploymentID=" + deploymentIDValue + "）: " + err.Error()) // エラーをログ出力する
				return
			}
			if err := deploymentRepo.UpdateDeploymentStatus(ctx, deploymentIDValue, models.DeploymentStatusPending); err != nil { // not_init → pending に遷移する
				logger.PrintErr("WatchBuildJobs: DeploymentStatus 更新に失敗しました（deploymentID=" + deploymentIDValue + "）: " + err.Error()) // エラーをログ出力する
				return
			}
		}
		logger.Println("WatchBuildJobs: ビルド成功。succeeded に更新し pending_image_url をセットしました: " + buildID) // 成功ログを出力する

	case k8sJob.Status.Failed > 0 && buildData.Status == models.BuildStatusBuilding: // Job が失敗かつ DB が building の場合
		finishedAt := time.Now()                                                                                                  // 完了時刻を取得する
		if err := buildRepo.UpdateBuildResult(ctx, buildID, models.BuildStatusFailed, "", 0, finishedAt); err != nil { // ビルド結果を failed で更新する
			logger.PrintErr("WatchBuildJobs: ビルド失敗結果更新に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
			return
		}
		// deployment のステータスは変更しない
		// not_init のまま維持することで再ビルドを促す。not_init 以外（pending / running など）も現状維持する
		logger.Println("WatchBuildJobs: ビルド失敗。ビルドレコードを failed に更新しました: " + buildID) // 失敗ログを出力する
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

// collectJobLogs は build-job-id ラベルに対応する Pod を取得し、
// initContainer → main container の順に各コンテナが起動するまで待機してからログをストリームする
func collectJobLogs(ctx context.Context, k8sClient kubernetes.Interface, namespace, buildID string) <-chan string {
	logCh := make(chan string, 100) // ログ行を送るチャンネルを生成する

	go func() {
		defer close(logCh) // 終了時にチャンネルをクローズする

		pod, err := waitForJobPod(ctx, k8sClient, namespace, buildID) // Job に属する Pod が起動するまで待機する
		if err != nil {
			logger.PrintErr("WatchBuildJobs: Pod 取得に失敗しました（buildID=" + buildID + "）: " + err.Error()) // エラーをログ出力する
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

		// initContainer は順番に実行されるため、各コンテナが running または terminated になるまで待機してからログを取得する
		for _, containerName := range initContainerNames {
			if err := waitForContainerRunning(ctx, k8sClient, namespace, pod.Name, containerName); err != nil { // コンテナが起動するまで待機する
				if ctx.Err() != nil { // コンテキストキャンセルの場合は終了する
					return
				}
				logger.PrintErr("WatchBuildJobs: initContainer の起動待機に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
				continue                                                                                                                     // 次のコンテナへ進む
			}
			if err := streamPodContainerLogs(ctx, k8sClient, namespace, pod.Name, containerName, logCh); err != nil { // コンテナのログをストリームする
				if ctx.Err() != nil { // コンテキストキャンセルの場合は終了する
					return
				}
				logger.PrintErr("WatchBuildJobs: initContainer ログ取得に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
			}
		}

		// main container のログを取得する（init コンテナがすべて完了した後に実行される）
		for _, containerName := range mainContainerNames {
			if err := waitForContainerRunning(ctx, k8sClient, namespace, pod.Name, containerName); err != nil { // コンテナが起動するまで待機する
				if ctx.Err() != nil { // コンテキストキャンセルの場合は終了する
					return
				}
				logger.PrintErr("WatchBuildJobs: main container の起動待機に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
				continue                                                                                                                      // 次のコンテナへ進む
			}
			if err := streamPodContainerLogs(ctx, k8sClient, namespace, pod.Name, containerName, logCh); err != nil { // コンテナのログをストリームする
				if ctx.Err() != nil { // コンテキストキャンセルの場合は終了する
					return
				}
				logger.PrintErr("WatchBuildJobs: main container ログ取得に失敗しました（container=" + containerName + "）: " + err.Error()) // エラーをログ出力する
			}
		}
	}()

	return logCh // チャンネルを返す
}

// waitForJobPod は build-job-id ラベルに対応する Pod が少なくとも1つ起動するまで最大 120 秒待機して返す
func waitForJobPod(ctx context.Context, k8sClient kubernetes.Interface, namespace, buildID string) (*corev1.Pod, error) {
	labelSelector := "build-job-id=" + buildID // build-job-id ラベルでフィルタするセレクタを生成する
	for retryIndex := range make([]struct{}, 120) {                             // 最大 120 回リトライする
		_ = retryIndex
		podList, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector, // build-job-id ラベルで Pod を絞り込む
		}) // Pod 一覧を取得する
		if err == nil && len(podList.Items) > 0 { // Pod が1つ以上存在する場合は返す
			return &podList.Items[0], nil // 最初の Pod を返す
		}
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return nil, ctx.Err()
		case <-time.After(time.Second): // 1秒待機してリトライする
		}
	}
	return nil, fmt.Errorf("build-job-id=%s に対応する Pod が 120 秒以内に起動しませんでした", buildID) // タイムアウトエラーを返す
}

// waitForContainerRunning は指定コンテナが Running または Terminated（ログ取得可能）になるまで最大 300 秒待機する
func waitForContainerRunning(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName, containerName string) error {
	for retryIndex := range make([]struct{}, 300) { // 最大 300 回（300 秒）リトライする
		_ = retryIndex
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{}) // Pod の最新状態を取得する
		if err != nil {
			select {
			case <-ctx.Done(): // コンテキストキャンセルの場合は終了する
				return ctx.Err()
			case <-time.After(time.Second):
				continue // Pod 取得失敗は一時的なものとしてリトライする
			}
		}

		// initContainer のステータスを確認する
		for statusIndex := range pod.Status.InitContainerStatuses {
			status := &pod.Status.InitContainerStatuses[statusIndex]
			if status.Name != containerName { // 対象コンテナでない場合はスキップする
				continue
			}
			if status.State.Running != nil || status.State.Terminated != nil { // Running または Terminated であればログ取得可能
				return nil
			}
		}

		// main container のステータスを確認する
		for statusIndex := range pod.Status.ContainerStatuses {
			status := &pod.Status.ContainerStatuses[statusIndex]
			if status.Name != containerName { // 対象コンテナでない場合はスキップする
				continue
			}
			if status.State.Running != nil || status.State.Terminated != nil { // Running または Terminated であればログ取得可能
				return nil
			}
		}

		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return ctx.Err()
		case <-time.After(time.Second): // 1秒待機してリトライする
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

	scanner := bufio.NewScanner(stream) // スキャナを生成する
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 長い行（buildctl の出力）に対応するためバッファを拡大する
	for scanner.Scan() {                               // ログを1行ずつ読み取る
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
	registryHost string,
) (string, error) {
	projectData, err := projectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を ProjectID から直接取得する
	if err != nil {
		return "", fmt.Errorf("project の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	// harbor credential はロボットアカウント認証情報の取得のみに使う（エンドポイントは registryHost を使う）
	_, err = harborCredentialRepo.FindByProjectIDNoTx(ctx, buildData.ProjectID) // harbor credential の存在確認をする
	if err != nil {
		return "", fmt.Errorf("harbor credential の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	// イメージ名にはビルド元の Deployment ID を使う（DeploymentID が nil の場合はビルド ID で代替する）
	imageName := buildData.ID // デフォルトはビルド ID を使う
	if buildData.DeploymentID != nil {
		imageName = *buildData.DeploymentID // Deployment ID が存在する場合はそれを使う
	}

	imageURL := fmt.Sprintf("%s/%s/%s:%s",
		registryHost,       // クラスタ内 DNS 名を使用する
		projectData.ID,     // Harbor プロジェクト名にプロジェクト ID を使う
		imageName,          // イメージ名に Deployment ID を使う
		buildData.ID,       // ビルド ID をタグに使用する
	) // イメージURLを組み立てる

	return imageURL, nil // イメージURLを返す
}

// fetchImageSizeBytes は Harbor からビルドイメージのサイズをバイト単位で取得する（失敗時は 0 を返す）
func fetchImageSizeBytes(ctx context.Context, buildData *models.DeploymentBuild, harborClient *HarborClient, harborCredentialRepo repository.HarborCredentialRepository) int64 {
	if harborClient == nil { // harborClient が nil の場合はスキップする（テスト環境など）
		return 0
	}
	credentialData, err := harborCredentialRepo.FindByProjectIDNoTx(ctx, buildData.ProjectID) // Harbor 認証情報を取得する
	if err != nil {
		logger.PrintErr("fetchImageSizeBytes: harbor credential の取得に失敗しました（projectID=" + buildData.ProjectID + "）: " + err.Error()) // エラーをログ出力する
		return 0                                                                                                                               // 失敗しても 0 を返して継続する
	}
	credential := HarborRobotCredential{
		Name:   credentialData.RobotName,   // robot アカウント名を設定する
		Secret: credentialData.RobotSecret, // シークレットを設定する
	}
	repositoryName := buildData.ID // デフォルトはビルド ID を使う
	if buildData.DeploymentID != nil {
		repositoryName = *buildData.DeploymentID // Deployment ID が存在する場合はそれをリポジトリ名として使う
	}
	sizeBytes, sizeErr := harborClient.GetArtifactSize(ctx, buildData.ProjectID, repositoryName, credential) // Harbor からサイズを取得する
	if sizeErr != nil {
		logger.PrintErr("fetchImageSizeBytes: イメージサイズの取得に失敗しました（buildID=" + buildData.ID + "）: " + sizeErr.Error()) // エラーをログ出力する
		return 0                                                                                                                  // 失敗しても 0 を返して継続する
	}
	return sizeBytes // サイズを返す
}

// resolveNamespace はビルドデータから namespace を解決する
func resolveNamespace(
	ctx context.Context,
	buildData *models.DeploymentBuild,
	deploymentRepo repository.DeploymentRepository,
	projectRepo repository.ProjectRepository,
) (string, error) {
	projectData, err := projectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を ProjectID から直接取得する
	if err != nil {
		return "", fmt.Errorf("project の取得に失敗しました: %w", err) // 取得エラーを返す
	}

	return projectData.Namespace, nil // namespace を返す
}
