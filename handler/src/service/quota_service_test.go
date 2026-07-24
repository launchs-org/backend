package service

import (
	"handler/models"
	"context"
	"errors"
	"testing"
)

// noopUserQuotaRepository はQuotaチェックを常に通過させるテスト用スタブ実装
type noopUserQuotaRepository struct{}

func (repo *noopUserQuotaRepository) GetOrCreate(ctx context.Context, userID string, defaultPlanID string) (*models.UserQuota, error) {
	return &models.UserQuota{PlanID: defaultPlanID}, nil // ダミーの quota を返す
}

func (repo *noopUserQuotaRepository) GetResolvedQuota(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
	return &models.ResolvedQuota{ // 無制限相当の実効上限値を返す
		MaxProjects:              9999,
		MaxDeployments:           9999,
		MaxReplicasPerDeployment: 9999,
		MaxVolumes:               9999,
		MaxVolumeSizeMB:          9999999,
		MaxTotalVolumeMB:         9999999,
		InstanceLimits:           map[string]int{},
	}, nil
}

func (repo *noopUserQuotaRepository) Update(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error) {
	return &models.UserQuota{}, nil // 空の quota を返す
}

func (repo *noopUserQuotaRepository) CountProjects(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0 を返す
}

func (repo *noopUserQuotaRepository) CountDeployments(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0 を返す
}

func (repo *noopUserQuotaRepository) CountVolumes(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0 を返す
}

func (repo *noopUserQuotaRepository) CountInstancesBySize(ctx context.Context, userID string, instanceSize string) (int, error) {
	return 0, nil // 0 を返す
}

func (repo *noopUserQuotaRepository) SumVolumeMB(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0 を返す
}

// mockUserQuotaRepository は UserQuotaRepository のテスト用モック実装
type mockUserQuotaRepository struct {
	getResolvedQuotaFunc    func(ctx context.Context, userID string) (*models.ResolvedQuota, error)
	countProjectsFunc       func(ctx context.Context, userID string) (int, error)
	countDeploymentsFunc    func(ctx context.Context, userID string) (int, error)
	countVolumesFunc        func(ctx context.Context, userID string) (int, error)
	countInstancesBySizeFunc func(ctx context.Context, userID string, instanceSize string) (int, error)
	sumVolumeMBFunc         func(ctx context.Context, userID string) (int, error)
}

func (mock *mockUserQuotaRepository) GetOrCreate(ctx context.Context, userID string, defaultPlanID string) (*models.UserQuota, error) {
	return &models.UserQuota{PlanID: defaultPlanID}, nil // ダミーの quota を返す
}

func (mock *mockUserQuotaRepository) GetResolvedQuota(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
	return mock.getResolvedQuotaFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) Update(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error) {
	return &models.UserQuota{}, nil // 空の quota を返す
}

func (mock *mockUserQuotaRepository) CountProjects(ctx context.Context, userID string) (int, error) {
	if mock.countProjectsFunc == nil {
		return 0, nil // デフォルトで 0 を返す
	}
	return mock.countProjectsFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) CountDeployments(ctx context.Context, userID string) (int, error) {
	if mock.countDeploymentsFunc == nil {
		return 0, nil // デフォルトで 0 を返す
	}
	return mock.countDeploymentsFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) CountVolumes(ctx context.Context, userID string) (int, error) {
	if mock.countVolumesFunc == nil {
		return 0, nil // デフォルトで 0 を返す
	}
	return mock.countVolumesFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) CountInstancesBySize(ctx context.Context, userID string, instanceSize string) (int, error) {
	if mock.countInstancesBySizeFunc == nil {
		return 0, nil // デフォルトで 0 を返す
	}
	return mock.countInstancesBySizeFunc(ctx, userID, instanceSize) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) SumVolumeMB(ctx context.Context, userID string) (int, error) {
	if mock.sumVolumeMBFunc == nil {
		return 0, nil // デフォルトで 0 を返す
	}
	return mock.sumVolumeMBFunc(ctx, userID) // モック関数を呼び出す
}

// TestCheckProjectQuota_上限未満なら通過する は上限未満のプロジェクト数でチェックが通過することを確認する
func TestCheckProjectQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxProjects: 5}, nil // 上限5を返す
		},
		countProjectsFunc: func(ctx context.Context, userID string) (int, error) {
			return 3, nil // 現在3プロジェクトを返す
		},
	}

	err := CheckProjectQuota(context.Background(), userQuotaRepo, "user-1") // Quotaチェックを実行する
	if err != nil {                                                          // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckProjectQuota_上限に達した場合はQuotaExceededErrorを返す は上限到達時にQuotaExceededErrorを返すことを確認する
func TestCheckProjectQuota_上限に達した場合はQuotaExceededErrorを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxProjects: 5}, nil // 上限5を返す
		},
		countProjectsFunc: func(ctx context.Context, userID string) (int, error) {
			return 5, nil // 上限と同数のプロジェクト数を返す
		},
	}

	err := CheckProjectQuota(context.Background(), userQuotaRepo, "user-1") // Quotaチェックを実行する
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) { // QuotaExceededError であることを確認する
		t.Errorf("*QuotaExceededError を期待しましたが、実際のエラー: %v", err)
	}
	if quotaErr.Resource != "projects" { // リソース名を確認する
		t.Errorf("Resource は 'projects' を期待しましたが、実際: %s", quotaErr.Resource)
	}
	if quotaErr.Current != 5 || quotaErr.Limit != 5 { // Current と Limit を確認する
		t.Errorf("Current=5, Limit=5 を期待しましたが、Current=%d, Limit=%d", quotaErr.Current, quotaErr.Limit)
	}
}

// TestCheckDeploymentQuota_上限未満なら通過する は上限未満のデプロイメント数でチェックが通過することを確認する
func TestCheckDeploymentQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxDeployments: 20}, nil // 上限20を返す
		},
		countDeploymentsFunc: func(ctx context.Context, userID string) (int, error) {
			return 10, nil // 現在10デプロイメントを返す
		},
	}

	err := CheckDeploymentQuota(context.Background(), userQuotaRepo, "user-1") // Quotaチェックを実行する
	if err != nil {                                                             // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckDeploymentQuota_上限に達した場合はQuotaExceededErrorを返す は上限到達時にエラーを返すことを確認する
func TestCheckDeploymentQuota_上限に達した場合はQuotaExceededErrorを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxDeployments: 20}, nil // 上限20を返す
		},
		countDeploymentsFunc: func(ctx context.Context, userID string) (int, error) {
			return 20, nil // 上限と同数のデプロイメント数を返す
		},
	}

	err := CheckDeploymentQuota(context.Background(), userQuotaRepo, "user-1") // Quotaチェックを実行する
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) { // QuotaExceededError であることを確認する
		t.Errorf("*QuotaExceededError を期待しましたが、実際のエラー: %v", err)
	}
	if quotaErr.Resource != "deployments" { // リソース名を確認する
		t.Errorf("Resource は 'deployments' を期待しましたが、実際: %s", quotaErr.Resource)
	}
}

// TestCheckReplicasQuota_上限以下なら通過する はレプリカ数が上限以下の場合にチェックが通過することを確認する
func TestCheckReplicasQuota_上限以下なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxReplicasPerDeployment: 5}, nil // 上限5を返す
		},
	}

	err := CheckReplicasQuota(context.Background(), userQuotaRepo, "user-1", 5) // レプリカ数5でチェックを実行する
	if err != nil {                                                               // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckReplicasQuota_上限超過時はQuotaExceededErrorを返す はレプリカ数超過時にエラーを返すことを確認する
func TestCheckReplicasQuota_上限超過時はQuotaExceededErrorを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxReplicasPerDeployment: 5}, nil // 上限5を返す
		},
	}

	err := CheckReplicasQuota(context.Background(), userQuotaRepo, "user-1", 6) // 上限を超えるレプリカ数6でチェックを実行する
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) { // QuotaExceededError であることを確認する
		t.Errorf("*QuotaExceededError を期待しましたが、実際のエラー: %v", err)
	}
	if quotaErr.Resource != "replicas" { // リソース名を確認する
		t.Errorf("Resource は 'replicas' を期待しましたが、実際: %s", quotaErr.Resource)
	}
}

// TestCheckTotalVolumeQuota_上限未満なら通過する は追加後も上限未満の場合にチェックが通過することを確認する
func TestCheckTotalVolumeQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxTotalVolumeMB: 10240}, nil // 上限10240MBを返す
		},
		sumVolumeMBFunc: func(ctx context.Context, userID string) (int, error) {
			return 5000, nil // 現在5000MB使用中を返す
		},
	}

	err := CheckTotalVolumeQuota(context.Background(), userQuotaRepo, "user-1", 1000) // 1000MB追加でチェックを実行する
	if err != nil {                                                                     // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckTotalVolumeQuota_上限超過時はQuotaExceededErrorを返す は追加後に上限を超える場合にエラーを返すことを確認する
func TestCheckTotalVolumeQuota_上限超過時はQuotaExceededErrorを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxTotalVolumeMB: 10240}, nil // 上限10240MBを返す
		},
		sumVolumeMBFunc: func(ctx context.Context, userID string) (int, error) {
			return 9500, nil // 現在9500MB使用中を返す
		},
	}

	err := CheckTotalVolumeQuota(context.Background(), userQuotaRepo, "user-1", 1000) // 1000MB追加で上限超過となるチェックを実行する
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) { // QuotaExceededError であることを確認する
		t.Errorf("*QuotaExceededError を期待しましたが、実際のエラー: %v", err)
	}
	if quotaErr.Resource != "total_volume_mb" { // リソース名を確認する
		t.Errorf("Resource は 'total_volume_mb' を期待しましたが、実際: %s", quotaErr.Resource)
	}
}

// TestCheckVolumeSizeQuota_上限以内なら通過する は指定サイズが上限以内の場合にチェックが通過することを確認する
func TestCheckVolumeSizeQuota_上限以内なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxVolumeSizeMB: 10240}, nil // 上限10240MBを返す
		},
	}

	err := CheckVolumeSizeQuota(context.Background(), userQuotaRepo, "user-1", 1024) // 1024MB でチェックを実行する
	if err != nil {                                                                    // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckVolumeSizeQuota_上限超過時はQuotaExceededErrorを返す は上限を超えるサイズ指定時にエラーを返すことを確認する
func TestCheckVolumeSizeQuota_上限超過時はQuotaExceededErrorを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{MaxVolumeSizeMB: 10240}, nil // 上限10240MBを返す
		},
	}

	err := CheckVolumeSizeQuota(context.Background(), userQuotaRepo, "user-1", 20480) // 20480MB（上限超過）でチェックを実行する
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) { // QuotaExceededError であることを確認する
		t.Errorf("*QuotaExceededError を期待しましたが、実際のエラー: %v", err)
	}
	if quotaErr.Resource != "volume_size_mb" { // リソース名を確認する
		t.Errorf("Resource は 'volume_size_mb' を期待しましたが、実際: %s", quotaErr.Resource)
	}
}

// TestCheckInstanceQuota_上限未満なら通過する はスペック別上限未満の場合にチェックが通過することを確認する
func TestCheckInstanceQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{InstanceLimits: map[string]int{"small": 5}}, nil // small の上限5を返す
		},
		countInstancesBySizeFunc: func(ctx context.Context, userID string, instanceSize string) (int, error) {
			return 3, nil // 現在3台を返す
		},
	}

	err := CheckInstanceQuota(context.Background(), userQuotaRepo, "user-1", "small") // small でチェックを実行する
	if err != nil {                                                                     // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckInstanceQuota_上限に達した場合はQuotaExceededErrorを返す はスペック別上限到達時にエラーを返すことを確認する
func TestCheckInstanceQuota_上限に達した場合はQuotaExceededErrorを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getResolvedQuotaFunc: func(ctx context.Context, userID string) (*models.ResolvedQuota, error) {
			return &models.ResolvedQuota{InstanceLimits: map[string]int{"small": 5}}, nil // small の上限5を返す
		},
		countInstancesBySizeFunc: func(ctx context.Context, userID string, instanceSize string) (int, error) {
			return 5, nil // 上限と同数を返す
		},
	}

	err := CheckInstanceQuota(context.Background(), userQuotaRepo, "user-1", "small") // small でチェックを実行する
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) { // QuotaExceededError であることを確認する
		t.Errorf("*QuotaExceededError を期待しましたが、実際のエラー: %v", err)
	}
	if quotaErr.Resource != "instance:small" { // リソース名を確認する
		t.Errorf("Resource は 'instance:small' を期待しましたが、実際: %s", quotaErr.Resource)
	}
}
