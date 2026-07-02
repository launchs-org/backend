# Image モデル分離（ビルド/イメージ/デプロイメントの責務整理）実装計画

## Context（背景・目的）

現状、「ビルド」と「イメージ」の概念が `DeploymentBuild` モデル1つに同居しており、責務が曖昧になっている。

- railpack が buildkit 経由で Harbor にイメージを push するところまでが「ビルド」であり、その成果物が「イメージ」である。しかし現状は独立した `Image` モデルが存在せず、成果物情報（`BuiltImageURL`, `ImageSizeBytes`）が `DeploymentBuild`（ビルド実行の記録）に埋め込まれている。
- `Deployment` は実際に使うイメージを `ImageURL`/`PendingImageURL`（文字列）で保持しつつ、`CurrentBuildID`（ビルドへのFK）も持つという二重管理状態になっている。
- 本来「イメージがどこかの Deployment から参照されていれば削除できない」という制約が必要だが、現状の唯一の削除API（`DELETE /builds/:buildId`）は参照先の `CurrentBuildID` を単に NULL 化して削除を強行しており、安全装置になっていない。
- フロントエンドのプロジェクト詳細ページには「ビルド管理」サイドバーがあるが、ユーザーが本当に管理したいのは「イメージ」であり、ビルドはその過程に過ぎない。

この変更により、「ビルド＝実行記録」「イメージ＝成果物」を明確に分離したモデルに直し、イメージ削除時に参照整合性を保証し、UIもイメージ中心の管理に置き換える。

調査の過程で以下の追加事実が判明し、計画に組み込んだ：
- `handler/` は `shared/` とは別の独立 Go module であり、models/repository を重複コピーしている（両方の編集が必要）。
- ビルド完了検知は `builder/`（Temporal Activity、本番稼働）と `watcher/`（k8s Job Watch、本番稼働）の2系統が並行して動いている。`handler/src/k8s/build.go` は同等ロジックの3つ目のコピーだが `main.go` から未使用（削除する）。
- Harbor の `DeleteHarborImage`／`GetArtifactSize` は現状どちらも「タグ指定なしのリポジトリ単位」で操作しており、Deployment単位のリポジトリ（`{project}/{deploymentID}`、タグ=`buildID`）に対して実行されるため、**1つのビルド（イメージ）を削除・サイズ取得すると同じDeploymentの他ビルドのイメージも巻き添えになるバグ**が実在する。これは今回のImage独立化の要件（他から参照されないイメージだけを安全に消せる）と直接衝突するため、本計画でタグ単位に修正する。

---

## 1. 新規 `Image` モデル

### 対象ファイル（2箇所、内容同一・import パスのみ差異）
- `shared/models/image.go`（新規作成）
- `handler/src/models/image.go`（新規作成）

```go
package models

import "time"

// Image はビルド成果物（Harbor に push されたコンテナイメージ）を表すモデル
// DeploymentBuild が「ビルド実行の記録」であるのに対し、Image は「成果物」として完全に分離する
type Image struct {
	ID        string           `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID string           `gorm:"type:uuid;not null;index"                       json:"project_id"` // 親プロジェクトID（Deployment削除後もImageを保持するため）

	// BuildID は成果物を生んだビルドのID（railpack/dockerfileビルド経由の場合のみ設定）。
	// image_url タイプ（外部イメージ直接指定）の Deployment 用に作られる Image は nil になる
	BuildID *string          `gorm:"type:uuid;index"           json:"build_id"`
	Build   *DeploymentBuild `gorm:"foreignKey:BuildID"        json:"build,omitempty"`

	ImageURL  string `gorm:"type:text;not null" json:"image_url"`  // Harbor 上のイメージ URL、または外部指定URL（旧 BuiltImageURL）
	SizeBytes int64  `gorm:"default:0"          json:"size_bytes"` // Harbor に格納されたイメージサイズ（バイト単位、旧 ImageSizeBytes）

	CreatedAt time.Time `json:"created_at"`
}

func (Image) TableName() string { return "images" }
```

補足:
- `ProjectID` は `DeploymentBuild` と同じ設計思想（Deployment 削除後もプロジェクト単位でイメージ一覧を見られるようにするため）。
- `BuildID` は nullable。既存コードの `CurrentBuildID`/`DeploymentID` と同様、明示的な DB `FOREIGN KEY` 制約は付与しない（GORM の `foreignKey` タグは Go 側のリレーション定義のみ）。

---

## 2. `DeploymentBuild` モデルの変更

### 対象ファイル
- `shared/models/deployment_build.go`（編集）
- `handler/src/models/deployment_build.go`（編集）

`BuiltImageURL`・`ImageSizeBytes` フィールドを削除する（Image モデルへ移設済み）。既存DBカラムは残留するが GORM からは読み書きされなくなる（合意事項7: 既存データ移行は考慮しない）。

---

## 3. `Deployment` モデルの変更

### 対象ファイル
- `shared/models/deployment.go`（編集）
- `handler/src/models/deployment.go`（編集）

`ImageURL`/`PendingImageURL`（文字列）を `ImageID`/`PendingImageID`（FK文字列ポインタ）に置き換える。

```go
// --- イメージ参照（image_url / railpack / dockerfile 共通）---
// nil = イメージ未設定。apply 時に Image レコードを引いて実URLをk8sに適用する
ImageID        *string `gorm:"type:uuid" json:"image_id"`
Image          *Image  `gorm:"foreignKey:ImageID"        json:"image,omitempty"`
PendingImageID *string `gorm:"type:uuid" json:"pending_image_id"`
PendingImage   *Image  `gorm:"foreignKey:PendingImageID" json:"pending_image,omitempty"`
```

`CurrentBuildID`/`CurrentBuild` は変更なし（今回のスコープ外、ビルド実行記録への参照として現状維持）。

**呼び出し側への影響**: 既存の `ImageURL string` は「空文字列 = 未設定」という判定パターンで使われている（`CreateDeployment`、`DiscardPending`、`ExecuteApply`）。`*string` 化に伴い、これらはすべて `== nil` 判定に置き換える（9・14節で詳述）。

---

## 4. 新規 `ImageRepository`

### 対象ファイル
- `shared/repository/image_repository.go`（新規作成）
- `handler/src/repository/image_repository.go`（新規作成、import パスのみ差異）

```go
type ImageRepository interface {
	Create(ctx context.Context, image *models.Image) error                          // イメージレコードを作成する
	FindByID(ctx context.Context, imageID string) (*models.Image, error)            // ID でイメージレコードを取得する
	FindByBuildID(ctx context.Context, buildID string) (*models.Image, error)       // buildID に紐づくイメージレコードを取得する（1ビルド1イメージの想定）
	FindAllByProjectID(ctx context.Context, projectID string) ([]models.Image, error) // projectID に紐づくイメージ一覧を取得する（Build を Preload して返す）
	UpdateSizeBytes(ctx context.Context, imageID string, sizeBytes int64) error      // Harbor から取得したイメージサイズを更新する
	Delete(ctx context.Context, image *models.Image) error                          // イメージレコードを1件削除する
}
```

`deployment_build_repository.go` と同一パターン（`db *gorm.DB` を保持する struct、`WithContext(ctx)`）で実装する。`FindAllByProjectID` は `Preload("Build")` して返す（17節のフロント要件：一覧にコミット情報を表示するため）。

---

## 5. `DeploymentBuildRepository` の変更

### 対象ファイル
- `shared/repository/deployment_build_repository.go`（編集）
- `handler/src/repository/deployment_build_repository.go`（編集）

`UpdateBuildResult` は「ビルド実行記録」の更新責務のみに絞り、引数から `builtImageURL string, imageSizeBytes int64` を除去する。

```go
// 変更後
UpdateBuildResult(ctx context.Context, buildID string, status models.BuildStatus, finishedAt time.Time) error
```

Image 作成は `DeploymentBuildRepository` の責務にせず、呼び出し元（`builder/src/activity/build_activity.go`, `watcher/src/k8s/build.go`）が `ImageRepository` を直接使う（Repository 同士を依存させない）。

---

## 6. `DeploymentRepository` の変更

### 対象ファイル
- `shared/repository/deployment_repository.go`（編集）
- `handler/src/repository/deployment_repository.go`（編集）

`UpdatePendingImageURL(ctx, deploymentID, imageURL string)` を `UpdatePendingImageID(ctx, deploymentID, imageID string)` に置き換える（`pending_image_id` カラムを更新）。`UpdateCurrentBuildID`/`ClearCurrentBuildID` は変更なし。

---

## 7. 新規 `ImageService`（参照チェック・Harborタグ単位削除）

### 対象ファイル
- `handler/src/service/image_service.go`（新規作成。Service層は handler プロセスのみに存在するため shared への複製は不要）

```go
var ErrImageInUse = errors.New("image is currently referenced by a deployment")

type ImageService interface {
	ListImagesByProject(ctx context.Context, userID string, projectID string) ([]models.Image, error)
	GetImage(ctx context.Context, userID string, imageID string) (*models.Image, error)
	DeleteImage(ctx context.Context, userID string, projectID string, imageID string) error
}
```

`DeleteImage` の実装方針:
1. 所有権チェック（project.UserID と一致するか）
2. Image を取得し、`imageData.ProjectID != projectID` なら `ErrForbidden`
3. **参照チェック**: `projectID` 配下の全 Deployment を取得し、`Deployment.ImageID` または `Deployment.PendingImageID` が対象 `imageID` と一致するものが1つでもあれば `ErrImageInUse` を返す（現在使用中かどうかのみで判定。過去の履歴上の紐付けは見ない）。このチェックは `BuildID` の有無に関わらず共通で行う。
4. **Harbor削除の分岐（`imageData.BuildID` の有無で分岐する）**:
   - `BuildID` が非nil（railpack/dockerfileビルド経由、Harbor上に実体が存在する）: 8節で修正した `DeleteHarborImage` をタグ単位で呼ぶ。`buildRepo.FindByID` でビルドレコードを取得し、`repositoryName = *buildData.DeploymentID`（nilなら `buildData.ID`）、`tag = buildData.ID` を渡す。
   - `BuildID` が nil（DockerHub等、外部URL直接指定でHarborに実体が存在しない）: **Harbor API は一切呼び出さない**。これはアプリ側の登録を消すだけの操作であり、外部レジストリ上の実イメージは削除されず残り続ける（削除不可能なため）。フロントの削除確認ダイアログでもこの旨をユーザーに明示することが望ましい（17節UI検討事項に追記）。
5. DB レコード (`images` テーブル) を削除する（4で分岐したどちらのケースでも実行する）

---

## 8. Harbor クライアントのタグ単位削除への修正

現状 `DeleteHarborImage`／`GetArtifactSize` は3箇所（`handler/src/k8s/harbor.go`, `controller/src/k8s/harbor.go`, `watcher/src/k8s/harbor.go`、いずれもL400-452相当）に同一実装がコピーされており、いずれも **タグ指定なしでリポジトリ全体**を対象にしている。Harborのリポジトリ構造はDeployment単位（`{project}/{deploymentID}`）で、複数ビルドが `buildID` をタグとして同一リポジトリに同居するため、現状の実装のままだと1つの Image を削除・サイズ取得すると同じ Deployment の他ビルドの Image も巻き添えになる（実在するバグ、ユーザー確認済みで今回修正する）。

### 対象ファイル（3箇所とも同様に修正）
- `handler/src/k8s/harbor.go`
- `controller/src/k8s/harbor.go`
- `watcher/src/k8s/harbor.go`
- （builder側は `builder/src/k8s/harbor.go` に `GetArtifactSize` 相当があるので同様に確認・修正する）

### `DeleteHarborImage` の変更
```go
// 変更前: DELETE /api/v2.0/projects/{project}/repositories/{repository}
// 変更後: タグ（reference）を引数で受け取り、そのアーティファクトのみ削除する
func (client *HarborClient) DeleteHarborImage(ctx context.Context, projectName string, repositoryName string, tag string, credential HarborRobotCredential) error {
	deleteURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s", client.endpoint, projectName, repositoryName, tag) // タグ指定のアーティファクト削除URLを組み立てる
	// 以降のリクエスト生成・送信処理は変更なし
}
```

### `GetArtifactSize` の変更
```go
// 変更前: GET /api/v2.0/projects/{project}/repositories/{repository}/artifacts?page_size=100 → 全アーティファクト合計
// 変更後: 特定タグのアーティファクト1件のみ取得してそのサイズを返す
func (client *HarborClient) GetArtifactSize(ctx context.Context, projectName string, repositoryName string, tag string, credential HarborRobotCredential) (int64, error) {
	artifactURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s", client.endpoint, projectName, repositoryName, tag) // タグ指定のアーティファクト取得URLを組み立てる
	// レスポンスは単一オブジェクトになるため配列デコードをやめ、単一 harborArtifact にデコードして Size を返す
}
```

呼び出し元はすべて `tag = buildData.ID`（`buildBuiltImageURL` でタグに使っているのと同じ値）を渡すよう修正する（11, 13, 7節で反映）。

---

## 9. `BuildService` の変更

### 対象ファイル
- `handler/src/service/build_service.go`（編集）

1. `DeleteBuild` メソッドをインターフェース・実装ともに削除する（Image削除に一本化）。
2. `harborClient *k8s.HarborClient` フィールドは `DeleteBuild` 専用の依存だったため削除し、`NewBuildService` のシグネチャからも除去する。
3. ビルド成功時の Image 作成は `build_service.go` の責務ではない（`builder/`・`watcher/` が担う）。
4. `TriggerBuild` 内のロールバック呼び出し `UpdateBuildResult(ctx, buildData.ID, models.BuildStatusFailed, "", 0, finishedAt)` を新シグネチャ `UpdateBuildResult(ctx, buildData.ID, models.BuildStatusFailed, finishedAt)` に修正する。

変更後のインターフェース:
```go
type BuildService interface {
	TriggerBuild(ctx context.Context, userID string, deploymentID string, commitMessage string, author string) (*models.DeploymentBuild, error)
	CancelBuild(ctx context.Context, userID string, buildID string) error // 変更なし（今回のスコープ外）
	GetBuild(ctx context.Context, userID string, buildID string) (*models.DeploymentBuild, error)
	GetBuildLogs(ctx context.Context, userID string, buildID string, since *time.Time) (string, *time.Time, error)
	ListBuilds(ctx context.Context, userID string, deploymentID string) ([]models.DeploymentBuild, error)
	ListBuildsByProject(ctx context.Context, userID string, projectID string) ([]models.DeploymentBuild, error)
}
```

---

## 10. `DeploymentService` の変更

### 対象ファイル
- `handler/src/service/deployment_service.go`（編集）

`image_url` タイプ（外部イメージ直接指定）も Image テーブル経由で管理する（`Image.BuildID = nil` として作成）。

#### `CreateDeployment`
```go
if req.ImageURL != "" { // 外部イメージURL直接指定の場合は Image レコードを作成する
	imageData := &models.Image{
		ProjectID: req.ProjectID,
		BuildID:   nil, // ビルドを経由しない直接指定のため nil
		ImageURL:  req.ImageURL,
	}
	if err := svc.imageRepo.Create(ctx, imageData); err != nil {
		return nil, err
	}
	deploymentData.PendingImageID = &imageData.ID
}
```
`deploymentServiceImpl` に `imageRepo repository.ImageRepository` フィールドを追加し、`NewDeploymentService` の引数にも追加する。

#### `UpdateDeployment`
`req.ImageURL != nil` の分岐で同様に Image レコードを作成してから `PendingImageID` にセットする。

#### `DiscardPending`
```go
// 変更前: deploymentData.PendingImageURL = deploymentData.ImageURL
// 変更後: deploymentData.PendingImageID = deploymentData.ImageID  （ポインタのコピーなのでそのまま代入可）
```

railpack/dockerfile ビルド経由の Image 作成・`PendingImageID` セットは `CreateDeployment`/`UpdateDeployment` では発生せず、ビルド成功イベント側（11, 13節）で行われる。

---

## 11. `handler/src/k8s/build.go` の削除

`handler/src/main.go` から一切呼び出されていない未使用コード（`watcher/src/k8s/build.go` と同等ロジックの重複）。モデル変更でコンパイルエラーになるため削除する。

### 対象ファイル
- `handler/src/k8s/build.go`（削除）
- `handler/src/k8s/build_test.go`（存在すれば削除。実装時に確認する）

---

## 12. `builder/src/activity/build_activity.go` の変更（本番ビルド成功パス・その1）

### 対象ファイル
- `builder/src/activity/build_activity.go`（編集）
- `builder/src/workflow/build_workflow.go`（編集）

`BuildActivities` に `ImageRepo repository.ImageRepository` を追加する。

`SetPendingImageURLActivity` → `SetPendingImageActivity` にリネームし、Image レコードを作成してから `DeploymentRepo.UpdatePendingImageID` を呼ぶ形に変更、戻り値を `builtImageURL string` から `imageID string` に変更する。

`UpdateBuildStatusActivity` の引数 `builtImageURL string` を `imageID string` に変更する。ビルド成功時のみ `ImageRepo.FindByID(imageID)` で Image を引き、8節で修正した `HarborClient.GetArtifactSize(ctx, projectID, repositoryName, buildData.ID /* tag */, credential)` でサイズ取得して `ImageRepo.UpdateSizeBytes` を呼ぶ。`BuildRepo.UpdateBuildResult` は新シグネチャ（5節）で呼ぶ。

`build_workflow.go` は Activity名・変数名のリネームに追従する（`"SetPendingImageURLActivity"` の文字列参照も含めて `"SetPendingImageActivity"` に変更する）。

---

## 13. `watcher/src/k8s/build.go` の変更（本番ビルド成功パス・その2）

### 対象ファイル
- `watcher/src/k8s/build.go`（編集）
- `watcher/src/main.go`（編集）

`handleBuildJobEvent`・`buildBuiltImageURL`・`fetchImageSizeBytes` は 12節と同等の変更を行う：
- 成功分岐で Image レコードを作成 → `imageRepo.Create`
- `fetchImageSizeBytes` は8節で修正した `GetArtifactSize` にタグ（`buildData.ID`）を渡すよう修正し、取得したサイズを `imageRepo.UpdateSizeBytes` で更新
- `buildRepo.UpdateBuildResult` を新シグネチャで呼ぶ
- 成功時 `deploymentRepo.UpdatePendingImageID(ctx, deploymentID, imageData.ID)` を呼ぶ
- 失敗分岐も新シグネチャの `UpdateBuildResult` に追従する

`WatchBuildJobs` のシグネチャに `imageRepo repository.ImageRepository` を追加し、`watcher/src/main.go` の呼び出し箇所で `repository.NewImageRepository(repository.Database)` を生成して渡す。

---

## 14. `controller/src/activity/apply_activity.go` の変更（apply時のURL解決）

### 対象ファイル
- `controller/src/activity/apply_activity.go`（編集）
- `controller/src/main.go`（編集: DI追加）
- `controller/src/k8s/harbor.go`（8節の修正、Harborタグ削除に伴い他にHarbor呼び出しがあれば同様に確認）

`ApplyActivities` に `ImageRepo repository.ImageRepository` を追加する。`ExecuteApply` 内の image URL 解決ロジックを変更する:

```go
// 変更前
imageURL := deploymentData.PendingImageURL
if imageURL == "" {
	imageURL = deploymentData.ImageURL
}

// 変更後
imageID := deploymentData.PendingImageID
if imageID == nil {
	imageID = deploymentData.ImageID
}
var imageURL string
if imageID != nil {
	imageData, imgErr := activities.ImageRepo.FindByID(ctx, *imageID)
	if imgErr != nil {
		return fmt.Errorf("image の取得に失敗しました: %w", imgErr)
	}
	imageURL = imageData.ImageURL
}
```

pending→current 昇格処理（`deploymentData.ImageURL = deploymentData.PendingImageURL` 相当の箇所）も `ImageID = PendingImageID` に置き換える。実装時に `apply_activity.go` 全文を検索して該当箇所を特定する。

`controller/src/activity/deployment_activity.go` の `ClearCurrentBuildID` 呼び出しは変更しない（CurrentBuildID は今回対象外）。

---

## 15. 新規 `ImageHandler` とルーティング

### 対象ファイル
- `handler/src/handler/image_handler.go`（新規作成）
- `handler/src/handler/build_handler.go`（編集: `DeleteBuild` ハンドラー削除）
- `handler/src/router/router.go`（編集）
- `handler/src/main.go`（編集）

`ImageHandler` は `BuildHandler` と同型パターンで実装する:
- `ListImagesByProject` — `GET /api/v1/projects/:id/images`
- `GetImage` — `GET /api/v1/images/:imageId`
- `DeleteImage` — `DELETE /api/v1/projects/:id/images/:imageId`（`ErrImageInUse` は 409 Conflict、メッセージ「このイメージは Deployment から参照されているため削除できません」）

ルーティング変更:
- 追加: 上記3エンドポイント
- 削除: `DELETE /api/v1/projects/:id/builds/:buildId`（`DeleteBuild`）

`main.go` の DI 組み立て:
- `imageRepo := repository.NewImageRepository(repository.Database)` を生成
- `buildServiceImpl := service.NewBuildService(...)` から `harborClient` 引数を削除
- `imageServiceImpl := service.NewImageService(imageRepo, deploymentRepo, projectRepo, harborCredentialRepo, buildRepo, harborClient)` を生成
- `imageHandler := handler.NewImageHandler(imageServiceImpl)` を生成
- `deploymentServiceImpl := service.NewDeploymentService(...)` に `imageRepo` を追加
- `router.New(...)` の `RouterOptions` に `ImageHandler: imageHandler` を追加

---

## 16. マイグレーション

### 対象ファイル
- `shared/repository/db.go`（編集、`AutoMigrate`）
- `handler/src/repository/db.go`（編集、同一内容）

`AutoMigrate` の対象リストに `&models.Image{}` を `&models.DeploymentBuild{}` の直後に追加する。既存の `CurrentBuildID` 等と同様、DB制約としての FK は生成されないため順序依存はない（実装時に `docker compose exec app` でマイグレーションを実行しエラーが出ないことを確認する）。

既存の `deployment_builds.built_image_url`/`image_size_bytes`、`deployments.image_url`/`pending_image_url` カラムは DB 上に残留するが GORM からは読み書きされなくなる（無害な残留カラム、合意事項7に基づき許容）。

---

## 17. フロントエンド変更

### 17.1 型定義: `frontend/src/lib/types.ts`（編集）
- `Build` 型から `built_image_url`/`image_size_bytes` を削除。
- 新規 `Image` 型を追加: `{ id, project_id, build_id: string | null, image_url, size_bytes, created_at, build?: Build }`。
- `Deployment` 型の `image_url`/`pending_image_url` を `image_id`/`pending_image_id`（+ 任意で `image`/`pending_image` ネスト）に変更。

### 17.2 `ProjectDetailPage.tsx`: 「ビルド管理」→「イメージ管理」への置き換え
- `showBuildSidebar` → `showImageSidebar`、`buildList: Build[]` → `imageList: Image[]`。API呼び出しを `GET /projects/:id/builds` → `GET /projects/:id/images` に変更。
- サイドバーのラベルを「ビルド」→「イメージ」に変更（アイコンは `Package` のまま流用可）。
- `BuildSidebar` → `ImageSidebar` にリネームし、一覧表示を Image のフィールド（`image_url`, `size_bytes`, `created_at`）＋ Preload された `build`（branch/commit_sha/commit_message/author）中心に変更する。
- 削除ボタンは `DELETE /projects/:id/images/:imageId` を呼ぶ。409（`ErrImageInUse`）時は「このイメージは使用中のため削除できません」というトーストを表示する。対象 Image の `build_id` が null（DockerHub等の外部URL直接指定）の場合、削除確認ダイアログに「このイメージはアプリ上の登録のみ削除されます。外部レジストリ上の実イメージは削除されません」という趣旨の注記を表示する（7節のHarbor削除スキップ仕様に対応、ユーザーが誤解しないようにするため）。
- 「このイメージでデプロイ」ボタンは新規デプロイメント作成フォームに `image_url`（Image.ImageURL）を引き継ぐ（`CreateDeploymentRequest.ImageURL` は文字列のまま維持、10節の通りサービス側で新規 Image レコードが作られる）。
- Harbor ストレージ使用量表示ブロックは変更不要（独立した `ProjectQuota` API）。
- 各 Image カードに「ビルドログを見る」リンクを残す（`image.build_id` が非nullの場合のみ表示、遷移先はビルドログ取得API）。独立した「ビルド一覧」UIとしては撤去しつつ、ログ参照手段は維持する。

### 17.3 `DeploymentDetailPage.tsx`: `BuildsTab` → `ImagesTab` への置き換え
- タブ名を「ビルド」→「イメージ」に変更する。
- `GET /deployments/:id/builds` の代わりに Image ベースのデータを取得する。既存 API に deployment 単位の絞り込みがないため、`ListImagesByProject` のレスポンスを `image.build.deployment_id` でフロント側フィルタするか、`GET /projects/:id/images?deployment_id=` のクエリパラメータ対応を `ImageRepository.FindAllByProjectID` に追加するかを実装時に選ぶ（後者を推奨: バックエンドでフィルタする方が効率的）。
- 「このビルドをデプロイ」ボタンは `image.image_url` を参照する形に変更する（現状 `buildItem.built_image_url` 参照 → コンパイルエラーになるため必須対応）。

---

## 18. 実装順序（推奨）

1. `shared/models/image.go` → `handler/src/models/image.go` 新規作成
2. `shared/models/deployment_build.go` / `handler/src/models/deployment_build.go` 編集（フィールド削除）
3. `shared/models/deployment.go` / `handler/src/models/deployment.go` 編集（ImageID化）
4. `shared/repository/image_repository.go` / `handler/src/repository/image_repository.go` 新規作成
5. `shared/repository/deployment_build_repository.go` / `handler/src/repository/deployment_build_repository.go` 編集
6. `shared/repository/deployment_repository.go` / `handler/src/repository/deployment_repository.go` 編集
7. `shared/repository/db.go` / `handler/src/repository/db.go` の AutoMigrate 更新
8. `handler/src/k8s/harbor.go`, `controller/src/k8s/harbor.go`, `watcher/src/k8s/harbor.go`, `builder/src/k8s/harbor.go` のタグ単位削除・サイズ取得への修正
9. `builder/src/activity/build_activity.go` / `builder/src/workflow/build_workflow.go` 編集
10. `watcher/src/k8s/build.go` / `watcher/src/main.go` 編集
11. `controller/src/activity/apply_activity.go` / `controller/src/main.go` 編集
12. `handler/src/k8s/build.go` 削除
13. `handler/src/service/image_service.go` 新規作成
14. `handler/src/service/build_service.go` 編集（DeleteBuild削除、harborClient除去）
15. `handler/src/service/deployment_service.go` 編集（Image作成呼び出し追加）
16. `handler/src/handler/image_handler.go` 新規作成、`handler/src/handler/build_handler.go` 編集
17. `handler/src/router/router.go` / `handler/src/main.go` 編集（DI・ルーティング）
18. 各層のテスト実装・修正（handler/service/repository 3層、CLAUDE.md規約に準拠：1文字変数禁止・日本語コメント必須）
19. フロントエンド変更（`types.ts`, `ProjectDetailPage.tsx`, `DeploymentDetailPage.tsx`）
20. `docker compose exec app go build ./...` 相当のビルド確認を各モジュール（handler/builder/controller/watcher）で実施

モデル変更は全モジュールに波及するため、1〜12までは一括で変更しないとどのモジュールもビルドが通らない点に注意。

---

## 19. 検証方法

1. `docker compose exec app go build ./...` を `handler/`, `builder/`, `controller/`, `watcher/` それぞれで実行し、コンパイルが通ることを確認する。
2. `docker compose exec app go vet ./...` で静的解析を通す。
3. `task test-all`（または `docker compose exec app go test ./...`）で新規実装した `ImageRepository`/`ImageService`/`ImageHandler` のテスト、既存 `BuildService`/`DeploymentService` の修正済みテストが全件パスすることを確認する。
4. 手動確認シナリオ:
   - railpack ビルドをトリガー → 成功後 `images` テーブルにレコードが作成され、`deployments.pending_image_id` が更新されることを確認する。
   - apply を実行し、Image の URL が実際に k8s Deployment に反映されることを確認する。
   - 使用中の Image に対して `DELETE /projects/:id/images/:imageId` を呼び、409 が返り削除されないことを確認する。
   - 参照されていない古い Image を削除し、200/204 が返り Harbor 上の該当タグのみ消え、同一 Deployment の他ビルドのイメージが巻き添えで消えないことを確認する（8節のタグ単位修正の検証）。
   - フロントの「イメージ管理」サイドバーが一覧・削除・デプロイ操作について正しく動作することをブラウザで確認する。
