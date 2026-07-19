# CLAUDE.md — backend コーディング規約

## プロジェクト概要

Go 製の PaaS バックエンド。Echo v4 を HTTP フレームワーク、GORM を ORM、k8s client-go で Kubernetes を操作する。
ユーザーがデプロイメントを定義し、`apply` 操作で Kubernetes リソースを同期するアーキテクチャ。

- **言語**: Go 1.26
- **フレームワーク**: Echo v4
- **ORM**: GORM (PostgreSQL)
- **インフラ**: Kubernetes (client-go, dynamic client)
- **モジュール名**: `app`

---

## ディレクトリ構造

```
app/src/
├── main.go           # エントリーポイント、Echo ルーター定義、DI 組み立て
├── models/           # GORM モデル定義（DB テーブルに対応）
├── repository/       # DB 接続・マイグレーション（グローバル Database 変数）
├── handler/          # HTTP ハンドラー（Echo のリクエスト受け取り・レスポンス返却）
├── service/          # ビジネスロジック（トランザクション・k8s 操作）
├── middlewares/      # JWT 検証・認証ミドルウェア
├── k8s/              # Kubernetes クライアントラッパー
└── logger/           # カスタムロガー
```

`docs/Issues/` に実装仕様書（ISSUE-XXX_*.md）が格納されている。

---

## 実装フロー（必須）

**Issue を元に実装する場合は以下の手順を厳守すること。**

### 1. Issue を読む

実装前に必ず対象 Issue を読む。

```
docs/Issues/ISSUE-{番号}_{説明}.md
```

Issue には以下が記載されている：
- **親 Issue**: 上位 Issue 番号
- **概要**: 何を実装するか
- **実装手順**: フォルダ構造・サンプルコード
- **テスト確認項目**: 実装後に確認すべきチェックリスト

### 2. 実装計画サマリーを提示する

実装を開始する前に、以下を含む計画サマリーをユーザーに提示する：

- **実装内容**: 何をどのように実装するか
- **変更予定ファイル**: 作成・編集するファイル一覧（各ファイルについて「何を追加・変更するか」と「なぜそのファイルを変更する必要があるか」の理由を記載する）

  記載例：
  ```
  - app/src/handler/deployment.go（新規作成）
      何を: DeploymentHandler 構造体と CreateDeployment/UpdateDeployment ハンドラーを実装する
      なぜ: HTTP リクエストの受け取りとレスポンス返却を担う層が必要なため

  - app/src/service/deployment.go（新規作成）
      何を: DeploymentService インターフェースと実装を追加する
      なぜ: ハンドラーからビジネスロジックを分離し、テスト可能にするため

  - app/src/main.go（編集）
      何を: DeploymentHandler の DI 組み立てとルーター登録を追加する
      なぜ: 新規ハンドラーをルーターに接続するエントリーポイントの更新が必要なため
  ```

- **テスト計画**: Issue の「テスト確認項目」を元にした確認手順

### 3. ユーザーの承認を得てから実装を開始する

計画サマリーを提示した後、**必ずユーザーの承認を待つ**。承認なしに実装を開始しない。

---

## コーディング規約

### 変数命名

- **1文字変数名は使用禁止**（`e`, `c`, `r`, `v`, `i` など）
- **キャメルケース（lowerCamelCase）** を使う（例: `userData`, `deploymentList`, `projectID`）
- 意味のある名前を使う

```go
// 禁止
e := echo.New()
c := context.Background()
u := &models.User{}
d, err := getDeployment(id)

// 推奨
router := echo.New()
ctx := context.Background()
userData := &models.User{}
deploymentData, err := getDeployment(projectID)
```

- ループ変数も同様（`i` → `itemIndex`, `deploymentIndex` など）
- 複数形はスライスに使う（`deploymentList`, `projectList`）
- エラー変数は `err` のみ例外として許容する

### コメント

- **各処理行の後に日本語コメントを記述する**
- コメントは「何をするか」ではなく「なぜするか・何をするか」を簡潔に書く

```go
func (handler *DeploymentHandler) CreateDeployment(echoCtx echo.Context) error {
    userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWTクレームを取得する
    var requestBody CreateDeploymentRequest                            // リクエストボディの構造体を定義する
    if err := echoCtx.Bind(&requestBody); err != nil {               // リクエストをバインドする
        return echoCtx.JSON(http.StatusBadRequest, err)               // バインドエラーを返す
    }
    result, err := handler.deploymentService.CreateDeployment(        // サービスを呼び出してデプロイメントを作成する
        echoCtx.Request().Context(),
        userClaim.UserID,
        requestBody,
    )
    if err != nil {                                                    // エラーが発生した場合
        return echoCtx.JSON(http.StatusInternalServerError, err)      // 500 エラーを返す
    }
    return echoCtx.JSON(http.StatusCreated, result)                   // 作成結果を返す
}
```

### インターフェース

- **Service レイヤー**と **Repository レイヤー** にインターフェースを定義する
- Handler は必ずインターフェース経由で依存を受け取る（具象型に依存しない）
- `main.go` で具体実装を生成し、コンストラクタで注入する

```go
// service/deployment.go に定義
type DeploymentService interface {
    CreateDeployment(ctx context.Context, userID string, req CreateDeploymentRequest) (*models.Deployment, error)
    UpdateDeployment(ctx context.Context, deploymentID string, req UpdateDeploymentRequest) (*models.Deployment, error)
    DeleteDeployment(ctx context.Context, deploymentID string) error
    GetDeployment(ctx context.Context, deploymentID string) (*models.Deployment, error)
    ListDeployments(ctx context.Context, projectID string) ([]*models.Deployment, error)
}
```

### Handler 構造体

- 依存をフィールドに持ち、コンストラクタで注入する
- フィールド名はインターフェース型で宣言する

```go
// handler/deployment.go
type DeploymentHandler struct {
    deploymentService service.DeploymentService // デプロイメントサービスのインターフェース
}

func NewDeploymentHandler(deploymentService service.DeploymentService) *DeploymentHandler {
    return &DeploymentHandler{
        deploymentService: deploymentService, // 依存を注入する
    }
}
```

### main.go での DI 組み立て

```go
// 具体実装の生成
deploymentServiceImpl := service.NewDeploymentServiceImpl(repository.Database, k8sClient) // サービス実装を生成する
deploymentHandler := handler.NewDeploymentHandler(deploymentServiceImpl)                   // ハンドラーに注入する

// ルーター登録
apiGroup := router.Group("/api", middlewares.RequireAuth)            // 認証必須グループを作成する
apiGroup.POST("/deployments", deploymentHandler.CreateDeployment)    // デプロイメント作成エンドポイント
```

---

## 既存コードパターン

### pending_* フィールドパターン

設定変更は `pending_*` フィールドに書き込み、`apply` 実行時に本フィールドへ昇格する。

```go
// 更新時は pending フィールドに書く
deployment.PendingImageURL = requestBody.ImageURL // 未適用の変更として保持する

// apply 時に昇格させる
deployment.ImageURL = deployment.PendingImageURL  // pending を実際の値に昇格する
deployment.PendingImageURL = ""                   // pending をクリアする
```

### Status 定数パターン

```go
type DeploymentStatus string

const (
    DeploymentStatusPending  DeploymentStatus = "pending"
    DeploymentStatusRunning  DeploymentStatus = "running"
    DeploymentStatusDeleting DeploymentStatus = "deleting"
    DeploymentStatusError    DeploymentStatus = "error"
)
```

### GORM モデルパターン

```go
type Deployment struct {
    ID        string           `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"` // UUID主キー
    ProjectID string           `gorm:"type:uuid;not null;index"                       json:"project_id"` // 親プロジェクトID
    Status    DeploymentStatus `gorm:"type:text;not null"                             json:"status"` // リソースのステータス
}

func (deployment *Deployment) TableName() string {
    return "deployments" // テーブル名を明示する
}
```

### Echo レスポンス形式

```go
// 成功レスポンス
return echoCtx.JSON(http.StatusOK, responseData)       // 200 OK
return echoCtx.JSON(http.StatusCreated, responseData)  // 201 Created

// エラーレスポンス
return echoCtx.JSON(http.StatusBadRequest, map[string]string{
    "error": "リクエストが不正です", // エラーメッセージ
})
return echoCtx.JSON(http.StatusNotFound, map[string]string{
    "error": "リソースが見つかりません", // 404 エラーメッセージ
})
return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
    "error": "内部サーバーエラー", // 500 エラーメッセージ
})
```

### JWT クレーム取得パターン

```go
userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // コンテキストからクレームを取得する
userID := userClaim.UserID                                        // ユーザーIDを取得する
```

### ロガーパターン

```go
logger.Println("処理を開始します")       // 通常ログ
logger.PrintErr("エラーが発生しました")   // エラーログ
```

---

## 実行環境

アプリケーションは Docker Compose で動作している。コマンドを実行する際は必ず以下の形式を使う。

```bash
docker compose exec app {コマンド}
```

例：
```bash
docker compose exec app go build ./...         # ビルド確認
docker compose exec app go test ./...          # テスト実行
docker compose exec app go vet ./...           # 静的解析
```

---

## テスト実装と確認

### テストの実装

実装完了後、Issue の「テスト確認項目」を元に Go のテストコードを実装する。
テストファイルは実装ファイルと同じパッケージに `_test.go` サフィックスで作成する。

**handler / service / repository の 3 層すべてにテストを実装すること。**

```
app/src/
├── handler/
│   ├── deployment.go
│   └── deployment_test.go           # handler テスト（service インターフェースをモック）
├── service/
│   ├── deployment.go
│   └── deployment_test.go           # service テスト（repository インターフェースをモック）
└── repository/
    ├── deployment_repository.go
    └── deployment_repository_test.go # repository テスト（実 DB または sqlmock）
```

各レイヤーのテスト方針：

- **handler テスト**: service インターフェースをモックし、Echo の `httptest` でリクエスト/レスポンスを検証する
- **service テスト**: repository インターフェースをモックし、ビジネスロジック（分岐・エラー処理・ステータス遷移）を検証する
- **repository テスト**: テスト用 PostgreSQL または `sqlmock` を使い、クエリの正しさを検証する

テストコードもコーディング規約（1文字変数禁止・日本語コメント）に従う。

```go
func TestCreateDeployment(t *testing.T) {
    // テストケースを定義する
    testCases := []struct {
        name           string
        requestBody    CreateDeploymentRequest
        expectedStatus int
    }{
        {
            name:           "正常にデプロイメントが作成される",       // 正常系のテストケース
            requestBody:    CreateDeploymentRequest{Name: "test"}, // リクエストボディを設定する
            expectedStatus: http.StatusCreated,                    // 期待するステータスコード
        },
    }

    for _, testCase := range testCases {
        t.Run(testCase.name, func(t *testing.T) {
            // テスト処理を記述する
        })
    }
}
```

### テストの実行と確認

テスト実装後、Docker 経由で実行して全件パスすることを確認する。

```bash
task test-all
```

Issue の「テスト確認項目」チェックリストをすべてテストコードでカバーできているか確認する。

---

## 依存関係の方向

```
main.go
  └─ handler（HTTP 層）
       └─ service（ビジネスロジック層）
            └─ repository インターフェース（DB アクセス）
            └─ k8s クライアント（Kubernetes 操作）
                 └─ repository 実装（GORM）
                      └─ repository.Database (*gorm.DB)
```

- 上位レイヤーは下位レイヤーのインターフェースにのみ依存する
- 具体実装は `main.go` でのみ生成する

---

## アーキテクチャ禁止事項

### Service レイヤーの制約

- **禁止**: `service` が GORM メソッド（`.Where()`, `.First()`, `.Save()` 等）を直接呼び出すこと
- **禁止**: `service` が `repository.Database` グローバル変数を直接参照すること
- **必須**: 全ての DB アクセスは repository インターフェースのメソッド経由で行う

```go
// 禁止
func (svc *projectServiceImpl) GetProject(ctx context.Context, id string) (*models.Project, error) {
    project := &models.Project{}
    err := repository.Database.Where("id = ?", id).First(project).Error // 直接 GORM を呼ぶのは禁止
    return project, err
}

// 推奨
func (svc *projectServiceImpl) GetProject(ctx context.Context, id string) (*models.Project, error) {
    return svc.projectRepo.FindByID(ctx, nil, id) // repository インターフェース経由
}
```

### トランザクションの扱い

- **許可**: service がトランザクション管理専用として `*gorm.DB` をフィールドに保持すること
- **許可**: service が `db.WithContext(ctx).Transaction()` でトランザクションを開始すること
- **必須**: トランザクション内の DB 操作は repository インターフェースのメソッドに `tx *gorm.DB` を引数として渡して委譲する
- **禁止**: トランザクション内で GORM メソッドを service が直接呼び出すこと

```go
// 正しいトランザクションパターン
func (svc *projectServiceImpl) CreateProject(ctx context.Context, ...) (*models.Project, error) {
    var projectData *models.Project                                                         // 結果格納用変数を宣言する
    err := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {                   // トランザクションを開始する
        projectData = &models.Project{Name: requestBody.Name}                              // プロジェクトデータを生成する
        if err := svc.projectRepo.Create(ctx, tx, projectData); err != nil {              // tx を渡して repository に委譲する
            return err                                                                      // エラーでロールバックする
        }
        return nil                                                                          // コミットする
    })
    return projectData, err                                                                 // 結果を返す
}
```

### Handler レイヤーの制約

- **禁止**: `handler` が repository を直接参照すること（service インターフェース経由のみ）
- **禁止**: `handler` が `*gorm.DB` をフィールドに持つこと
