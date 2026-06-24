package service

import (
	"app/models"
	"app/repository"
	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodLogsEntry は1 Pod 分のログを表す
type PodLogsEntry struct {
	PodName       string     `json:"pod_name"`       // Pod 名
	Logs          string     `json:"logs"`           // 結合したログ文字列
	LastTimestamp *time.Time `json:"last_timestamp"` // 最後のチャンクの CreatedAt（次回の since に使う）
}

// GetPodLogsResult は GetPodLogs の返却値
type GetPodLogsResult struct {
	ActivePodNames []string       `json:"active_pod_names"` // k8s 上で現在稼働中の Pod 名一覧
	Pods           []PodLogsEntry `json:"pods"`             // Pod ごとのログ一覧
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
	k8sClient       kubernetes.Interface             // k8s クライアント（Pod 一覧取得に使う）
}

// NewLogService は LogService の実装を返す
func NewLogService(deploymentRepo repository.DeploymentRepository, projectRepo repository.ProjectRepository, podLogChunkRepo repository.PodLogChunkRepository, k8sClient kubernetes.Interface) LogService {
	return &logServiceImpl{
		deploymentRepo:  deploymentRepo,  // 依存を注入する
		projectRepo:     projectRepo,     // 依存を注入する
		podLogChunkRepo: podLogChunkRepo, // 依存を注入する
		k8sClient:       k8sClient,       // 依存を注入する
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

	// 3. k8s から現在稼働中の Pod 一覧を取得する
	k8sPodList, podListErr := svc.k8sClient.CoreV1().Pods(projectData.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=" + deploymentData.Name, // app ラベルで対象 Deployment の Pod を絞り込む
	}) // Pod 一覧を取得する
	activePodNames := make([]string, 0) // アクティブな Pod 名一覧を初期化する
	if podListErr == nil {              // 取得成功時のみ Pod 名を収集する
		for podIndex := range k8sPodList.Items { // 各 Pod 名を収集する
			activePodNames = append(activePodNames, k8sPodList.Items[podIndex].Name) // Pod 名を追加する
		}
	}

	// 4. ログチャンクを取得する（since 指定あり／なしで分岐する）
	var chunkList []models.PodLogChunk
	if since != nil { // since パラメータが指定されている場合は差分を取得する
		chunkList, err = svc.podLogChunkRepo.FindByDeploymentIDSince(ctx, deploymentID, *since) // since より後のチャンクを取得する
	} else {
		chunkList, err = svc.podLogChunkRepo.FindByDeploymentID(ctx, deploymentID) // 全チャンクを取得する
	}
	if err != nil {
		return nil, err // 取得エラーを返す
	}

	// 5. チャンクを Pod 名でグループ化して結合する
	podOrderList := make([]string, 0)                        // Pod 名の登場順を記録するスライスを生成する
	podChunkMap := make(map[string][]models.PodLogChunk)     // Pod 名をキーにしたチャンクマップを生成する
	for _, chunkData := range chunkList {                    // 各チャンクを Pod 名でグループ化する
		if _, exists := podChunkMap[chunkData.PodName]; !exists { // Pod 名が未登録の場合は順序リストに追加する
			podOrderList = append(podOrderList, chunkData.PodName) // 登場順を記録する
		}
		podChunkMap[chunkData.PodName] = append(podChunkMap[chunkData.PodName], chunkData) // Pod 名に対応するチャンクを追加する
	}

	podEntryList := make([]PodLogsEntry, 0, len(podOrderList)) // Pod ごとのログエントリ一覧を生成する
	for _, podName := range podOrderList {                     // 登場順に Pod ごとのエントリを生成する
		chunks := podChunkMap[podName]                         // 対応するチャンク一覧を取得する
		var logBuilder strings.Builder                         // ログ文字列ビルダーを生成する
		for _, chunkData := range chunks {                     // 各チャンクを結合する
			logBuilder.WriteString(chunkData.Content)          // チャンクの内容を追記する
		}
		entry := PodLogsEntry{ // Pod ログエントリを生成する
			PodName: podName,             // Pod 名を設定する
			Logs:    logBuilder.String(), // 結合したログ文字列を設定する
		}
		if len(chunks) > 0 { // チャンクが1件以上ある場合は最後のチャンクの CreatedAt を設定する
			lastTimestamp := chunks[len(chunks)-1].CreatedAt // 最後のチャンクの CreatedAt を取得する
			entry.LastTimestamp = &lastTimestamp             // LastTimestamp を設定する
		}
		podEntryList = append(podEntryList, entry) // エントリ一覧に追加する
	}

	result := &GetPodLogsResult{ // 結果を生成する
		ActivePodNames: activePodNames, // アクティブな Pod 名一覧を設定する
		Pods:           podEntryList,   // Pod ごとのログ一覧を設定する
	}

	return result, nil // 結果を返す
}
