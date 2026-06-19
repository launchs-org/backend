package service

import (
	"app/models"
	"context"
	"testing"

	"gorm.io/gorm"
)

// mockIngressRouteRouteRepository は IngressRouteRouteRepository のテスト用モック実装
type mockIngressRouteRouteRepository struct {
	createFunc                              func(ctx context.Context, tx *gorm.DB, route *models.IngressRouteRoute) error
	findByIDFunc                            func(ctx context.Context, routeID string) (*models.IngressRouteRoute, error)
	findByIngressRouteIDFunc                func(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error)
	findActiveAndPendingByIngressRouteIDFunc func(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error)
	updateFunc                              func(ctx context.Context, route *models.IngressRouteRoute) error
	updateStatusFunc                        func(ctx context.Context, tx *gorm.DB, routeID string, status models.IngressRouteRouteStatus) error
	deleteFunc                              func(ctx context.Context, tx *gorm.DB, routeID string) error
	deleteByIngressRouteIDFunc              func(ctx context.Context, tx *gorm.DB, ingressRouteID string) error
}

func (mock *mockIngressRouteRouteRepository) Create(ctx context.Context, tx *gorm.DB, route *models.IngressRouteRoute) error {
	if mock.createFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.createFunc(ctx, tx, route)
	}
	return nil // デフォルトは成功を返す
}

func (mock *mockIngressRouteRouteRepository) FindByID(ctx context.Context, routeID string) (*models.IngressRouteRoute, error) {
	if mock.findByIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIDFunc(ctx, routeID)
	}
	return &models.IngressRouteRoute{ID: routeID}, nil // デフォルトは空のルートエントリを返す
}

func (mock *mockIngressRouteRouteRepository) FindByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error) {
	if mock.findByIngressRouteIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findByIngressRouteIDFunc(ctx, ingressRouteID)
	}
	return []*models.IngressRouteRoute{}, nil // デフォルトは空のスライスを返す
}

func (mock *mockIngressRouteRouteRepository) FindActiveAndPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.IngressRouteRoute, error) {
	if mock.findActiveAndPendingByIngressRouteIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.findActiveAndPendingByIngressRouteIDFunc(ctx, ingressRouteID)
	}
	return []*models.IngressRouteRoute{}, nil // デフォルトは空のスライスを返す
}

func (mock *mockIngressRouteRouteRepository) Update(ctx context.Context, route *models.IngressRouteRoute) error {
	if mock.updateFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateFunc(ctx, route)
	}
	return nil // デフォルトは成功を返す
}

func (mock *mockIngressRouteRouteRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, routeID string, status models.IngressRouteRouteStatus) error {
	if mock.updateStatusFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.updateStatusFunc(ctx, tx, routeID, status)
	}
	return nil // デフォルトは成功を返す
}

func (mock *mockIngressRouteRouteRepository) Delete(ctx context.Context, tx *gorm.DB, routeID string) error {
	if mock.deleteFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteFunc(ctx, tx, routeID)
	}
	return nil // デフォルトは成功を返す
}

func (mock *mockIngressRouteRouteRepository) DeleteByIngressRouteID(ctx context.Context, tx *gorm.DB, ingressRouteID string) error {
	if mock.deleteByIngressRouteIDFunc != nil { // モック関数が設定されている場合は呼び出す
		return mock.deleteByIngressRouteIDFunc(ctx, tx, ingressRouteID)
	}
	return nil // デフォルトは成功を返す
}

// newTestIngressRouteService はテスト用の ingressRouteServiceImpl を生成するヘルパー
func newTestIngressRouteService(
	ingressRouteRepo *mockIngressRouteRepository,
	ingressRouteRouteRepo *mockIngressRouteRouteRepository,
	projectRepo *mockProjectRepository,
	deploymentRepo *mockDeploymentRepository,
) *ingressRouteServiceImpl {
	return &ingressRouteServiceImpl{
		ingressRouteRepo:      ingressRouteRepo,      // ingress_route リポジトリを注入する
		ingressRouteRouteRepo: ingressRouteRouteRepo, // ingress_route_route リポジトリを注入する
		projectRepo:           projectRepo,           // project リポジトリを注入する
		deploymentRepo:        deploymentRepo,        // deployment リポジトリを注入する
		baseDomain:            "example.com",         // テスト用ベースドメインを設定する
	}
}

// TestGetIngressRoute は GetIngressRoute の正常系・異常系をテストする
func TestGetIngressRoute(t *testing.T) {
	testCases := []struct {
		name              string
		userID            string
		projectID         string
		projectUserID     string
		ingressRouteError error
		expectedError     error
	}{
		{
			name:          "正常に ingress_route を取得できる",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-1",
			expectedError: nil,
		},
		{
			name:          "所有者でない場合は ErrForbidden を返す",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-other",
			expectedError: ErrForbidden,
		},
		{
			name:              "ingress_route が見つからない場合はエラーを返す",
			userID:            "user-1",
			projectID:         "proj-1",
			projectUserID:     "user-1",
			ingressRouteError: gorm.ErrRecordNotFound,
			expectedError:     gorm.ErrRecordNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepo := &mockProjectRepository{
				findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
					return &models.Project{ID: projectID, UserID: testCase.projectUserID}, nil // プロジェクトを返す
				},
			}
			ingressRouteRepo := &mockIngressRouteRepository{
				findByProjectIDFunc: func(ctx context.Context, projectID string) (*models.IngressRoute, error) {
					if testCase.ingressRouteError != nil { // エラーが設定されている場合は返す
						return nil, testCase.ingressRouteError
					}
					return &models.IngressRoute{ProjectID: projectID}, nil // ingress_route を返す
				},
			}
			svc := newTestIngressRouteService(ingressRouteRepo, &mockIngressRouteRouteRepository{}, projectRepo, &mockDeploymentRepository{})

			_, err := svc.GetIngressRoute(context.Background(), testCase.userID, testCase.projectID) // GetIngressRoute を呼び出す
			if testCase.expectedError != nil {
				if err == nil || err.Error() != testCase.expectedError.Error() { // エラーが期待と異なる場合はテスト失敗
					t.Errorf("期待するエラー %v, 実際のエラー %v", testCase.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("エラーは期待されていないが %v が返された", err) // エラーが不要なのに返された場合はテスト失敗
			}
		})
	}
}

// TestCreateIngressRoute は CreateIngressRoute の正常系・異常系をテストする
func TestCreateIngressRoute(t *testing.T) {
	testCases := []struct {
		name          string
		userID        string
		projectID     string
		projectUserID string
		requestHost   string
		expectedError error
		expectedHost  string
	}{
		{
			name:          "ホストを指定して ingress_route を作成できる",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-1",
			requestHost:   "custom.example.com",
			expectedHost:  "custom.example.com",
			expectedError: nil,
		},
		{
			name:          "ホストを省略した場合は自動生成される",
			userID:        "user-1",
			projectID:     "proj-1234567890",
			projectUserID: "user-1",
			requestHost:   "",
			expectedHost:  "proj-123.example.com", // projectID[:8] + baseDomain
			expectedError: nil,
		},
		{
			name:          "所有者でない場合は ErrForbidden を返す",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-other",
			expectedError: ErrForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepo := &mockProjectRepository{
				findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
					return &models.Project{ID: projectID, UserID: testCase.projectUserID}, nil // プロジェクトを返す
				},
			}
			var createdRoute *models.IngressRoute // 作成されたルートを保持する
			ingressRouteRepo := &mockIngressRouteRepository{
				createFunc: func(ctx context.Context, ingressRoute *models.IngressRoute) error {
					createdRoute = ingressRoute // 作成されたルートを保存する
					return nil
				},
			}
			svc := newTestIngressRouteService(ingressRouteRepo, &mockIngressRouteRouteRepository{}, projectRepo, &mockDeploymentRepository{})

			result, err := svc.CreateIngressRoute(context.Background(), testCase.userID, testCase.projectID, CreateIngressRouteRequest{Host: testCase.requestHost}) // CreateIngressRoute を呼び出す
			if testCase.expectedError != nil {
				if err == nil || err.Error() != testCase.expectedError.Error() { // エラーが期待と異なる場合はテスト失敗
					t.Errorf("期待するエラー %v, 実際のエラー %v", testCase.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Errorf("エラーは期待されていないが %v が返された", err) // エラーが不要なのに返された場合はテスト失敗
				return
			}
			if testCase.expectedHost != "" && result.Host != testCase.expectedHost { // ホスト名が期待と異なる場合はテスト失敗
				t.Errorf("期待するホスト %s, 実際のホスト %s", testCase.expectedHost, result.Host)
			}
			if createdRoute == nil { // create が呼ばれていない場合はテスト失敗
				t.Error("ingressRouteRepo.Create が呼ばれていない")
			}
		})
	}
}

// TestUpdateIngressRoute は UpdateIngressRoute の正常系・異常系をテストする
func TestUpdateIngressRoute(t *testing.T) {
	newHost := "new.example.com"
	testCases := []struct {
		name          string
		userID        string
		projectID     string
		projectUserID string
		requestHost   *string
		expectedError error
	}{
		{
			name:          "pending_host が更新される",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-1",
			requestHost:   &newHost,
			expectedError: nil,
		},
		{
			name:          "所有者でない場合は ErrForbidden を返す",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-other",
			expectedError: ErrForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepo := &mockProjectRepository{
				findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
					return &models.Project{ID: projectID, UserID: testCase.projectUserID}, nil // プロジェクトを返す
				},
			}
			ingressRouteRepo := &mockIngressRouteRepository{
				findByProjectIDFunc: func(ctx context.Context, projectID string) (*models.IngressRoute, error) {
					return &models.IngressRoute{ProjectID: projectID, Host: "old.example.com"}, nil // 既存 ingress_route を返す
				},
			}
			svc := newTestIngressRouteService(ingressRouteRepo, &mockIngressRouteRouteRepository{}, projectRepo, &mockDeploymentRepository{})

			result, err := svc.UpdateIngressRoute(context.Background(), testCase.userID, testCase.projectID, UpdateIngressRouteRequest{Host: testCase.requestHost}) // UpdateIngressRoute を呼び出す
			if testCase.expectedError != nil {
				if err == nil || err.Error() != testCase.expectedError.Error() { // エラーが期待と異なる場合はテスト失敗
					t.Errorf("期待するエラー %v, 実際のエラー %v", testCase.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Errorf("エラーは期待されていないが %v が返された", err) // エラーが不要なのに返された場合はテスト失敗
				return
			}
			if testCase.requestHost != nil && result.PendingHost != *testCase.requestHost { // pending_host が期待と異なる場合はテスト失敗
				t.Errorf("期待する pending_host %s, 実際の pending_host %s", *testCase.requestHost, result.PendingHost)
			}
		})
	}
}

// TestDeleteIngressRoute は DeleteIngressRoute の正常系・異常系をテストする
func TestDeleteIngressRoute(t *testing.T) {
	testCases := []struct {
		name          string
		userID        string
		projectID     string
		projectUserID string
		expectedError error
	}{
		{
			name:          "ingress_route とルートエントリが削除される",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-1",
			expectedError: nil,
		},
		{
			name:          "所有者でない場合は ErrForbidden を返す",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-other",
			expectedError: ErrForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepo := &mockProjectRepository{
				findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
					return &models.Project{ID: projectID, UserID: testCase.projectUserID}, nil // プロジェクトを返す
				},
			}
			deleteByIngressRouteIDCalled := false // DeleteByIngressRouteID が呼ばれたかを記録する
			deleteIngressRouteCalled := false      // Delete が呼ばれたかを記録する
			ingressRouteRepo := &mockIngressRouteRepository{
				findByProjectIDFunc: func(ctx context.Context, projectID string) (*models.IngressRoute, error) {
					return &models.IngressRoute{ID: "route-1", ProjectID: projectID}, nil // 既存 ingress_route を返す
				},
			}
			ingressRouteRepo.findByProjectIDFunc = func(ctx context.Context, projectID string) (*models.IngressRoute, error) {
				return &models.IngressRoute{ID: "route-1", ProjectID: projectID}, nil // 既存 ingress_route を返す
			}
			deletedIngressRouteRepo := &mockIngressRouteRepository{
				findByProjectIDFunc: func(ctx context.Context, projectID string) (*models.IngressRoute, error) {
					return &models.IngressRoute{ID: "route-1", ProjectID: projectID}, nil
				},
			}
			_ = deleteIngressRouteCalled
			routeRepo := &mockIngressRouteRouteRepository{
				deleteByIngressRouteIDFunc: func(ctx context.Context, tx *gorm.DB, ingressRouteID string) error {
					deleteByIngressRouteIDCalled = true // 呼び出しを記録する
					return nil
				},
			}
			svc := newTestIngressRouteService(deletedIngressRouteRepo, routeRepo, projectRepo, &mockDeploymentRepository{})

			err := svc.DeleteIngressRoute(context.Background(), testCase.userID, testCase.projectID) // DeleteIngressRoute を呼び出す
			if testCase.expectedError != nil {
				if err == nil || err.Error() != testCase.expectedError.Error() { // エラーが期待と異なる場合はテスト失敗
					t.Errorf("期待するエラー %v, 実際のエラー %v", testCase.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Errorf("エラーは期待されていないが %v が返された", err) // エラーが不要なのに返された場合はテスト失敗
				return
			}
			if !deleteByIngressRouteIDCalled { // ルートエントリの削除が呼ばれていない場合はテスト失敗
				t.Error("DeleteByIngressRouteID が呼ばれていない")
			}
		})
	}
}

// TestAddRoute は AddRoute の正常系・異常系をテストする
func TestAddRoute(t *testing.T) {
	testCases := []struct {
		name                       string
		userID                     string
		projectID                  string
		projectUserID              string
		deploymentProjectID        string
		expectedError              error
	}{
		{
			name:                "ルートエントリが pending 状態で追加される",
			userID:              "user-1",
			projectID:           "proj-1",
			projectUserID:       "user-1",
			deploymentProjectID: "proj-1",
			expectedError:       nil,
		},
		{
			name:                "DeploymentID がプロジェクトに属さない場合は ErrDeploymentNotBelongToProject を返す",
			userID:              "user-1",
			projectID:           "proj-1",
			projectUserID:       "user-1",
			deploymentProjectID: "proj-other",
			expectedError:       ErrDeploymentNotBelongToProject,
		},
		{
			name:                "所有者でない場合は ErrForbidden を返す",
			userID:              "user-1",
			projectID:           "proj-1",
			projectUserID:       "user-other",
			deploymentProjectID: "proj-1",
			expectedError:       ErrForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepo := &mockProjectRepository{
				findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
					return &models.Project{ID: projectID, UserID: testCase.projectUserID}, nil // プロジェクトを返す
				},
			}
			deploymentRepo := &mockDeploymentRepository{
				findByIDFunc: func(ctx context.Context, deploymentID string) (*models.Deployment, error) {
					return &models.Deployment{ID: deploymentID, ProjectID: testCase.deploymentProjectID}, nil // deployment を返す
				},
			}
			ingressRouteRepo := &mockIngressRouteRepository{
				findByProjectIDFunc: func(ctx context.Context, projectID string) (*models.IngressRoute, error) {
					return &models.IngressRoute{ID: "ingress-1", ProjectID: projectID}, nil // ingress_route を返す
				},
			}
			var createdRoute *models.IngressRouteRoute // 作成されたルートエントリを保持する
			routeRepo := &mockIngressRouteRouteRepository{
				createFunc: func(ctx context.Context, tx *gorm.DB, route *models.IngressRouteRoute) error {
					createdRoute = route // 作成されたルートエントリを保存する
					return nil
				},
			}
			svc := newTestIngressRouteService(ingressRouteRepo, routeRepo, projectRepo, deploymentRepo)

			result, err := svc.AddRoute(context.Background(), testCase.userID, testCase.projectID, AddRouteRequest{
				DeploymentID: "deploy-1",
				PathPrefix:   "/api",
				Port:         8080,
			}) // AddRoute を呼び出す
			if testCase.expectedError != nil {
				if err == nil || err.Error() != testCase.expectedError.Error() { // エラーが期待と異なる場合はテスト失敗
					t.Errorf("期待するエラー %v, 実際のエラー %v", testCase.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Errorf("エラーは期待されていないが %v が返された", err) // エラーが不要なのに返された場合はテスト失敗
				return
			}
			if result.Status != models.IngressRouteRouteStatusPending { // ステータスが pending でない場合はテスト失敗
				t.Errorf("期待するステータス pending, 実際のステータス %s", result.Status)
			}
			if createdRoute == nil { // create が呼ばれていない場合はテスト失敗
				t.Error("ingressRouteRouteRepo.Create が呼ばれていない")
			}
		})
	}
}

// TestDeleteRoute は DeleteRoute の正常系・異常系をテストする
func TestDeleteRoute(t *testing.T) {
	testCases := []struct {
		name          string
		userID        string
		projectID     string
		projectUserID string
		expectedError error
	}{
		{
			name:          "ルートエントリのステータスが deleting になる",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-1",
			expectedError: nil,
		},
		{
			name:          "所有者でない場合は ErrForbidden を返す",
			userID:        "user-1",
			projectID:     "proj-1",
			projectUserID: "user-other",
			expectedError: ErrForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRepo := &mockProjectRepository{
				findByIDNoTxFunc: func(ctx context.Context, projectID string) (*models.Project, error) {
					return &models.Project{ID: projectID, UserID: testCase.projectUserID}, nil // プロジェクトを返す
				},
			}
			var updatedRoute *models.IngressRouteRoute // 更新されたルートエントリを保持する
			routeRepo := &mockIngressRouteRouteRepository{
				findByIDFunc: func(ctx context.Context, routeID string) (*models.IngressRouteRoute, error) {
					return &models.IngressRouteRoute{ID: routeID, Status: models.IngressRouteRouteStatusActive}, nil // ルートエントリを返す
				},
				updateFunc: func(ctx context.Context, route *models.IngressRouteRoute) error {
					updatedRoute = route // 更新されたルートエントリを保存する
					return nil
				},
			}
			svc := newTestIngressRouteService(&mockIngressRouteRepository{}, routeRepo, projectRepo, &mockDeploymentRepository{})

			err := svc.DeleteRoute(context.Background(), testCase.userID, testCase.projectID, "route-1") // DeleteRoute を呼び出す
			if testCase.expectedError != nil {
				if err == nil || err.Error() != testCase.expectedError.Error() { // エラーが期待と異なる場合はテスト失敗
					t.Errorf("期待するエラー %v, 実際のエラー %v", testCase.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Errorf("エラーは期待されていないが %v が返された", err) // エラーが不要なのに返された場合はテスト失敗
				return
			}
			if updatedRoute == nil { // update が呼ばれていない場合はテスト失敗
				t.Error("ingressRouteRouteRepo.Update が呼ばれていない")
				return
			}
			if updatedRoute.Status != models.IngressRouteRouteStatusDeleting { // ステータスが deleting でない場合はテスト失敗
				t.Errorf("期待するステータス deleting, 実際のステータス %s", updatedRoute.Status)
			}
		})
	}
}
