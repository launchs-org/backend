package service

import (
	"app/models"
	"app/repository"
	"context"
	"strings"
	"time"
)

// GetPodLogsResult は GetPodLogs の返却値
type GetPodLogsResult struct {
	Logs          string     `json:"logs"`           // 結合したログ文字列
	LastTimestamp *time.Time `json:"last_timestamp"` // 最後のチャンクの CreatedAt（次回の since に使う）
}

// LogService は Pod ログ取得のビジネスロジックを定義するインターフェース
type LogService interface {
	GetPodLogs(ctx context.Context, userID string, deploymentID string, since *time.Time) (*GetPodLogsResult, error) // Pod ログを取得して結合した文字列を返す
}

// logServiceImpl は LogService の実装
type logServiceImpl struct {
	deploymentRepo  repository.DeploymentRepository  // deployment リポジトリ
	projectRepo     repository.ProjectRepository     // project リポジトリ
	podLogChunkRepo repository.PodLogChunkRepository // pod_log_chunk リポジトリ
}

// NewLogService は LogService の実装を返す
func NewLogService(deploymentRepo repository.DeploymentRepository, projectRepo repository.ProjectRepository, podLogChunkRepo repository.PodLogChunkRepository) LogService {
	return &logServiceImpl{
		deploymentRepo:  deploymentRepo,  // 依存を注入する
		projectRepo:     projectRepo,     // 依存を注入する
		podLogChunkRepo: podLogChunkRepo, // 依存を注入する
	}
}

// GetPodLogs は deploymentID に紐づく Pod ログを取得して結合した文字列を返す
func (svc *logServiceImpl) GetPodLogs(ctx context.Context, userID string, deploymentID string, since *time.Time) (*GetPodLogsResult, error) {
	// 1. Deployment レコードを取得する
	deploymentData, err := svc.deploymentRepo.FindByID(ctx, deploymentID) // deployment を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	// 2. 所有権チェック（Deployment → Project → UserID を辿る）
	projectData, err := svc.projectRepo.FindByIDNoTx(ctx, deploymentData.ProjectID) // project を取得する
	if err != nil {
		return nil, err // 取得エラーを返す
	}
	if projectData.UserID != userID { // UserID が一致しない場合は禁止エラーを返す
		return nil, ErrForbidden
	}

	// 3. ログチャンクを取得する（since 指定あり／なしで分岐する）
	var chunkList []models.PodLogChunk
	if since != nil { // since パラメータが指定されている場合は差分を取得する
		chunkList, err = svc.podLogChunkRepo.FindByDeploymentIDSince(ctx, deploymentID, *since) // since より後のチャンクを取得する
	} else {
		chunkList, err = svc.podLogChunkRepo.FindByDeploymentID(ctx, deploymentID) // 全チャンクを取得する
	}
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	// 4. チャンクを結合して返す
	var logBuilder strings.Builder                       // ログ文字列ビルダーを生成する
	for _, chunkData := range chunkList {                // 各チャンクを結合する
		logBuilder.WriteString(chunkData.Content)        // チャンクの内容を追記する
	}

	result := &GetPodLogsResult{ // 結果を生成する
		Logs: logBuilder.String(), // 結合したログ文字列を設定する
	}

	if len(chunkList) > 0 { // チャンクが1件以上ある場合は最後のチャンクの CreatedAt を設定する
		lastTimestamp := chunkList[len(chunkList)-1].CreatedAt // 最後のチャンクの CreatedAt を取得する
		result.LastTimestamp = &lastTimestamp                  // LastTimestamp を設定する
	}

	return result, nil // 結果を返す
}
