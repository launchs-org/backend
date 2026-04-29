package controller

import (
	"backend/database" // データベースパッケージ
	"backend/model"    // モデルパッケージ
	"backend/service"  // サービスパッケージ
	"bytes"           // バイト操作
	"encoding/json"    // JSON
	"net/http"         // HTTP
	"net/http/httptest" // HTTPテスト
	"testing"          // テストパッケージ

	"github.com/labstack/echo/v5" // Echo
	"gorm.io/driver/sqlite"      // SQLite
	"gorm.io/gorm"               // GORM
	"k8s.io/client-go/kubernetes/fake" // K8sフェイク
)

// TestCreateProjectController は CreateProject ハンドラーのテストです
func TestCreateProjectController(t *testing.T) {
	// 1. 依存関係のモック化
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{}) // インメモリDB
	db.AutoMigrate(&model.Project{}) // マイグレーション
	database.DB = db // グローバル変数を上書き
	database.K8sClientset = fake.NewSimpleClientset() // K8sフェイクを上書き

	// 2. Echoのセットアップ
	e := echo.New()

	// 3. テストケースの定義
	tests := []struct {
		name         string // テストケース名
		requestBody  map[string]interface{} // リクエストボディ
		userID       string // コンテキストにセットするユーザーID
		wantStatus   int    // 期待されるステータスコード
		wantCode     string // 期待されるエラーコード (エラー時のみ)
	}{
		{
			name: "正常系: プロジェクトが作成できること",
			requestBody: map[string]interface{}{
				"name": "valid-project",
			},
			userID:     "user-123",
			wantStatus: http.StatusCreated,
		},
		{
			name: "異常系: バリデーションエラー (名前が不正)",
			requestBody: map[string]interface{}{
				"name": "Invalid_Name!",
			},
			userID:     "user-123",
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name: "異常系: 認証エラー (UserIDが未設定)",
			requestBody: map[string]interface{}{
				"name": "some-project",
			},
			userID:     "", // 未設定をシミュレート
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	// 4. テストの実行
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// リクエストボディをJSON化
			body, _ := json.Marshal(tt.requestBody)
			// POSTリクエストを作成
			req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			// レスポンス記録用レコーダー
			rec := httptest.NewRecorder()
			// Echoコンテキストを作成
			c := e.NewContext(req, rec)
			
			// UserIDをコンテキストにセット (認証済み状態をシミュレート)
			if tt.userID != "" {
				c.Set("UserID", tt.userID)
			}

			// ハンドラーを実行
			err := CreateProject(c)
			// ハンドラー自体がエラーを返した場合はテスト失敗
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			// ステータスコードのチェック
			if rec.Code != tt.wantStatus {
				t.Errorf("status code = %v, want %v", rec.Code, tt.wantStatus)
			}

			// エラー時のコードチェック
			if tt.wantCode != "" {
				var resp map[string]string
				json.Unmarshal(rec.Body.Bytes(), &resp)
				if resp["code"] != tt.wantCode {
					t.Errorf("error code = %v, want %v", resp["code"], tt.wantCode)
				}
			}
		})
	}
}

// TestCreateProjectConflictController は名前重複時のテストです
func TestCreateProjectConflictController(t *testing.T) {
	// 初期化
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&model.Project{})
	database.DB = db
	database.K8sClientset = fake.NewSimpleClientset()
	e := echo.New()

	// 1件作成しておく
	service.CreateProject(nil, service.CreateProjectInput{Name: "duplicate", OwnerID: "user-1"})

	// 重複する名前でリクエスト
	body, _ := json.Marshal(map[string]string{"name": "duplicate"})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("UserID", "user-1")

	// 実行
	CreateProject(c)

	// 409 Conflict を期待
	if rec.Code != http.StatusConflict {
		t.Errorf("status code = %v, want %v", rec.Code, http.StatusConflict)
	}
}

// TestGetProjectController は GetProject ハンドラーのテストです
func TestGetProjectController(t *testing.T) {
	// 1. 初期化
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&model.Project{})
	database.DB = db
	e := echo.New()

	// 2. テスト用データを投入
	p1 := model.Project{ID: "proj-1", Name: "my-app", OwnerID: "user-1"}
	model.CreateProject(&p1)

	// 3. テストケース
	tests := []struct {
		name       string
		projectID  string
		userID     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "正常系: プロジェクトが取得できること",
			projectID:  "proj-1",
			userID:     "user-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "異常系: 他人のプロジェクトにアクセス (403)",
			projectID:  "proj-1",
			userID:     "user-other",
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "異常系: 存在しないプロジェクト (404)",
			projectID:  "not-exists",
			userID:     "user-1",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/projects/"+tt.projectID, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPathValues(echo.PathValues{{Name: "id", Value: tt.projectID}})
			if tt.userID != "" {
				c.Set("UserID", tt.userID)
			}

			GetProject(c)

			if rec.Code != tt.wantStatus {
				t.Errorf("status code = %v, want %v", rec.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				var resp map[string]string
				json.Unmarshal(rec.Body.Bytes(), &resp)
				if resp["code"] != tt.wantCode {
					t.Errorf("error code = %v, want %v", resp["code"], tt.wantCode)
				}
			}
		})
	}
}

// TestListProjectsController は ListProjects ハンドラーのテストです
func TestListProjectsController(t *testing.T) {
	// 1. 初期化
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&model.Project{})
	database.DB = db
	e := echo.New()

	// 2. テスト用データを投入
	model.CreateProject(&model.Project{ID: "p1", Name: "app-1", OwnerID: "user-1"})
	model.CreateProject(&model.Project{ID: "p2", Name: "app-2", OwnerID: "user-1"})
	model.CreateProject(&model.Project{ID: "p3", Name: "app-3", OwnerID: "user-2"})

	// 3. テスト実行
	t.Run("正常系: 自分のプロジェクトのみ取得されること", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("UserID", "user-1")

		ListProjects(c)

		if rec.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rec.Code, http.StatusOK)
		}

		var resp struct {
			Data struct {
				Items []model.Project `json:"items"`
				Total int             `json:"total"`
			} `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &resp)

		if resp.Data.Total != 2 {
			t.Errorf("total = %v, want 2", resp.Data.Total)
		}
		if len(resp.Data.Items) != 2 {
			t.Errorf("items count = %v, want 2", len(resp.Data.Items))
		}
	})
}
