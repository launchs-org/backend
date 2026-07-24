# PaaS 設計書 — テーブル定義（GORM）

---

## instance_sizes（グローバルマスター）

```go
type InstanceSize struct {
    Size          string `gorm:"primaryKey;type:varchar(16)"`
    CPURequest    string `gorm:"type:varchar(16);not null"`
    CPULimit      string `gorm:"type:varchar(16);not null"`
    MemoryRequest string `gorm:"type:varchar(16);not null"`
    MemoryLimit   string `gorm:"type:varchar(16);not null"`
}
```

| size   | cpu_request | cpu_limit | memory_request | memory_limit |
|--------|-------------|-----------|----------------|--------------|
| micro  | 50m         | 200m      | 64Mi           | 128Mi        |
| small  | 100m        | 500m      | 128Mi          | 256Mi        |
| medium | 250m        | 1000m     | 256Mi          | 512Mi        |
| large  | 500m        | 2000m     | 512Mi          | 1024Mi       |
| xlarge | 1000m       | 4000m     | 1024Mi         | 2048Mi       |

---

## user_quotas

認証は別サービスが担当し、このサービスには `user_id`（UUID文字列）だけが渡ってくる。
`UserID` を主キーとして quota を管理する。レコードが存在しない場合は初回アクセス時に upsert で作成する。

```go
type UserQuota struct {
    ID                       string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    UserID                   string    `gorm:"type:varchar(255);not null;uniqueIndex"`
    MaxProjects              int       `gorm:"not null;default:5"`
    MaxDeployments           int       `gorm:"not null;default:20"`
    MaxReplicasPerDeployment int       `gorm:"not null;default:5"`
    MaxVolumeMB              int       `gorm:"not null;default:10240"`
    CreatedAt                time.Time
    UpdatedAt                time.Time
}
```

---

## projects

```go
type ProjectStatus string

const (
    ProjectStatusProvisioning ProjectStatus = "provisioning"
    ProjectStatusActive       ProjectStatus = "active"
    ProjectStatusDeleting     ProjectStatus = "deleting"
)

type Project struct {
    ID       string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    UserID   string         `gorm:"type:varchar(255);not null;index"`
    Name     string         `gorm:"type:varchar(63);not null;uniqueIndex"`
    Namespace string         `gorm:"type:varchar(63);not null;uniqueIndex"`
    Status    ProjectStatus  `gorm:"type:varchar(32);not null;default:'provisioning'"`
    K8sStatus datatypes.JSON `gorm:"type:jsonb"` // null = 未同期
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

ステータス遷移: `provisioning → active` / `active → deleting`

---

## deployments

```go
type DeploymentType string

const (
    DeploymentTypeImageURL   DeploymentType = "image_url"
    DeploymentTypeDockerfile DeploymentType = "dockerfile"
    DeploymentTypeRailpack   DeploymentType = "railpack"
)

type DeploymentStatus string

const (
    DeploymentStatusPending  DeploymentStatus = "pending"  // 初回作成・未 apply
    DeploymentStatusRunning  DeploymentStatus = "running"
    DeploymentStatusFailed   DeploymentStatus = "failed"
    DeploymentStatusDeleting DeploymentStatus = "deleting"
)

type AppStatus string

const (
    AppStatusPending   AppStatus = "pending"
    AppStatusBuilding  AppStatus = "building"
    AppStatusDeploying AppStatus = "deploying"
    AppStatusRunning   AppStatus = "running"
    AppStatusError     AppStatus = "error"
)

type Deployment struct {
    ID        string           `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    ProjectID string           `gorm:"type:uuid;not null;index"`
    Name      string           `gorm:"type:varchar(63);not null"`
    Type      DeploymentType   `gorm:"type:varchar(32);not null"` // 作成後変更不可

    // --- image_url 専用 ---
    ImageURL        string `gorm:"type:text"`
    PendingImageURL string `gorm:"type:text"`

    // --- dockerfile / railpack 共通（GitHub）---
    GithubRepoURL        string `gorm:"type:text"`
    PendingGithubRepoURL string `gorm:"type:text"`

    GithubBranch        string `gorm:"type:varchar(255)"`
    PendingGithubBranch string `gorm:"type:varchar(255)"`

    // "HEAD" 指定可。apply 時に最新 SHA を取得して上書き
    GithubCommitSHA        string `gorm:"type:varchar(40)"`
    PendingGithubCommitSHA string `gorm:"type:varchar(40)"`

    // ビルド作業ディレクトリ。このディレクトリに CD した状態でビルドを開始する
    GithubRepoDirectory        string `gorm:"type:varchar(255);default:'./'"`
    PendingGithubRepoDirectory string `gorm:"type:varchar(255)"`

    // --- dockerfile 専用 ---
    DockerfilePath        string `gorm:"type:varchar(255);default:'./Dockerfile'"`
    PendingDockerfilePath string `gorm:"type:varchar(255)"`

    // --- ビルド管理 ---
    // 空文字 = ビルド中。完了時に build_id をセット
    CurrentBuildID string           `gorm:"type:uuid"`
    CurrentBuild   *DeploymentBuild `gorm:"foreignKey:CurrentBuildID"`

    // --- デプロイ設定 ---
    InstanceSize        string `gorm:"type:varchar(16);not null;default:'small'"`
    PendingInstanceSize string `gorm:"type:varchar(16)"`

    Replicas        int32 `gorm:"not null;default:1"`
    PendingReplicas int32

    // --- 起動設定 ---
    Command        pq.StringArray `gorm:"type:text[]"` // k8s command (ENTRYPOINT 上書き)
    PendingCommand pq.StringArray `gorm:"type:text[]"`

    Args        pq.StringArray `gorm:"type:text[]"` // k8s args (CMD 上書き)
    PendingArgs pq.StringArray `gorm:"type:text[]"`

    // --- ステータス ---
    Status    DeploymentStatus `gorm:"type:varchar(32);not null;default:'pending'"`
    AppStatus AppStatus        `gorm:"type:varchar(32);not null;default:'pending'"`
    K8sStatus datatypes.JSON   `gorm:"type:jsonb"` // null = 未同期
    AppliedAt *time.Time

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## deployment_builds

```go
type BuildStatus string

const (
    BuildStatusPending   BuildStatus = "pending"
    BuildStatusBuilding  BuildStatus = "building"
    BuildStatusSucceeded BuildStatus = "succeeded"
    BuildStatusFailed    BuildStatus = "failed"
    BuildStatusCancelled BuildStatus = "cancelled"
)

type DeploymentBuild struct {
    ID           string      `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    DeploymentID string      `gorm:"type:uuid;not null;index"`
    Status       BuildStatus `gorm:"type:varchar(32);not null;default:'pending'"`

    // k8s Job 名。キャンセル時に Job を削除するために使用
    K8sJobName string `gorm:"type:varchar(63)"`

    // ビルド成功時の push 先 URL
    BuiltImageURL string `gorm:"type:text"`

    // ビルド時点のソーススナップショット（HEAD は解決済みの実 SHA）
    CommitSHA     string `gorm:"type:varchar(40)"`
    CommitMessage string `gorm:"type:text"`
    Branch        string `gorm:"type:varchar(255)"`
    Author        string `gorm:"type:varchar(255)"`
    Directory     string `gorm:"type:varchar(255)"` // build_directory スナップショット
    DockerfilePath string `gorm:"type:varchar(255)"`

    // ビルドログ（k8s Job の Pod ログを収集して保存）
    BuildLog string `gorm:"type:text"`

    StartedAt  *time.Time
    FinishedAt *time.Time
    CreatedAt  time.Time
}
```

---

## apply_history

`POST /apply` が叩かれた瞬間に1レコード生成。

```go
type ApplyStatus string

const (
    ApplyStatusApplied ApplyStatus = "applied"
    ApplyStatusFailed  ApplyStatus = "failed"
)

type ApplyHistory struct {
    ID           string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    DeploymentID string         `gorm:"type:uuid;not null;index"`
    Manifests    datatypes.JSON `gorm:"type:jsonb;not null"` // 生成した k8s manifest 全スナップショット
    Status       ApplyStatus    `gorm:"type:varchar(32);not null"`
    ErrorMessage string         `gorm:"type:text"`
    AppliedAt    time.Time
}
```

---

## deployment_webhooks

```go
type DeploymentWebhook struct {
    ID           string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    DeploymentID string `gorm:"type:uuid;not null;index"`
    // HMAC 検証用シークレット（GitHub Webhook の Secret に設定する値）
    Secret    string    `gorm:"type:varchar(255);not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- Webhook URL: `POST /webhooks/{deployment_id}/github`
- push イベントを受け取り、push された branch が `deployment.github_branch` と一致する場合のみ自動 apply
- HMAC-SHA256 で署名を検証（`X-Hub-Signature-256` ヘッダー）

---

## services

```go
type ServiceStatus string

const (
    ServiceStatusPending  ServiceStatus = "pending"
    ServiceStatusActive   ServiceStatus = "active"
    ServiceStatusDeleting ServiceStatus = "deleting"
)

// [{"protocol": "TCP", "port": 8080}, {"protocol": "UDP", "port": 9090}]
type ServicePort struct {
    Protocol string `json:"protocol"`
    Port     int    `json:"port"`
}

type Service struct {
    ID           string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    DeploymentID string         `gorm:"type:uuid;not null;uniqueIndex"`
    Ports        datatypes.JSON `gorm:"type:jsonb"` // 現在稼働中
    PendingPorts datatypes.JSON `gorm:"type:jsonb"` // 未 apply
    Status       ServiceStatus  `gorm:"type:varchar(32);not null;default:'pending'"`
    K8sStatus    datatypes.JSON `gorm:"type:jsonb"` // null = 未同期
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

---

## ingress_routes

作成後すべてのフィールドは変更不可。Service とは独立して任意タイミングで作成する。

```go
type IngressRouteStatus string

const (
    IngressRouteStatusPending  IngressRouteStatus = "pending"
    IngressRouteStatusActive   IngressRouteStatus = "active"
    IngressRouteStatusDeleting IngressRouteStatus = "deleting"
)

type IngressRoute struct {
    ID        string             `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    ServiceID string             `gorm:"type:uuid;not null;uniqueIndex"` // Service と 1:1

    // 作成後変更不可。自動生成: {deployment_name}-{uuid8}.launchs.org
    Host       string `gorm:"type:varchar(253);not null;uniqueIndex"`
    PathPrefix string `gorm:"type:varchar(255);not null;default:'/'"`

    // 転送先の TCP ポート（Service の ports のうちいずれか）
    Port int `gorm:"not null"`

    Status    IngressRouteStatus `gorm:"type:varchar(32);not null;default:'pending'"`
    K8sStatus datatypes.JSON     `gorm:"type:jsonb"` // null = 未同期
    CreatedAt time.Time
}
```

---

## env_vars

`value` は即時更新（pending なし）。k8s への反映は関連 deployment の apply 時。

```go
type EnvVar struct {
    ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    ProjectID string `gorm:"type:uuid;not null;index"`
    Key       string `gorm:"type:varchar(255);not null"`
    Value     string `gorm:"type:text"` // 即時更新。k8s 反映は apply 時
    IsSecret  bool   `gorm:"not null;default:false"` // true → k8s Secret / UI マスク
    Status    string `gorm:"type:varchar(32);not null;default:'active'"` // active / deleting
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## env_var_mounts（中間テーブル）

```go
type EnvVarMountStatus string

const (
    EnvVarMountStatusPending  EnvVarMountStatus = "pending"  // mount 済み・未 apply
    EnvVarMountStatusApplied  EnvVarMountStatus = "applied"  // apply 済み
    EnvVarMountStatusDeleting EnvVarMountStatus = "deleting"
)

type EnvVarMount struct {
    ID           string            `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    EnvVarID     string            `gorm:"type:uuid;not null;index"`
    DeploymentID string            `gorm:"type:uuid;not null;index"`

    // NULL = 元の key をそのまま使用。指定時はコンテナ側でこの名前でマウント
    OverrideKey        string `gorm:"type:varchar(255)"`
    PendingOverrideKey string `gorm:"type:varchar(255)"`

    Status    EnvVarMountStatus `gorm:"type:varchar(32);not null;default:'pending'"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
// UNIQUE 制約: (env_var_id, deployment_id)
// apply 前チェック: 実効キー (COALESCE(override_key, key)) の重複をエラーで弾く
```

---

## volumes

作成後 `size_mb` は変更不可。PVC ReclaimPolicy = Delete。

```go
type VolumeStatus string

const (
    VolumeStatusPending  VolumeStatus = "pending"  // 作成済み・未 apply
    VolumeStatusBound    VolumeStatus = "bound"
    VolumeStatusDeleting VolumeStatus = "deleting"
)

type Volume struct {
    ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    ProjectID string         `gorm:"type:uuid;not null;index"`
    Name      string         `gorm:"type:varchar(63);not null"`
    SizeMB    int            `gorm:"not null"` // 作成後変更不可
    Status    VolumeStatus   `gorm:"type:varchar(32);not null;default:'pending'"`
    K8sStatus datatypes.JSON `gorm:"type:jsonb"` // null = 未同期
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## volume_mounts（中間テーブル）

```go
type VolumeMountStatus string

const (
    VolumeMountStatusPending  VolumeMountStatus = "pending"
    VolumeMountStatusMounted  VolumeMountStatus = "mounted"
    VolumeMountStatusDeleting VolumeMountStatus = "deleting"
)

type VolumeMount struct {
    ID           string            `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    VolumeID     string            `gorm:"type:uuid;not null;index"`
    DeploymentID string            `gorm:"type:uuid;not null;index"`
    MountPath        string `gorm:"type:varchar(255);not null"`
    PendingMountPath string `gorm:"type:varchar(255)"`
    Status    VolumeMountStatus `gorm:"type:varchar(32);not null;default:'pending'"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
// UNIQUE 制約: (volume_id, deployment_id)
```

---

## pending_*** まとめ

| テーブル | pending あり | pending なし |
|----------|-------------|-------------|
| deployments | image_url, github_repo_url, github_branch, github_commit_sha, github_repo_directory, dockerfile_path, instance_size, replicas, command, args | type, name |
| services | ports | - |
| env_vars | なし（value は即時更新） | - |
| env_var_mounts | override_key | - |
| volume_mounts | mount_path | - |
| volumes | なし（作成 = pending status） | size_mb（変更不可） |
| ingress_routes | なし（作成後変更不可） | - |

## API リクエストフィールド名規則まとめ

クライアントは常に `pending_` なしで送信。サーバーが `pending_***` カラムに書き込む。

| エンドポイント | リクエストフィールド | DB カラム |
|--------------|-------------------|---------|
| POST/PUT /deployments | `image_url` | `pending_image_url` |
| POST/PUT /deployments | `github_branch` | `pending_github_branch` |
| PUT /service | `ports` | `pending_ports` |
| PUT /env-mounts/:id | `override_key` | `pending_override_key` |
| PUT /volume-mounts/:id | `mount_path` | `pending_mount_path` |
