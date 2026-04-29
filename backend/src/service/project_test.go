package service

import (
	"backend/database" // データベースパッケージ
	"backend/model"    // モデルパッケージ
	"context"         // コンテキスト
	"testing"         // テストパッケージ

	"gorm.io/driver/sqlite" // SQLiteドライバ
	"gorm.io/gorm"          // GORM
	"k8s.io/client-go/kubernetes/fake" // K8sフェイククライアント
)

// TestCreateProject は CreateProject 関数のテストです
func TestCreateProject(t *testing.T) {
	// 1. テスト用のDB初期化 (インメモリSQLite)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	// DB接続に失敗した場合
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	// マイグレーション実行
	db.AutoMigrate(&model.Project{})
	// グローバルなDB変数をテスト用に上書き
	database.DB = db

	// 2. テスト用のK8sクライアント初期化 (フェイク)
	clientset := fake.NewSimpleClientset()
	// グローバルなK8sクライアント変数をテスト用に上書き
	database.K8sClientset = clientset

	// 3. テストケースの定義
	tests := []struct {
		name    string // テストケース名
		input   CreateProjectInput // 入力データ
		wantErr error // 期待されるエラー
	}{
		{
			name: "正常系: プロジェクトが作成できること",
			input: CreateProjectInput{
				Name:    "test-project",
				OwnerID: "user-1",
			},
			wantErr: nil,
		},
		{
			name: "異常系: プロジェクト名が空の場合はエラー",
			input: CreateProjectInput{
				Name:    "",
				OwnerID: "user-1",
			},
			wantErr: ErrInvalidProjectName,
		},
		{
			name: "異常系: プロジェクト名に不正な文字が含まれる場合はエラー",
			input: CreateProjectInput{
				Name:    "Test_Project!",
				OwnerID: "user-1",
			},
			wantErr: ErrInvalidProjectName,
		},
	}

	// 4. テストの実行
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// テスト対象の関数を呼び出し
			got, err := CreateProject(context.Background(), tt.input)
			
			// エラーの期待値チェック
			if err != tt.wantErr {
				t.Errorf("CreateProject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// 正常系の場合の追加チェック
			if tt.wantErr == nil {
				// 戻り値がnilでないこと
				if got == nil {
					t.Error("expected non-nil project")
					return
				}
				// 名前が一致すること
				if got.Name != tt.input.Name {
					t.Errorf("got.Name = %v, want %v", got.Name, tt.input.Name)
				}
				// OwnerIDが一致すること
				if got.OwnerID != tt.input.OwnerID {
					t.Errorf("got.OwnerID = %v, want %v", got.OwnerID, tt.input.OwnerID)
				}
				// Namespaceが ns-{uuid} の形式であること (uuidはuuid.New().String()なので簡易チェック)
				if got.Namespace == "" {
					t.Error("expected non-empty namespace")
				}
			}
		})
	}
}

// TestCreateProjectDuplicate は重複チェックのテストです
func TestCreateProjectDuplicate(t *testing.T) {
	// DB初期化
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&model.Project{})
	database.DB = db
	// K8s初期化
	database.K8sClientset = fake.NewSimpleClientset()

	// 1件作成しておく
	input := CreateProjectInput{
		Name:    "duplicate-project",
		OwnerID: "user-1",
	}
	CreateProject(context.Background(), input)

	// 同じ名前で再度作成を試みる
	_, err := CreateProject(context.Background(), input)
	
	// 重複エラーが返ることを期待
	if err != ErrProjectAlreadyExists {
		t.Errorf("expected ErrProjectAlreadyExists, got %v", err)
	}
}
