# ISSUE-054 PathRule モデル・リポジトリ作成

## 親 Issue
ISSUE-052

## 概要

IngressRoute に対してパスごとのルーティングルールを管理する `PathRule` モデルとリポジトリを新規作成する。
PathRule は `status` フィールドで pending / active / deleting を管理し、apply 時に Kubernetes へ反映する。

## 実装手順

### モデル作成

`app/src/models/path_rule.go` を新規作成する。

```go
package models

import (
    "time"
)

type PathRuleStatus string

const (
    PathRuleStatusPending  PathRuleStatus = "pending"
    PathRuleStatusActive   PathRuleStatus = "active"
    PathRuleStatusDeleting PathRuleStatus = "deleting"
)

type PathRule struct {
    ID             string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    IngressRouteID string         `gorm:"type:uuid;not null;index"                       json:"ingress_route_id"`
    PathPrefix     string         `gorm:"type:text;not null"                             json:"path_prefix"`   // ルーティング対象パス
    ServiceID      string         `gorm:"type:uuid;not null"                             json:"service_id"`    // 対象 Service の ID
    Status         PathRuleStatus `gorm:"type:text;not null"                             json:"status"`
    CreatedAt      time.Time      `json:"created_at"`
    UpdatedAt      time.Time      `json:"updated_at"`
}

func (pathRule *PathRule) TableName() string {
    return "path_rules" // テーブル名を明示する
}
```

### リポジトリ作成

`app/src/repository/path_rule_repository.go` を新規作成する。

```go
type PathRuleRepository interface {
    Create(ctx context.Context, tx *gorm.DB, pathRule *models.PathRule) error
    FindByID(ctx context.Context, id string) (*models.PathRule, error)
    FindByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error)
    FindActiveAndPendingByIngressRouteID(ctx context.Context, ingressRouteID string) ([]*models.PathRule, error) // apply 時に使用
    Update(ctx context.Context, tx *gorm.DB, pathRule *models.PathRule) error
    UpdateStatus(ctx context.Context, tx *gorm.DB, pathRuleID string, status models.PathRuleStatus) error
    Delete(ctx context.Context, tx *gorm.DB, pathRuleID string) error // 物理削除（apply 後の deleting 行の削除）
}
```

### マイグレーション登録

`app/src/repository/repository.go`（または DB 初期化箇所）に `PathRule` を AutoMigrate に追加する。

### 変更ファイル

- `app/src/models/path_rule.go`（新規作成）
    - 何を: PathRule モデルを定義する
    - なぜ: パスルールを独立したエンティティとして管理するため

- `app/src/repository/path_rule_repository.go`（新規作成）
    - 何を: PathRule の CRUD リポジトリを実装する
    - なぜ: サービス層から DB アクセスを分離するため

- `app/src/repository/repository.go`（編集）
    - 何を: `PathRule` を AutoMigrate に追加する
    - なぜ: テーブルを自動生成するため

## テスト確認項目

- [ ] PathRule が作成・取得できること
- [ ] `FindByIngressRouteID` で IngressRoute に紐づく PathRule 一覧を取得できること
- [ ] `FindActiveAndPendingByIngressRouteID` で status が active / pending の行のみ取得できること
- [ ] status=deleting の行は `FindActiveAndPendingByIngressRouteID` に含まれないこと
- [ ] `Delete` で物理削除されること
