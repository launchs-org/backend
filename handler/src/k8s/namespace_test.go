package k8s

import (
	"handler/models"
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"gorm.io/gorm"
)

// TestToNamespaceName_変換が正しく行われる は ToNamespaceName の変換結果を確認する
func TestToNamespaceName_変換が正しく行われる(t *testing.T) {
	testCases := []struct {
		input    string // 入力値
		expected string // 期待する変換結果
	}{
		{input: "MyProject", expected: "myproject"},                        // 大文字を小文字に変換する
		{input: "my_project", expected: "my-project"},                      // アンダーバーをハイフンに変換する
		{input: "my project", expected: "my-project"},                      // スペースをハイフンに変換する
		{input: "My_Project_123", expected: "my-project-123"},              // 複合ケース
		{input: "-leading-trailing-", expected: "leading-trailing"},        // 先頭・末尾のハイフンを除去する
		{input: "already-valid", expected: "already-valid"},                // 変換不要なケース
	}

	for _, testCase := range testCases {
		actualResult := ToNamespaceName(testCase.input) // 変換関数を実行する
		if actualResult != testCase.expected {
			t.Errorf("入力: %q, 期待: %q, 実際: %q", testCase.input, testCase.expected, actualResult)
		}
	}
}

// TestCreateNamespace_正常にnamespaceが作成される は CreateNamespace で k8s に namespace が作られることを確認する
func TestCreateNamespace_正常にnamespaceが作成される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	err := CreateNamespace(ctx, fakeClient, "test-namespace") // namespace を作成する
	if err != nil {
		t.Fatalf("CreateNamespace() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	namespaceObj, err := fakeClient.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{}) // 作成した namespace を取得する
	if err != nil {
		t.Fatalf("namespace の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if namespaceObj.Name != "test-namespace" { // namespace 名を確認する
		t.Errorf("期待する namespace 名: test-namespace, 実際の名前: %s", namespaceObj.Name)
	}
	if namespaceObj.Labels["launchs.org/managed"] != "true" { // ラベルが付与されていることを確認する
		t.Errorf("launchs.org/managed ラベルが設定されていません")
	}
}

// TestDeleteNamespace_正常にnamespaceが削除される は DeleteNamespace で k8s から namespace が削除されることを確認する
func TestDeleteNamespace_正常にnamespaceが削除される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	err := CreateNamespace(ctx, fakeClient, "test-namespace") // 削除対象の namespace を作成する
	if err != nil {
		t.Fatalf("事前の CreateNamespace() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	err = DeleteNamespace(ctx, fakeClient, "test-namespace") // namespace を削除する
	if err != nil {
		t.Fatalf("DeleteNamespace() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	_, err = fakeClient.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{}) // 削除後に取得を試みる
	if err == nil {
		t.Fatal("削除後も namespace が存在しています") // 削除後に取得できた場合はテスト失敗とする
	}
}

// TestCreateNamespace_同名namespaceを2回作成するとエラーになる は重複作成時にエラーが返ることを確認する
func TestCreateNamespace_同名namespaceを2回作成するとエラーになる(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	err := CreateNamespace(ctx, fakeClient, "duplicate-namespace") // 1回目の作成
	if err != nil {
		t.Fatalf("1回目の CreateNamespace() がエラーを返しました: %v", err) // 1回目は成功するべきなのでテスト失敗とする
	}

	err = CreateNamespace(ctx, fakeClient, "duplicate-namespace") // 2回目の同名作成
	if err == nil {
		t.Fatal("同名 namespace の2回目作成はエラーを返すべきです") // エラーが返らない場合はテスト失敗とする
	}
}

// mockProjectRepositoryForNamespace は WatchNamespaces テスト用の ProjectRepository モック
type mockProjectRepositoryForNamespace struct {
	findByNamespaceFunc func(ctx context.Context, namespace string) (*models.Project, error) // FindByNamespace のモック関数
	deleteNoTxFunc      func(ctx context.Context, project *models.Project) error             // DeleteNoTx のモック関数
	deleteNoTxCalled    bool                                                                  // DeleteNoTx が呼ばれたかどうかを記録する
}

func (mock *mockProjectRepositoryForNamespace) Create(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) FindByID(ctx context.Context, tx *gorm.DB, projectID string) (*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) FindByIDNoTx(ctx context.Context, projectID string) (*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) FindByNamespace(ctx context.Context, namespace string) (*models.Project, error) {
	if mock.findByNamespaceFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByNamespaceFunc(ctx, namespace)
	}
	return &models.Project{ID: "test-project-id", Namespace: namespace}, nil // デフォルトは対応する Project を返す
}

func (mock *mockProjectRepositoryForNamespace) FindAllByUserID(ctx context.Context, userID string) ([]*models.Project, error) {
	return nil, nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) UpdateStatus(ctx context.Context, tx *gorm.DB, project *models.Project, status models.ProjectStatus) error {
	return nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) Save(ctx context.Context, project *models.Project) error {
	return nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) Delete(ctx context.Context, tx *gorm.DB, project *models.Project) error {
	return nil // 使用しない
}

func (mock *mockProjectRepositoryForNamespace) DeleteNoTx(ctx context.Context, project *models.Project) error {
	mock.deleteNoTxCalled = true // 呼ばれたことを記録する
	if mock.deleteNoTxFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteNoTxFunc(ctx, project)
	}
	return nil // デフォルトは正常終了する
}

// TestHandleNamespaceEvent_Deletedイベントで対応するProjectが削除される は status が deleting の Project に Deleted イベントが来た場合に DB レコードが削除されることを確認する
func TestHandleNamespaceEvent_Deletedイベントで対応するProjectが削除される(t *testing.T) {
	projectRepo := &mockProjectRepositoryForNamespace{ // モック repository を生成する
		findByNamespaceFunc: func(ctx context.Context, namespace string) (*models.Project, error) {
			return &models.Project{ID: "test-project-id", Namespace: namespace, Status: models.ProjectStatusDeleting}, nil // status=deleting の Project を返す
		},
	}
	ctx := context.Background() // テスト用コンテキストを生成する

	namespaceObj := &corev1.Namespace{ // テスト用 Namespace オブジェクトを生成する
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",            // Namespace 名を設定する
			Labels: map[string]string{
				"launchs.org/managed": "true", // 管理ラベルを設定する
			},
		},
	}

	event := watch.Event{ // Deleted イベントを生成する
		Type:   watch.Deleted, // イベントタイプを Deleted に設定する
		Object: namespaceObj,  // Namespace オブジェクトを設定する
	}

	handleNamespaceEvent(ctx, event, projectRepo) // イベントを処理する

	if !projectRepo.deleteNoTxCalled { // DeleteNoTx が呼ばれていることを確認する
		t.Fatal("status=deleting の Project に Deleted イベントが来た場合に DeleteNoTx が呼ばれていません")
	}
}

// TestHandleNamespaceEvent_status_が_deleting_以外の場合はProjectが削除されない は status が deleting 以外の Project に Deleted イベントが来ても DB レコードが削除されないことを確認する
func TestHandleNamespaceEvent_status_が_deleting_以外の場合はProjectが削除されない(t *testing.T) {
	nonDeletingStatuses := []models.ProjectStatus{ // deleting 以外のステータス一覧
		models.ProjectStatusProvisioning, // provisioning ステータス
		models.ProjectStatusActive,       // active ステータス
	}

	for _, projectStatus := range nonDeletingStatuses { // 各ステータスでテストする
		projectRepo := &mockProjectRepositoryForNamespace{ // モック repository を生成する
			findByNamespaceFunc: func(ctx context.Context, namespace string) (*models.Project, error) {
				return &models.Project{ID: "test-project-id", Namespace: namespace, Status: projectStatus}, nil // 対象ステータスの Project を返す
			},
		}
		ctx := context.Background() // テスト用コンテキストを生成する

		namespaceObj := &corev1.Namespace{ // テスト用 Namespace オブジェクトを生成する
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace", // Namespace 名を設定する
			},
		}

		event := watch.Event{ // Deleted イベントを生成する
			Type:   watch.Deleted, // イベントタイプを Deleted に設定する
			Object: namespaceObj,  // Namespace オブジェクトを設定する
		}

		handleNamespaceEvent(ctx, event, projectRepo) // イベントを処理する

		if projectRepo.deleteNoTxCalled { // DeleteNoTx が呼ばれていないことを確認する
			t.Fatalf("status=%v の Project に Deleted イベントが来た場合に DeleteNoTx が誤って呼ばれました", projectStatus)
		}
	}
}

// TestHandleNamespaceEvent_Deleted以外のイベントではProjectが削除されない は Added/Modified イベントで DB が変更されないことを確認する
func TestHandleNamespaceEvent_Deleted以外のイベントではProjectが削除されない(t *testing.T) {
	projectRepo := &mockProjectRepositoryForNamespace{} // モック repository を生成する
	ctx := context.Background()                         // テスト用コンテキストを生成する

	namespaceObj := &corev1.Namespace{ // テスト用 Namespace オブジェクトを生成する
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace", // Namespace 名を設定する
		},
	}

	for _, eventType := range []watch.EventType{watch.Added, watch.Modified} { // Added/Modified イベントをテストする
		event := watch.Event{ // イベントを生成する
			Type:   eventType,   // イベントタイプを設定する
			Object: namespaceObj, // Namespace オブジェクトを設定する
		}

		handleNamespaceEvent(ctx, event, projectRepo) // イベントを処理する

		if projectRepo.deleteNoTxCalled { // DeleteNoTx が呼ばれていないことを確認する
			t.Fatalf("イベントタイプ %v で DeleteNoTx が誤って呼ばれました", eventType)
		}
	}
}

// TestHandleNamespaceEvent_FindByNamespaceがエラーを返す場合はDeleteが呼ばれない は Project 取得失敗時に削除が行われないことを確認する
func TestHandleNamespaceEvent_FindByNamespaceがエラーを返す場合はDeleteが呼ばれない(t *testing.T) {
	projectRepo := &mockProjectRepositoryForNamespace{ // モック repository を生成する
		findByNamespaceFunc: func(ctx context.Context, namespace string) (*models.Project, error) {
			return nil, errors.New("project not found") // エラーを返す
		},
	}
	ctx := context.Background() // テスト用コンテキストを生成する

	namespaceObj := &corev1.Namespace{ // テスト用 Namespace オブジェクトを生成する
		ObjectMeta: metav1.ObjectMeta{
			Name: "unknown-namespace", // 存在しない Namespace 名を設定する
		},
	}

	event := watch.Event{ // Deleted イベントを生成する
		Type:   watch.Deleted,   // イベントタイプを Deleted に設定する
		Object: namespaceObj,    // Namespace オブジェクトを設定する
	}

	handleNamespaceEvent(ctx, event, projectRepo) // イベントを処理する

	if projectRepo.deleteNoTxCalled { // DeleteNoTx が呼ばれていないことを確認する
		t.Fatal("FindByNamespace がエラーを返した場合に DeleteNoTx が呼ばれるべきではありません")
	}
}
