# ISSUE-018 IngressRouteエンドポイントCRUD

## 親 Issue
ISSUE-015

## 概要
Traefik IngressRouteの設定を管理するエンドポイントを実装する。ホスト名・パスルーティングの設定をDBに保存し、applyで反映する。

## 変更ファイル一覧

- `app/src/models/ingress_route.go`（編集）
    - **何を**: IngressRouteモデルの定義。host・path・tls_enabled・certificate_resolverフィールドを持つ。pending_*パターンを適用する。
    - **なぜ**: Traefik IngressRouteの設定をDBで管理するため

- `app/src/repository/ingress_route_repository.go`（新規作成）
    - **何を**: IngressRouteRepositoryインターフェースと実装。FindByDeploymentID・Create・Updateメソッドを持つ。
    - **なぜ**: IngressRouteのDB操作を抽象化するため

- `app/src/service/deployment_service.go`（編集）
    - **何を**: GetIngressRoute・CreateIngressRoute・UpdateIngressRouteメソッドをDeploymentServiceに追加。更新はpending_*フィールドへの書き込みのみ。
    - **なぜ**: IngressRoute設定のビジネスロジックをハンドラーから分離するため

- `app/src/handler/deployment_handler.go`（編集）
    - **何を**: GetIngressRoute・CreateIngressRoute・UpdateIngressRouteハンドラーの追加。
    - **なぜ**: IngressRoute設定管理のHTTPエントリーポイントが必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/POST/PUT /api/v1/deployments/:id/ingress-routeエンドポイントの登録。
    - **なぜ**: IngressRouteエンドポイントをルーターに接続するため

## テスト確認項目

- [ ] POST /api/v1/deployments/:id/ingress-routeでIngressRoute設定が作成できること
- [ ] GET /api/v1/deployments/:id/ingress-routeでIngressRoute設定が取得できること
- [ ] PUT /api/v1/deployments/:id/ingress-routeでpending_*フィールドが更新されること
- [ ] apply後にpending値が実際の値に昇格されること

### repository 層テスト

- [ ] IngressRouteRepository.FindByDeploymentIDでIngressRoute設定が取得できること
- [ ] IngressRouteRepository.Createで設定が作成できること
