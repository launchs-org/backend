package service

import (
	"app/models"
	"context"
	"errors"
	"testing"
)

// noopUserQuotaRepository はQuotaチェックを常に通過させるテスト用スタブ実装
type noopUserQuotaRepository struct{}

func (repo *noopUserQuotaRepository) GetOrCreate(ctx context.Context, userID string) (*models.UserQuota, error) {
	return &models.UserQuota{MaxProjects: 9999, MaxDeployments: 9999, MaxReplicasPerDeployment: 9999, MaxVolumeMB: 9999999}, nil // 無制限相当のquotaを返す
}

func (repo *noopUserQuotaRepository) Update(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error) {
	return &models.UserQuota{}, nil // 空のquotaを返す
}

func (repo *noopUserQuotaRepository) CountProjects(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0を返す
}

func (repo *noopUserQuotaRepository) CountDeployments(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0を返す
}

func (repo *noopUserQuotaRepository) SumVolumeMB(ctx context.Context, userID string) (int, error) {
	return 0, nil // 0を返す
}

// mockUserQuotaRepository は UserQuotaRepository のテスト用モック実装
type mockUserQuotaRepository struct {
	getOrCreateFunc     func(ctx context.Context, userID string) (*models.UserQuota, error)
	updateFunc          func(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error)
	countProjectsFunc   func(ctx context.Context, userID string) (int, error)
	countDeploymentsFunc func(ctx context.Context, userID string) (int, error)
	sumVolumeMBFunc     func(ctx context.Context, userID string) (int, error)
}

func (mock *mockUserQuotaRepository) GetOrCreate(ctx context.Context, userID string) (*models.UserQuota, error) {
	return mock.getOrCreateFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) Update(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserQuota, error) {
	return mock.updateFunc(ctx, userID, updates) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) CountProjects(ctx context.Context, userID string) (int, error) {
	return mock.countProjectsFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) CountDeployments(ctx context.Context, userID string) (int, error) {
	return mock.countDeploymentsFunc(ctx, userID) // モック関数を呼び出す
}

func (mock *mockUserQuotaRepository) SumVolumeMB(ctx context.Context, userID string) (int, error) {
	return mock.sumVolumeMBFunc(ctx, userID) // モック関数を呼び出す
}

// TestCheckProjectQuota_上限未満なら通過する は上限未満のプロジェクト数でチェックが通過することを確認する
func TestCheckProjectQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxProjects: 5}, nil // 上限5を返す
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

// TestCheckProjectQuota_上限に達した場合はErrProjectQuotaExceededを返す は上限到達時にエラーを返すことを確認する
func TestCheckProjectQuota_上限に達した場合はErrProjectQuotaExceededを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxProjects: 5}, nil // 上限5を返す
		},
		countProjectsFunc: func(ctx context.Context, userID string) (int, error) {
			return 5, nil // 上限と同数のプロジェクト数を返す
		},
	}

	err := CheckProjectQuota(context.Background(), userQuotaRepo, "user-1") // Quotaチェックを実行する
	if !errors.Is(err, ErrProjectQuotaExceeded) {                           // ErrProjectQuotaExceeded であることを確認する
		t.Errorf("ErrProjectQuotaExceeded を期待しましたが、実際のエラー: %v", err)
	}
}

// TestCheckDeploymentQuota_上限未満なら通過する は上限未満のデプロイメント数でチェックが通過することを確認する
func TestCheckDeploymentQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxDeployments: 20}, nil // 上限20を返す
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

// TestCheckDeploymentQuota_上限に達した場合はErrDeploymentQuotaExceededを返す は上限到達時にエラーを返すことを確認する
func TestCheckDeploymentQuota_上限に達した場合はErrDeploymentQuotaExceededを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxDeployments: 20}, nil // 上限20を返す
		},
		countDeploymentsFunc: func(ctx context.Context, userID string) (int, error) {
			return 20, nil // 上限と同数のデプロイメント数を返す
		},
	}

	err := CheckDeploymentQuota(context.Background(), userQuotaRepo, "user-1") // Quotaチェックを実行する
	if !errors.Is(err, ErrDeploymentQuotaExceeded) {                           // ErrDeploymentQuotaExceeded であることを確認する
		t.Errorf("ErrDeploymentQuotaExceeded を期待しましたが、実際のエラー: %v", err)
	}
}

// TestCheckReplicasQuota_上限以下なら通過する はレプリカ数が上限以下の場合にチェックが通過することを確認する
func TestCheckReplicasQuota_上限以下なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxReplicasPerDeployment: 5}, nil // 上限5を返す
		},
	}

	err := CheckReplicasQuota(context.Background(), userQuotaRepo, "user-1", 5) // レプリカ数5でチェックを実行する
	if err != nil {                                                               // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckReplicasQuota_上限超過時はErrReplicasQuotaExceededを返す はレプリカ数超過時にエラーを返すことを確認する
func TestCheckReplicasQuota_上限超過時はErrReplicasQuotaExceededを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxReplicasPerDeployment: 5}, nil // 上限5を返す
		},
	}

	err := CheckReplicasQuota(context.Background(), userQuotaRepo, "user-1", 6) // 上限を超えるレプリカ数6でチェックを実行する
	if !errors.Is(err, ErrReplicasQuotaExceeded) {                              // ErrReplicasQuotaExceeded であることを確認する
		t.Errorf("ErrReplicasQuotaExceeded を期待しましたが、実際のエラー: %v", err)
	}
}

// TestCheckVolumeQuota_上限未満なら通過する は追加後も上限未満の場合にチェックが通過することを確認する
func TestCheckVolumeQuota_上限未満なら通過する(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxVolumeMB: 10240}, nil // 上限10240MBを返す
		},
		sumVolumeMBFunc: func(ctx context.Context, userID string) (int, error) {
			return 5000, nil // 現在5000MB使用中を返す
		},
	}

	err := CheckVolumeQuota(context.Background(), userQuotaRepo, "user-1", 1000) // 1000MB追加でチェックを実行する
	if err != nil {                                                               // エラーがないことを確認する
		t.Errorf("エラーが発生しないことを期待しましたが、エラーが返りました: %v", err)
	}
}

// TestCheckVolumeQuota_上限超過時はErrVolumeQuotaExceededを返す は追加後に上限を超える場合にエラーを返すことを確認する
func TestCheckVolumeQuota_上限超過時はErrVolumeQuotaExceededを返す(t *testing.T) {
	userQuotaRepo := &mockUserQuotaRepository{
		getOrCreateFunc: func(ctx context.Context, userID string) (*models.UserQuota, error) {
			return &models.UserQuota{MaxVolumeMB: 10240}, nil // 上限10240MBを返す
		},
		sumVolumeMBFunc: func(ctx context.Context, userID string) (int, error) {
			return 9500, nil // 現在9500MB使用中を返す
		},
	}

	err := CheckVolumeQuota(context.Background(), userQuotaRepo, "user-1", 1000) // 1000MB追加で上限超過となるチェックを実行する
	if !errors.Is(err, ErrVolumeQuotaExceeded) {                                 // ErrVolumeQuotaExceeded であることを確認する
		t.Errorf("ErrVolumeQuotaExceeded を期待しましたが、実際のエラー: %v", err)
	}
}
