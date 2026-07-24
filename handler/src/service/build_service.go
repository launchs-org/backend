package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	archivecrypto "handler/crypto"
	"handler/fileio"
	"handler/logger"
	"handler/middlewares"
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

// ErrInvalidArchiveType はアップロードされたファイルがzip/tar.gz形式ではない場合のエラー
var ErrInvalidArchiveType = errors.New("zip または tar.gz 形式のみサポートしています")

// ErrArchiveUploadFailed は一時ファイル共有サービス(litterbox)へのアップロードに失敗した場合のエラー
var ErrArchiveUploadFailed = errors.New("アーカイブのアップロードに失敗しました")

// ErrDeploymentTypeMismatch はデプロイメントタイプがリクエストと一致しない場合のエラー
var ErrDeploymentTypeMismatch = errors.New("このデプロイメントタイプでは利用できません")

// ErrArchiveTokenInvalid はアーカイブアップロードトークンが無効・期限切れの場合のエラー
var ErrArchiveTokenInvalid = errors.New("アーカイブアップロードトークンが無効です")

// ErrArchiveTokenDeploymentMismatch はアーカイブアップロードトークンの対象デプロイメントが一致しない場合のエラー
var ErrArchiveTokenDeploymentMismatch = errors.New("アーカイブアップロードトークンの対象デプロイメントが一致しません")

// BuildService はビルドトリガーのビジネスロジックを定義するインターフェース
type BuildService interface {
	TriggerBuild(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) // ビルドをトリガーする
	UploadBuildArchive(ctx context.Context, userID string, deploymentID string, fileName string, fileReader io.Reader, fileSize int64) (string, error) // アーカイブをアップロードしトークンを返す
	CancelBuild(ctx context.Context, userID string, buildID string) error                                          // ビルドをキャンセルする
	GetBuild(ctx context.Context, userID string, buildID string) (*models.DeploymentBuild, error)                  // ビルド情報を取得する
	GetBuildLogs(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error) // ビルドログを取得する（ログ文字列・最終チャンク時刻・エラー）
	ListBuilds(ctx context.Context, userID string, deploymentID string) ([]models.DeploymentBuild, error)          // ビルド一覧を取得する（deployment 単位）
	ListBuildsByProject(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error)    // ビルド一覧を取得する（project 単位）
}

// TriggerBuildRequest は TriggerBuild のリクエスト構造体（現時点では deploymentID のみ）

// BuildWorkflowInput は BuildWorkflow への入力（builder worker のタスクキューに送信する）
type BuildWorkflowInput struct {
	BuildID string // 対象ビルドの ID（あらかじめ DB に pending 状態で作成済み）

	// --- archive タイプのビルドでのみ使用（railpack/dockerfile タイプでは空）---
	ArchiveDownloadURL string // 一時ファイル共有サービス(litterbox) のダウンロードリンク
	ArchiveEncKeyHex   string // AES-256-CBC復号鍵（16進）
	ArchiveSHA256Hex   string // 暗号文全体のSHA256ハッシュ（16進、破損・改竄検知用）
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
	registryHost         string                                // ビルドジョブが使う Harbor ホスト名（スキームなし）
	temporalClient       WorkflowStarter                      // Temporal クライアント（Workflow 起動用）
	fileIOClient         *fileio.FileIOClient                 // 一時ファイル共有サービス(litterbox) アップロードクライアント
}

// NewBuildService は BuildService の実装を返す
func NewBuildService(
	deploymentRepo repository.DeploymentRepository,
	buildRepo repository.DeploymentBuildRepository,
	projectRepo repository.ProjectRepository,
	harborCredentialRepo repository.HarborCredentialRepository,
	logChunkRepo repository.BuildLogChunkRepository,
	k8sClient kubernetes.Interface,
	registryHost string,
	temporalClient WorkflowStarter,
	fileIOClient *fileio.FileIOClient,
) BuildService {
	return &buildServiceImpl{
		deploymentRepo:       deploymentRepo,       // deployment リポジトリを注入する
		buildRepo:            buildRepo,            // build リポジトリを注入する
		projectRepo:          projectRepo,          // project リポジトリを注入する
		harborCredentialRepo: harborCredentialRepo, // harbor credential リポジトリを注入する
		logChunkRepo:         logChunkRepo,         // ログチャンクリポジトリを注入する
		k8sClient:            k8sClient,            // k8s クライアントを注入する
		registryHost:         registryHost,         // クラスタ内 DNS 名を注入する
		temporalClient:       temporalClient,       // Temporal クライアントを注入する
		fileIOClient:         fileIOClient,         // 一時ファイル共有サービス(litterbox) アップロードクライアントを注入する
	}
}

// TriggerBuild はデプロイメントのビルドをトリガーする。
// archiveUploadToken が指定された場合はアーカイブアップロードトークンを検証し archive タイプのビルドとして処理する。
// 空文字の場合は既存の GitHub ビルドフローとして処理する。
func (svc *buildServiceImpl) TriggerBuild(ctx context.Context, userID string, deploymentID string, commitMessage string, author string, archiveUploadToken string, buildDirectory string) (*models.DeploymentBuild, error) {
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
	deploymentIDValue := deploymentID // pointer 用にローカル変数にコピーする
	buildData := &models.DeploymentBuild{
		ProjectID:     deploymentData.ProjectID, // project ID を設定する（Deployment 削除後もビルドを保持するため）
		DeploymentID:  &deploymentIDValue,       // deployment ID を設定する（nullable pointer）
		BuildType:     buildType,                // ビルドタイプを設定する
		Status:        models.BuildStatusPending, // 初期ステータスを pending に設定する
		CommitMessage: commitMessage,             // コミットメッセージを設定する
		Author:        author,                    // コミット著者を設定する
	}

	// 6. アーカイブアップロードトークンの有無でソースを分岐する
	var workflowInput BuildWorkflowInput
	if archiveUploadToken != "" { // archive タイプのビルド
		archiveClaim, tokenErr := middlewares.ValidateArchiveUploadToken(archiveUploadToken) // トークンを検証する
		if tokenErr != nil {
			return nil, ErrArchiveTokenInvalid // 無効・期限切れトークンのエラーを返す
		}
		if archiveClaim.DeploymentID != deploymentID { // トークンの誤流用を防ぐ
			return nil, ErrArchiveTokenDeploymentMismatch
		}

		buildData.Directory = buildDirectory              // ビルドディレクトリを設定する
		buildData.ArchiveFileName = archiveClaim.FileName  // アーカイブファイル名を設定する
		buildData.ArchiveSizeBytes = archiveClaim.SizeBytes // アーカイブサイズを設定する

		workflowInput.ArchiveDownloadURL = archiveClaim.ArchiveURL // 一時ファイル共有サービス(litterbox) のダウンロードリンクを設定する
		workflowInput.ArchiveEncKeyHex = archiveClaim.EncKeyHex     // 復号鍵を設定する
		workflowInput.ArchiveSHA256Hex = archiveClaim.SHA256Hex     // SHA256ハッシュを設定する
	} else { // 既存の GitHub ビルドフロー
		buildData.GithubRepoURL = deploymentData.PendingGithubRepoURL       // GitHub リポジトリ URL をスナップショットする
		buildData.CommitSHA = deploymentData.PendingGithubCommitSHA         // コミット SHA を設定する
		buildData.Branch = deploymentData.PendingGithubBranch               // ブランチを設定する
		buildData.Directory = deploymentData.PendingGithubRepoDirectory     // ビルドディレクトリを設定する
		buildData.DockerfilePath = deploymentData.PendingDockerfilePath     // Dockerfile パスを設定する
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
	workflowInput.BuildID = buildData.ID                                                                    // Workflow 入力にビルド ID を設定する
	_, startErr := svc.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "BuildWorkflow", workflowInput) // Workflow を起動する
	if startErr != nil {
		// Workflow 起動失敗時はビルドレコードを failed に更新してから返す（pending のまま残すと再ビルドが永久に弾かれるため）
		finishedAt := time.Now()
		_ = svc.buildRepo.UpdateBuildResult(ctx, buildData.ID, models.BuildStatusFailed, finishedAt) // ロールバック：failed に更新する
		return nil, fmt.Errorf("ビルド workflow の起動に失敗しました: %w", startErr)                              // 起動エラーを返す
	}

	return buildData, nil // 作成したビルドレコードを返す
}

// archiveMagicBytes はzip/tar.gz形式を識別するためのマジックバイト
var archiveMagicBytesZip = []byte{0x50, 0x4b, 0x03, 0x04}    // "PK\x03\x04"
var archiveMagicBytesGzip = []byte{0x1f, 0x8b}                // gzip マジックバイト

// UploadBuildArchive はzip/tar.gzアーカイブを暗号化して一時ファイル共有サービス(litterbox)にアップロードし、
// アップロードトークン(JWT)を発行して返す。この時点ではビルドは開始しない。
func (svc *buildServiceImpl) UploadBuildArchive(ctx context.Context, userID string, deploymentID string, fileName string, fileReader io.Reader, fileSize int64) (string, error) {
	// 1. Deployment を取得しタイプを検証する
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return "", err // 取得エラーを返す
	}
	if deploymentData.Type != models.DeploymentTypeArchive { // archive タイプのみ許可する
		return "", ErrDeploymentTypeMismatch
	}

	// 2. 所有権チェック
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return "", err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return "", ErrForbidden
	}

	// 3. ファイルを読み込み、拡張子/マジックバイトを検証する
	plainData, err := io.ReadAll(fileReader) // アーカイブ本体を読み込む
	if err != nil {
		return "", fmt.Errorf("アーカイブの読み込みに失敗しました: %w", err) // 読み込みエラーを返す
	}
	if !isValidArchiveContent(plainData) { // zip/tar.gzのマジックバイトを確認する
		return "", ErrInvalidArchiveType
	}

	// 4. 暗号化する
	encoded, encKeyHex, sha256Hex, err := archivecrypto.EncryptArchive(plainData) // AES-256-CBCで暗号化する
	if err != nil {
		return "", fmt.Errorf("アーカイブの暗号化に失敗しました: %w", err) // 暗号化エラーを返す
	}

	// 5. 一時ファイル共有サービス(litterbox) にアップロードする
	downloadURL, err := svc.fileIOClient.Upload(ctx, fileName, strings.NewReader(string(encoded)), int64(len(encoded))) // 一時ファイル共有サービス(litterbox) にアップロードする
	if err != nil {
		logger.PrintErr(fmt.Sprintf("一時ファイル共有サービス(litterbox) へのアーカイブアップロードに失敗しました（deploymentID=%s, fileName=%s）: %v", deploymentID, fileName, err)) // 実際の失敗原因（HTTPステータスやレスポンス内容）をログに残す
		return "", ErrArchiveUploadFailed
	}

	// 6. アップロードトークンを発行する
	claim := middlewares.ArchiveUploadTokenClaim{
		DeploymentID: deploymentID, // 対象デプロイメントIDを設定する
		ArchiveURL:   downloadURL,  // 一時ファイル共有サービス(litterbox) のダウンロードリンクを設定する
		EncKeyHex:    encKeyHex,    // 復号鍵を設定する
		SHA256Hex:    sha256Hex,    // SHA256ハッシュを設定する
		FileName:     fileName,     // 元ファイル名を設定する
		SizeBytes:    fileSize,     // 元サイズを設定する
	}
	uploadToken, err := middlewares.IssueArchiveUploadToken(claim) // トークンを発行する
	if err != nil {
		return "", fmt.Errorf("アップロードトークンの発行に失敗しました: %w", err) // 発行エラーを返す
	}

	return uploadToken, nil // トークン文字列を返す
}

// isValidArchiveContent はバイト列がzipまたはtar.gzのマジックバイトで始まるか判定する
func isValidArchiveContent(data []byte) bool {
	if len(data) >= len(archiveMagicBytesZip) && string(data[:len(archiveMagicBytesZip)]) == string(archiveMagicBytesZip) {
		return true // zip形式
	}
	if len(data) >= len(archiveMagicBytesGzip) && string(data[:len(archiveMagicBytesGzip)]) == string(archiveMagicBytesGzip) {
		return true // gzip形式（tar.gz）
	}
	return false
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

// resolveBuildType は DeploymentType からビルドタイプを解決する
func resolveBuildType(deploymentType models.DeploymentType) (models.BuildType, error) {
	switch deploymentType {
	case models.DeploymentTypeDockerfile:
		return models.BuildTypeDockerfile, nil // dockerfile タイプを返す
	case models.DeploymentTypeRailpack:
		return models.BuildTypeRailpack, nil // railpack タイプを返す
	case models.DeploymentTypeArchive:
		return models.BuildTypeRailpack, nil // archive タイプもRailpackベースの成果物のためBuildTypeRailpackを返す
	default:
		return "", fmt.Errorf("ビルドをサポートしないデプロイメントタイプ: %s", deploymentType) // 非対応タイプはエラーを返す
	}
}
