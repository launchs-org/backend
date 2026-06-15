package service

import (
	"app/k8s"
	"app/models"
	"app/railpack"
	"app/repository"
	"context"
	"errors"
	"fmt"

	"k8s.io/client-go/kubernetes"
)

// ErrBuildConflict はビルドが既に進行中の場合のエラー
var ErrBuildConflict = errors.New("build already in progress")

// ErrBuildNotCancellable はビルドがキャンセル不可能な状態の場合のエラー
var ErrBuildNotCancellable = errors.New("build is not cancellable")

// BuildService はビルドトリガーのビジネスロジックを定義するインターフェース
type BuildService interface {
	TriggerBuild(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error) // ビルドをトリガーする
	CancelBuild(ctx context.Context, userID string, buildID string) error                                  // ビルドをキャンセルする
}

// TriggerBuildRequest は TriggerBuild のリクエスト構造体（現時点では deploymentID のみ）

// buildServiceImpl は BuildService の実装
type buildServiceImpl struct {
	deploymentRepo       repository.DeploymentRepository       // deployment リポジトリ
	buildRepo            repository.DeploymentBuildRepository  // build リポジトリ
	projectRepo          repository.ProjectRepository          // project リポジトリ（所有権チェック用）
	harborCredentialRepo repository.HarborCredentialRepository // harbor credential リポジトリ
	k8sClient            kubernetes.Interface                  // k8s クライアント
}

// NewBuildService は BuildService の実装を返す
func NewBuildService(
	deploymentRepo repository.DeploymentRepository,
	buildRepo repository.DeploymentBuildRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	k8sClient kubernetes.Interface,
) BuildService {
	return &buildServiceImpl{
		deploymentRepo:       deploymentRepo,       // deployment リポジトリを注入する
		buildRepo:            buildRepo,            // build リポジトリを注入する
		projectRepo:          projectRepo,          // project リポジトリを注入する
		harborCredentialRepo: harborCredentialRepo, // harbor credential リポジトリを注入する
		k8sClient:            k8sClient,            // k8s クライアントを注入する
	}
}

// TriggerBuild はデプロイメントのビルドをトリガーする
func (svc *buildServiceImpl) TriggerBuild(ctx context.Context, userID string, deploymentID string) (*models.DeploymentBuild, error) {
	// 1. Deployment を取得する
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
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

	// 5. Harbor 認証情報を取得する
	harborCredential, err := svc.harborCredentialRepo.FindByProjectIDNoTx(ctx, deploymentData.ProjectID) // harbor credential を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	// 6. DeploymentBuild レコードを pending で作成する
	buildData := &models.DeploymentBuild{
		DeploymentID:   deploymentID,                        // deployment ID を設定する
		BuildType:      buildType,                           // ビルドタイプを設定する
		Status:         models.BuildStatusPending,           // 初期ステータスを pending に設定する
		CommitSHA:      deploymentData.PendingGithubCommitSHA, // コミット SHA を設定する
		Branch:         deploymentData.PendingGithubBranch,  // ブランチを設定する
		Directory:      deploymentData.PendingGithubRepoDirectory, // ビルドディレクトリを設定する
		DockerfilePath: deploymentData.PendingDockerfilePath, // Dockerfile パスを設定する
	}
	if err := svc.buildRepo.Create(ctx, buildData); err != nil { // build レコードを作成する
		return nil, err // 作成エラーを返す
	}

	// 7. ビルドタイプに応じた k8s Job を起動する
	var jobName string
	switch buildData.BuildType {
	case models.BuildTypeDockerfile:
		jobName, err = k8s.CreateBuildJob( // dockerfile ビルダーは ISSUE-051 で実装予定
			ctx,
			svc.k8sClient,
			buildData.ID,        // ビルド ID を渡す
			projectData.Namespace, // namespace を指定する
		)
	case models.BuildTypeRailpack:
		railpackClient, railpackErr := railpack.New(svc.k8sClient, railpack.BuildConfig{ // railpack クライアントを生成する
			JobID:            buildData.ID,                      // ビルド ID をジョブ ID に使う
			GitRepo:          deploymentData.PendingGithubRepoURL, // Git リポジトリ URL を設定する
			GitBranch:        buildData.Branch,                  // ブランチを設定する
			Subdir:           buildData.Directory,               // ビルドサブディレクトリを設定する
			ImageName:        deploymentData.Name,               // イメージ名にデプロイメント名を使う
			ImageTag:         buildData.ID,                      // タグにビルド ID を使う
			RegistryHost:     harborCredential.HarborEndpoint,   // Harbor ホストを設定する
			RegistryProject:  projectData.Name,                  // Harbor プロジェクト名を設定する
			RegistryUsername: harborCredential.RobotName,        // robot アカウント名を設定する
			RegistryPassword: harborCredential.RobotSecret,      // robot シークレットを設定する
			Namespace:        projectData.Namespace,             // namespace を設定する
		})
		if railpackErr != nil {
			return nil, railpackErr // クライアント生成エラーを返す
		}
		railpackJobID, buildErr := railpackClient.Build(ctx) // ビルドジョブを起動する
		jobName = "railpack-" + railpackJobID                // railpack の命名規則に合わせる
		err = buildErr
	default:
		return nil, fmt.Errorf("未知のビルドタイプ: %s", buildData.BuildType) // 未知のビルドタイプはエラーを返す
	}
	if err != nil {
		return nil, err // Job 作成エラーを返す
	}

	// 8. k8s Job 名をビルドレコードに保存する
	buildData.K8sJobName = jobName                                // Job 名を設定する
	if err := svc.buildRepo.UpdateK8sJobName(ctx, buildData.ID, jobName); err != nil { // Job 名を DB に保存する
		return nil, err // 保存エラーを返す
	}

	return buildData, nil // 作成したビルドレコードを返す
}

// CancelBuild は実行中のビルドをキャンセルする
func (svc *buildServiceImpl) CancelBuild(ctx context.Context, userID string, buildID string) error {
	// 1. ビルドレコードを取得する
	buildData, err := svc.buildRepo.FindByID(ctx, buildID) // ビルドレコードを取得する
	if err != nil {
		return err // 取得エラーを返す
	}

	// 2. 所有権チェック（Build → Deployment → Project → UserID を辿る）
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, buildData.DeploymentID) // deployment を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return ErrForbidden
	}

	// 3. キャンセル可能なステータスか確認する（pending / building のみキャンセル可能）
	if buildData.Status != models.BuildStatusPending && buildData.Status != models.BuildStatusBuilding { // 完了済み・失敗済みはキャンセル不可
		return ErrBuildNotCancellable
	}

	// 4. k8s Job を削除する（Job 名が設定されている場合のみ）
	if buildData.K8sJobName != "" { // Job 名が空の場合は pending 状態で Job 未作成なのでスキップする
		if err := k8s.DeleteBuildJob(ctx, svc.k8sClient, projectData.Namespace, buildData.K8sJobName); err != nil { // k8s Job を削除する
			return fmt.Errorf("k8s Job の削除に失敗しました: %w", err) // 削除エラーを返す
		}
	}

	// 5. ビルドステータスを cancelled に更新する
	if err := svc.buildRepo.UpdateStatus(ctx, buildID, models.BuildStatusCancelled); err != nil { // ステータスを更新する
		return err // 更新エラーを返す
	}

	return nil // キャンセル成功
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
