package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"handler/k8s"
	"handler/models"
	"handler/repository"

	temporalclient "go.temporal.io/sdk/client"
	"k8s.io/client-go/kubernetes"
)

// ErrBuildConflict はビルドが既に進行中の場合のエラー
var ErrBuildConflict = errors.New("build already in progress")

// ErrBuildNotCancellable はビルドがキャンセル不可能な状態の場合のエラー
var ErrBuildNotCancellable = errors.New("build is not cancellable")

// ErrDockerfileNotSupported は dockerfile ビルドタイプが現在サポートされていない場合のエラー
var ErrDockerfileNotSupported = errors.New("dockerfile タイプは現在サポートされていません")

// BuildService はビルドトリガーのビジネスロジックを定義するインターフェース
type BuildService interface {
	TriggerBuild(ctx context.Context, userID string, deploymentID string, commitMessage string, author string) (*models.DeploymentBuild, error) // ビルドをトリガーする
	CancelBuild(ctx context.Context, userID string, buildID string) error                                          // ビルドをキャンセルする
	GetBuild(ctx context.Context, userID string, buildID string) (*models.DeploymentBuild, error)                  // ビルド情報を取得する
	GetBuildLogs(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) // ビルドログを取得する（ログ文字列・最終チャンク時刻・エラー）
	ListBuilds(ctx context.Context, userID string, deploymentID string) ([]models.DeploymentBuild, error)          // ビルド一覧を取得する（deployment 単位）
	ListBuildsByProject(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error)    // ビルド一覧を取得する（project 単位）
	DeleteBuild(ctx context.Context, userID string, projectID string, buildID string) error                        // ビルドを削除する（Harbor イメージも削除する）
}

// TriggerBuildRequest は TriggerBuild のリクエスト構造体（現時点では deploymentID のみ）

// BuildWorkflowInput は BuildWorkflow への入力（builder worker のタスクキューに送信する）
type BuildWorkflowInput struct {
	BuildID string // 対象ビルドの ID（あらかじめ DB に pending 状態で作成済み）
}

// CancelBuildWorkflowInput は CancelBuildWorkflow への入力
type CancelBuildWorkflowInput struct {
	BuildID string // キャンセル対象ビルドの ID
}

// buildServiceImpl は BuildService の実装
type buildServiceImpl struct {
	deploymentRepo       repository.DeploymentRepository       // deployment リポジトリ
	buildRepo            repository.DeploymentBuildRepository  // build リポジトリ
	projectRepo          repository.ProjectRepository          // project リポジトリ（所有権チェック用）
	harborCredentialRepo repository.HarborCredentialRepository // harbor credential リポジトリ
	logChunkRepo         repository.BuildLogChunkRepository    // ログチャンクリポジトリ
	k8sClient            kubernetes.Interface                  // k8s クライアント（Harbor クライアント等に使用）
	harborClient         *k8s.HarborClient                    // Harbor API クライアント（イメージ削除用）
	registryHost         string                                // ビルドジョブが使う Harbor ホスト名（スキームなし）
	temporalClient       WorkflowStarter                      // Temporal クライアント（Workflow 起動用）
}

// NewBuildService は BuildService の実装を返す
func NewBuildService(
	deploymentRepo repository.DeploymentRepository,
	buildRepo repository.DeploymentBuildRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	logChunkRepo repository.BuildLogChunkRepository,
	k8sClient kubernetes.Interface,
	harborClient *k8s.HarborClient,
	registryHost string,
	temporalClient WorkflowStarter,
) BuildService {
	return &buildServiceImpl{
		deploymentRepo:       deploymentRepo,       // deployment リポジトリを注入する
		buildRepo:            buildRepo,            // build リポジトリを注入する
		projectRepo:          projectRepo,          // project リポジトリを注入する
		harborCredentialRepo: harborCredentialRepo, // harbor credential リポジトリを注入する
		logChunkRepo:         logChunkRepo,         // ログチャンクリポジトリを注入する
		k8sClient:            k8sClient,            // k8s クライアントを注入する
		harborClient:         harborClient,         // Harbor クライアントを注入する
		registryHost:         registryHost,         // クラスタ内 DNS 名を注入する
		temporalClient:       temporalClient,       // Temporal クライアントを注入する
	}
}

// TriggerBuild はデプロイメントのビルドをトリガーする
func (svc *buildServiceImpl) TriggerBuild(ctx context.Context, userID string, deploymentID string, commitMessage string, author string) (*models.DeploymentBuild, error) {
	// 1. Deployment を取得する
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	// 1.5. dockerfile タイプはサポートされていないため拒否する
	if deploymentData.Type == models.DeploymentTypeDockerfile { // dockerfile タイプは ISSUE-051 未完のため拒否する
		return nil, ErrDockerfileNotSupported // サポート外エラーを返す
	}

	// 2. 所有権チェック（Deployment の ProjectID から Project を取得して UserID を比較する）
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	// 3. 進行中のビルドがないか確認する（pending / building は二重起動不可）
	buildList, err := svc.buildRepo.FindAllByDeploymentID(ctx, deploymentID) // ビルド一覧を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	for _, existingBuild := range buildList { // 各ビルドの状態を確認する
		if existingBuild.Status == models.BuildStatusPending || existingBuild.Status == models.BuildStatusBuilding { // ビルド中または待機中の場合はコンフリクトエラーを返す
			return nil, ErrBuildConflict
		}
	}

	// 4. ビルドタイプを決定する
	buildType, err := resolveBuildType(deploymentData.Type) // deployment タイプからビルドタイプを解決する
	if err != nil {
		return nil, err // 解決エラーを返す
	}

	// 5. DeploymentBuild レコードを pending で作成する
	deploymentIDValue := deploymentID                        // pointer 用にローカル変数にコピーする
	buildData := &models.DeploymentBuild{
		ProjectID:      deploymentData.ProjectID,              // project ID を設定する（Deployment 削除後もビルドを保持するため）
		DeploymentID:   &deploymentIDValue,                    // deployment ID を設定する（nullable pointer）
		BuildType:      buildType,                             // ビルドタイプを設定する
		Status:         models.BuildStatusPending,             // 初期ステータスを pending に設定する
		GithubRepoURL:  deploymentData.PendingGithubRepoURL,       // GitHub リポジトリ URL をスナップショットする
		CommitSHA:      deploymentData.PendingGithubCommitSHA,      // コミット SHA を設定する
		Branch:         deploymentData.PendingGithubBranch,         // ブランチを設定する
		Directory:      deploymentData.PendingGithubRepoDirectory,  // ビルドディレクトリを設定する
		DockerfilePath: deploymentData.PendingDockerfilePath,       // Dockerfile パスを設定する
		CommitMessage:  commitMessage,                               // コミットメッセージを設定する
		Author:         author,                                      // コミット著者を設定する
	}
	if err := svc.buildRepo.Create(ctx, buildData); err != nil { // build レコードを作成する
		return nil, err // 作成エラーを返す
	}

	if err := svc.deploymentRepo.UpdateCurrentBuildID(ctx, deploymentID, buildData.ID); err != nil { // current_build_id を最新ビルドに更新する
		return nil, err // 更新エラーを返す
	}

	// 7. Temporal BuildWorkflow を起動する（K8s Job 作成とログ収集は builder Worker が担う）
	workflowOptions := temporalclient.StartWorkflowOptions{
		ID:        "build-" + buildData.ID, // WorkflowID を設定して冪等性を保証する
		TaskQueue: "builder-queue",         // builder Worker のタスクキューを指定する
	}
	workflowInput := BuildWorkflowInput{BuildID: buildData.ID} // Workflow 入力を構築する
	_, startErr := svc.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "BuildWorkflow", workflowInput) // Workflow を起動する
	if startErr != nil {
		// Workflow 起動失敗時はビルドレコードを failed に更新してから返す（pending のまま残すと再ビルドが永久に弾かれるため）
		finishedAt := time.Now()
		_ = svc.buildRepo.UpdateBuildResult(ctx, buildData.ID, models.BuildStatusFailed, "", 0, finishedAt) // ロールバック：failed に更新する
		return nil, fmt.Errorf("ビルド workflow の起動に失敗しました: %w", startErr)                              // 起動エラーを返す
	}

	return buildData, nil // 作成したビルドレコードを返す
}

// GetBuild は指定したビルドレコードを返す
func (svc *buildServiceImpl) GetBuild(ctx context.Context, userID string, buildID string) (*models.DeploymentBuild, error) {
	buildData, err := svc.buildRepo.FindByID(ctx, buildID) // ビルドレコードを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	if buildData.DeploymentID == nil { // DeploymentID が nil の場合（Deployment 削除済み）は project 経由で確認する
		projectData, projectErr := svc.projectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を取得する
		if projectErr != nil {
			return nil, projectErr // 取得エラーを返す
		}
		if projectData.UserID != userID { // 所有権を確認する
			return nil, ErrForbidden // 権限なしエラーを返す
		}
		return buildData, nil // ビルドレコードを返す
	}

	deploymentData, err := svc.deploymentRepo.FindByID(ctx, *buildData.DeploymentID) // 所有権チェック用にデプロイメントを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // プロジェクトを取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	if projectData.UserID != userID { // 所有権を確認する
		return nil, ErrForbidden // 権限なしエラーを返す
	}

	return buildData, nil // ビルドレコードを返す
}

// CancelBuild は実行中のビルドをキャンセルする
func (svc *buildServiceImpl) CancelBuild(ctx context.Context, userID string, buildID string) error {
	// 1. ビルドレコードを取得する
	buildData, err := svc.buildRepo.FindByID(ctx, buildID) // ビルドレコードを取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	// 2. 所有権チェック（Build → Project → UserID を辿る）
	cancelProjectData, err := svc.projectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if cancelProjectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}

	// 3. キャンセル可能なステータスか確認する（pending / building のみキャンセル可能）
	if buildData.Status != models.BuildStatusPending && buildData.Status != models.BuildStatusBuilding { // 完了済み・失敗済みはキャンセル不可
		return ErrBuildNotCancellable
	}

	// 4. 実行中の BuildWorkflow をキャンセルする（Temporal 側のワークフローをキャンセル）
	buildWorkflowID := "build-" + buildID                                                      // WorkflowID を設定する
	_ = svc.temporalClient.CancelWorkflow(ctx, buildWorkflowID, "")                           // Workflow をキャンセルする（存在しない場合のエラーは無視する）

	// 5. Temporal CancelBuildWorkflow を起動する（K8s Job 削除と DB ステータス更新を builder Worker が担う）
	workflowOptions := temporalclient.StartWorkflowOptions{
		ID:        "cancel-build-" + buildID, // WorkflowID を設定して冪等性を保証する
		TaskQueue: "builder-queue",           // builder Worker のタスクキューを指定する
	}
	workflowInput := CancelBuildWorkflowInput{BuildID: buildID} // Workflow 入力を構築する
	_, startErr := svc.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "CancelBuildWorkflow", workflowInput) // Workflow を起動する
	if startErr != nil {
		return fmt.Errorf("ビルドキャンセル workflow の起動に失敗しました: %w", startErr) // 起動エラーを返す
	}

	return nil // キャンセル Workflow 起動成功
}

// GetBuildLogs はビルドIDに紐づくログを取得して結合した文字列と最終チャンク時刻を返す
func (svc *buildServiceImpl) GetBuildLogs(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) {
	// 1. ビルドレコードを取得する
	buildData, err := svc.buildRepo.FindByID(ctx, buildID) // ビルドレコードを取得する
	if err != nil {
		return "", nil, err // 取得エラーを返す
	}

	// 2. 所有権チェック（Build → Project → UserID を辿る）
	logsProjectData, err := svc.projectRepo.FindByIDNoTx(ctx, buildData.ProjectID) // project を取得する
	if err != nil {
		return "", nil, err // 取得エラーを返す
	}
	if logsProjectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return "", nil, ErrForbidden
	}

	// 3. ログチャンクを取得する（since 指定あり／なしで分岐する）
	var chunkList []models.BuildLogChunk
	if since != nil { // since パラメータが指定されている場合は差分を取得する
		chunkList, err = svc.logChunkRepo.FindByBuildIDSince(ctx, buildID, *since) // since より後のチャンクを取得する
	} else {
		chunkList, err = svc.logChunkRepo.FindByBuildID(ctx, buildID) // 全チャンクを取得する
	}
	if err != nil {
		return "", nil, err // 取得エラーを返す
	}

	// 4. チャンクを結合して最終チャンクの時刻を返す
	var logBuilder strings.Builder               // ログ文字列ビルダーを生成する
	var lastChunkTime *time.Time                 // 最終チャンクの時刻を保持する
	for _, chunk := range chunkList {            // 各チャンクを結合する
		logBuilder.WriteString(chunk.Content)    // チャンクの内容を追記する
		chunkTime := chunk.CreatedAt             // ループ変数のアドレスをコピーする
		lastChunkTime = &chunkTime               // 最終チャンクの時刻を更新する
	}
	return logBuilder.String(), lastChunkTime, nil // 結合したログ文字列と最終時刻を返す
}

// ListBuilds は deploymentID に紐づくビルド一覧を返す
func (svc *buildServiceImpl) ListBuilds(ctx context.Context, userID string, deploymentID string) ([]models.DeploymentBuild, error) {
	// 1. 所有権チェック
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	// 2. ビルド一覧を取得して返す
	buildList, err := svc.buildRepo.FindAllByDeploymentID(ctx, deploymentID) // ビルド一覧を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return buildList, nil // ビルド一覧を返す
}


// ListBuildsByProject は projectID に紐づくビルド一覧を返す
func (svc *buildServiceImpl) ListBuildsByProject(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error) {
	// 1. 所有権チェック
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	// 2. ビルド一覧を取得して返す
	buildList, err := svc.buildRepo.FindAllByProjectID(ctx, projectID) // project 単位でビルド一覧を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	return buildList, nil // ビルド一覧を返す
}

// DeleteBuild はビルドを削除する（Harbor イメージも削除する）
func (svc *buildServiceImpl) DeleteBuild(ctx context.Context, userID string, projectID string, buildID string) error {
	// 1. 所有権チェック
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, projectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}

	// 2. ビルドレコードを取得する
	buildData, err := svc.buildRepo.FindByID(ctx, buildID) // ビルドを取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if buildData.ProjectID != projectID { // 別プロジェクトのビルドへのアクセスを禁止する
		return ErrForbidden
	}

	// 3. 進行中のビルドは削除不可とする
	if buildData.Status == models.BuildStatusPending || buildData.Status == models.BuildStatusBuilding { // ビルド中の場合は削除不可
		return ErrBuildConflict
	}

	// 4. FK 制約回避: このビルドを current_build_id として参照している Deployment の current_build_id を NULL にクリアする
	if buildData.DeploymentID != nil { // DeploymentID が存在する場合のみ確認する
		deploymentData, findErr := svc.deploymentRepo.FindByID(ctx, *buildData.DeploymentID) // Deployment を取得する
		if findErr == nil && deploymentData.CurrentBuildID != nil && *deploymentData.CurrentBuildID == buildID { // current_build_id が削除対象ビルドを指している場合
			if clearErr := svc.deploymentRepo.ClearCurrentBuildID(ctx, *buildData.DeploymentID); clearErr != nil { // current_build_id を NULL にクリアする
				return fmt.Errorf("current_build_id のクリアに失敗しました: %w", clearErr) // クリアエラーを返す
			}
		}
	}

	// 6. Harbor からイメージを削除する（succeeded 状態の場合のみ Harbor 上にイメージが存在する）
	if buildData.BuiltImageURL != "" { // BuiltImageURL が設定されている場合のみ Harbor 削除を試みる
		credentialData, credErr := svc.harborCredentialRepo.FindByProjectIDNoTx(ctx, projectID) // Harbor 認証情報を取得する
		if credErr == nil {                                                                       // 認証情報が取得できた場合のみ削除を試みる
			repositoryName := buildData.ID               // デフォルトはビルド ID をリポジトリ名として使う
			if buildData.DeploymentID != nil {
				repositoryName = *buildData.DeploymentID // Deployment ID が存在する場合はそれを使う
			}
			credential := k8s.HarborRobotCredential{
				Name:   credentialData.RobotName,   // robot アカウント名を設定する
				Secret: credentialData.RobotSecret, // シークレットを設定する
			}
			if harborErr := svc.harborClient.DeleteHarborImage(ctx, projectID, repositoryName, credential); harborErr != nil { // Harbor イメージを削除する
				return fmt.Errorf("Harbor イメージの削除に失敗しました: %w", harborErr) // 削除エラーを返す
			}
		}
	}

	// 7. DB レコードを削除する
	if err := svc.buildRepo.Delete(ctx, buildData); err != nil { // ビルドレコードを削除する
		return err // 削除エラーを返す
	}
	return nil // 正常終了
}

// resolveBuildType は DeploymentType からビルドタイプを解決する
func resolveBuildType(deploymentType models.DeploymentType) (models.BuildType, error) {
	switch deploymentType {
	case models.DeploymentTypeDockerfile:
		return models.BuildTypeDockerfile, nil // dockerfile タイプを返す
	case models.DeploymentTypeRailpack:
		return models.BuildTypeRailpack, nil // railpack タイプを返す
	default:
		return "", fmt.Errorf("ビルドをサポートしないデプロイメントタイプ: %s", deploymentType) // 非対応タイプはエラーを返す
	}
}
