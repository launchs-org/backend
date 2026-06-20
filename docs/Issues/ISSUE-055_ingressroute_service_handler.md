# ISSUE-055 IngressRoute・PathRule サービス・ハンドラー・ルーター変更

## 親 Issue
ISSUE-052

## 概要

IngressRoute と PathRule の CRUD を Project ベースのサービス・ハンドラー・エンドポイントに変更する。
IngressRoute 作成時に `{ingressroute-uuid}.{BASE_DOMAIN}` でホストを自動生成する。

## 実装手順

### サービスインターフェース

`app/src/service/ingress_route_service.go` を新規作成し、IngressRoute・PathRule の CRUD を定義する（現状は `deployment_service.go` に混在しているため分離する）。

```go
type IngressRouteService interface {
    // IngressRoute
    GetIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error)
    CreateIngressRoute(ctx context.Context, userID string, projectID string) (*models.IngressRoute, error) // ホストは自動生成
    DeleteIngressRoute(ctx context.Context, userID string, projectID string) error

    // PathRule
    ListPathRules(ctx context.Context, userID string, ingressRouteID string) ([]*models.PathRule, error)
    CreatePathRule(ctx context.Context, userID string, ingressRouteID string, req CreatePathRuleRequest) (*models.PathRule, error)
    DeletePathRule(ctx context.Context, userID string, pathRuleID string) error // status=deleting に変更
}

type CreatePathRuleRequest struct {
    PathPrefix string `json:"path_prefix"`
    ServiceID  string `json:"service_id"`
}
```

### ホスト自動生成ロジック

`CreateIngressRoute` 内で以下のように生成する。

```go
ingressRouteData := &models.IngressRoute{} // 先に ID を確定させるため一度構造体を生成する
host := fmt.Sprintf("%s.%s", ingressRouteData.ID, os.Getenv("BASE_DOMAIN")) // UUID とベースドメインを結合する
ingressRouteData.Host = host
```

※ UUID は DB の `gen_random_uuid()` で生成されるため、Create 前に Go 側で UUID を生成して ID にセットする方法（`github.com/google/uuid`）を使う。

### 所有権チェック

- IngressRoute 操作: `project_id` からプロジェクトの `user_id` を確認する
- PathRule 操作: `ingress_route_id` → `project_id` → `user_id` と辿って確認する

### ハンドラー

`app/src/handler/ingress_route_handler.go` を新規作成する。

```go
type IngressRouteHandler struct {
    ingressRouteService service.IngressRouteService
}
```

エンドポイント一覧:
- `GET    /projects/:id/ingress-route`                   - GetIngressRoute
- `POST   /projects/:id/ingress-route`                   - CreateIngressRoute
- `DELETE /projects/:id/ingress-route`                   - DeleteIngressRoute
- `GET    /ingress-routes/:id/path-rules`                - ListPathRules
- `POST   /ingress-routes/:id/path-rules`                - CreatePathRule
- `DELETE /ingress-routes/:id/path-rules/:pathRuleID`    - DeletePathRule

### 変更ファイル

- `app/src/service/ingress_route_service.go`（新規作成）
    - 何を: IngressRouteService インターフェースと実装を定義する
    - なぜ: deployment_service.go から IngressRoute ロジックを分離し、Project ベースに変更するため

- `app/src/handler/ingress_route_handler.go`（新規作成）
    - 何を: IngressRoute・PathRule の HTTP ハンドラーを実装する
    - なぜ: エンドポイントを Project ベースに変更するため

- `app/src/handler/deployment_handler.go`（編集）
    - 何を: IngressRoute 関連の 4 メソッドを削除する
    - なぜ: ingress_route_handler.go に移管するため

- `app/src/service/deployment_service.go`（編集）
    - 何を: IngressRoute 関連メソッドをインターフェースと実装から削除する
    - なぜ: ingress_route_service.go に移管するため

- `app/src/router/router.go`（編集）
    - 何を: 旧 IngressRoute エンドポイントを削除し、新エンドポイントを登録する
    - なぜ: URL 構造を Project ベースに変更するため

- `app/src/main.go`（編集）
    - 何を: IngressRouteService・IngressRouteHandler の DI 組み立てを追加する
    - なぜ: 新規サービス・ハンドラーをルーターに接続するため

## テスト確認項目

- [ ] `POST /projects/:id/ingress-route` でホストが `{uuid}.{BASE_DOMAIN}` 形式で生成されること
- [ ] 同一プロジェクトに2回 CreateIngressRoute するとエラーになること
- [ ] `POST /ingress-routes/:id/path-rules` でパスルールが status=pending で作成されること
- [ ] `DELETE /ingress-routes/:id/path-rules/:pathRuleID` で status=deleting に変更されること
- [ ] 他ユーザーのリソースに操作すると 403 が返ること
