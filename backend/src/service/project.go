package service

import (
	"backend/database" // データベースパッケージ
	"backend/model"    // モデルパッケージ
	"context"         // コンテキスト
	"errors"          // エラー処理
	"fmt"             // 文字列フォーマット
	"regexp"          // 正規表現

	"github.com/google/uuid" // UUID生成
	corev1 "k8s.io/api/core/v1" // K8s API
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // K8s Meta API
)

// プロジェクト名のバリデーション用正規表現 (英小文字、数字、ハイフンのみ)
var projectNameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

var (
	// 不正なプロジェクト名のエラー
	ErrInvalidProjectName   = errors.New("INVALID_PROJECT_NAME")
	// プロジェクト名重複のエラー
	ErrProjectAlreadyExists = errors.New("PROJECT_ALREADY_EXISTS")
)

// CreateProjectInput はプロジェクト作成の入力データです
type CreateProjectInput struct {
	Name    string // プロジェクト名
	OwnerID string // 所有者ID
}

// CreateProject はプロジェクトを作成するビジネスロジックを実行します
func CreateProject(ctx context.Context, input CreateProjectInput) (*model.Project, error) {
	// プロジェクト名のバリデーションを実行 (空文字チェックと正規表現チェック)
	if input.Name == "" || !projectNameRegex.MatchString(input.Name) {
		// 不正な名前の場合はエラーを返す
		return nil, ErrInvalidProjectName
	}

	// データベースから同名のプロジェクトが存在するか確認
	existing, _ := model.GetProjectByName(input.Name)
	// 既に存在する場合
	if existing != nil {
		// 重複エラーを返す
		return nil, ErrProjectAlreadyExists
	}

	// 新しいプロジェクトIDをUUIDで生成
	projectID := uuid.New().String()
	// K8sで使用するNamespace名を決定 (ns-{uuid})
	namespace := fmt.Sprintf("ns-%s", projectID)

	// プロジェクトエンティティを作成
	project := &model.Project{
		ID:              projectID,      // ID
		Name:            input.Name,      // プロジェクト名
		K8sResourceName: input.Name,      // K8s用リソース名
		Namespace:       namespace,       // Namespace
		OwnerID:         input.OwnerID,   // 所有者ID
	}

	// Kubernetes Namespace の定義を作成
	nsSpec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace, // Namespace名
			Labels: map[string]string{
				"managed-by": "launchs",    // 管理者ラベル
				"project-id": projectID,   // プロジェクトIDラベル
			},
		},
	}
	// Kubernetes API を呼び出して Namespace を実際に作成
	_, err := database.K8sClientset.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	// K8sリソース作成に失敗した場合
	if err != nil {
		// エラーをラップして返す
		return nil, fmt.Errorf("Kubernetes Namespace の作成に失敗しました: %w", err)
	}

	// データベースにプロジェクト情報を保存
	if err := model.CreateProject(project); err != nil {
		// 保存に失敗した場合はエラーを返す
		return nil, fmt.Errorf("プロジェクトの保存に失敗しました: %w", err)
	}

	// 作成したプロジェクトを返す
	return project, nil
}
