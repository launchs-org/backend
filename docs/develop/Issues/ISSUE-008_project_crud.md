# ISSUE-008 プロジェクトCRUD

## 親 Issue
ISSUE-005

## 概要
プロジェクトのCRUDエンドポイントを実装する。作成時はHarborプロジェクト・Robotアカウント作成とk8s Namespace作成をトランザクション的に行う。削除時はHarborとk8sリソースを順次削除する。

## 変更ファイル一覧

- `app/src/models/project.go`（編集）
    - **何を**: Projectモデルの定義。Status定数（provisioning/active/deleting）、NamespaceフィールドとUserIDフィールドを持つ。
    - **なぜ**: プロジェクトエンティティのDB表現を定義するため

- `app/src/repository/project_repository.go`（編集）
    - **何を**: ProjectRepositoryインターフェースと実装。Create・FindByID・FindAllByUserID・UpdateStatus・Save・Deleteメソッドを持つ。トランザクション引数（tx *gorm.DB）を受け取る。
    - **なぜ**: プロジェクトのDB操作を抽象化し、トランザクション管理をServiceに委譲するため

- `app/src/service/project_service.go`（編集）
    - **何を**: ProjectServiceインターフェースと実装。CreateProjectでHarbor連携・Namespace作成を含むトランザクション処理。DeleteProjectでHarborとk8sリソースの削除。ListProjects・GetProject・UpdateProjectのCRUD。GetProject・UpdateProject・DeleteProjectではProjectのUserIDとリクエストユーザーIDを比較し、不一致の場合はErrForbiddenを返す（ハンドラーで403に変換）。
    - **なぜ**: プロジェクト作成の複合オペレーション（Harbor + k8s + DB）をサービス層で調整するため。また、他ユーザーのプロジェクトへの不正アクセスを防ぐため

- `app/src/handler/project_handler.go`（編集）
    - **何を**: ListProjects・CreateProject・GetProject・UpdateProject・DeleteProjectハンドラーの実装。
    - **なぜ**: HTTPリクエストの受け取りとレスポンス返却を担う層が必要なため

- `app/src/router/router.go`（編集）
    - **何を**: GET/POST /api/v1/projects、GET/PUT/DELETE /api/v1/projects/:idエンドポイントの登録。
    - **なぜ**: プロジェクトCRUDエンドポイントをルーターに接続するため

- `app/src/main.go`（編集）
    - **何を**: ProjectServiceとProjectHandlerのDI組み立てとNewRouter()への注入。
    - **なぜ**: 依存関係をエントリーポイントで組み立てるため

## テスト確認項目

- [ ] POST /api/v1/projectsでプロジェクトが作成されること
- [ ] 作成時にHarborプロジェクトとRobotアカウントが作成されること
- [ ] 作成時にk8s Namespaceが作成されること
- [ ] GET /api/v1/projectsで自分のプロジェクト一覧が取得できること
- [ ] GET /api/v1/projects/:idでプロジェクトが取得できること
- [ ] PUT /api/v1/projects/:idでプロジェクト名が更新できること
- [ ] DELETE /api/v1/projects/:idでHarborとk8sリソースが削除されること
- [ ] 他ユーザーのプロジェクトにGETすると403が返ること
- [ ] 他ユーザーのプロジェクトをPUTすると403が返ること
- [ ] 他ユーザーのプロジェクトをDELETEすると403が返ること

### repository 層テスト

- [ ] ProjectRepository.Createでプロジェクトが作成できること
- [ ] ProjectRepository.FindByIDでプロジェクトが取得できること
- [ ] ProjectRepository.FindAllByUserIDでユーザーのプロジェクト一覧が取得できること
- [ ] ProjectRepository.UpdateStatusでstatusが更新できること
