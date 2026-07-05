# ISSUE-053 IngressRoute モデル・リポジトリ変更（Project 紐づけ）

## 親 Issue
ISSUE-052

## 概要

`IngressRoute` モデルを `deployment_id` ベースから `project_id` ベースに変更する。
`PathPrefix` / `Port` / `pending_*` フィールドを削除し、ホスト名を IngressRoute 作成時に `{ingressroute-uuid}.{BASE_DOMAIN}` で自動生成する形にする。

## 実装手順

### モデル変更

`app/src/models/ingress_route.go` を以下の構造に変更する。

```go
type IngressRoute struct {
    ID        string             `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    ProjectID string             `gorm:"type:uuid;not null;uniqueIndex"                 json:"project_id"` // プロジェクトに1つ
    Host      string             `gorm:"type:text;not null"                             json:"host"`       // 払い出し済みドメイン
    Status    IngressRouteStatus `gorm:"type:text;not null"                             json:"status"`
    K8sStatus datatypes.JSON     `gorm:"type:jsonb"                                     json:"k8s_status"`
    CreatedAt time.Time          `json:"created_at"`
    UpdatedAt time.Time          `json:"updated_at"`
}
```

削除するフィールド:
- `DeploymentID`
- `PathPrefix` / `PendingPathPrefix`
- `Port` / `PendingPort`
- `TLSEnabled` / `PendingTLSEnabled`
- `CertificateResolver` / `PendingCertificateResolver`
- `PendingHost`

### リポジトリ変更

`app/src/repository/ingress_route_repository.go` のインターフェースを変更する。

```go
type IngressRouteRepository interface {
    Create(ctx context.Context, tx *gorm.DB, ingressRoute *models.IngressRoute) error
    FindByID(ctx context.Context, id string) (*models.IngressRoute, error)
    FindByProjectID(ctx context.Context, projectID string) (*models.IngressRoute, error) // FindByDeploymentID を置き換え
    Update(ctx context.Context, tx *gorm.DB, ingressRoute *models.IngressRoute) error
    UpdateStatus(ctx context.Context, ingressRouteID string, status models.IngressRouteStatus, k8sStatus []byte) error
    Delete(ctx context.Context, tx *gorm.DB, ingressRouteID string) error
}
```

### 変更ファイル

- `app/src/models/ingress_route.go`（編集）
    - 何を: `project_id` への変更・不要フィールド削除
    - なぜ: IngressRoute を Project 単位で管理するため

- `app/src/repository/ingress_route_repository.go`（編集）
    - 何を: `FindByDeploymentID` → `FindByProjectID` に変更、tx 引数追加
    - なぜ: Project 紐づけに対応するため

## テスト確認項目

- [ ] IngressRoute が `project_id` で作成・取得できること
- [ ] 同一プロジェクトに2つ目の IngressRoute を作成しようとすると uniqueIndex エラーになること
- [ ] `FindByProjectID` で正しく取得できること
