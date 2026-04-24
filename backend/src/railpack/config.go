package railpack

import "time"

// BuildConfig はビルドに必要な全設定をまとめた構造体です。
// コンストラクタ New() に渡すことでビルドパイプラインを設定します。
type BuildConfig struct {
	// ── Git ソース ──────────────────────────────────────────
	// GitRepo: クローンするリポジトリのURL (例: "https://github.com/org/repo")
	GitRepo string
	// GitBranch: チェックアウトするブランチ名 (省略時: "main")
	GitBranch string
	// Subdir: ビルドコンテキストのサブディレクトリ (省略時: ".")
	Subdir string
	// GitSubmodules: true にすると git submodule も再帰的にクローンします
	GitSubmodules bool

	// ── 成果物 ──────────────────────────────────────────────
	// ImageName: プッシュ先のイメージ名 (例: "my-app")
	ImageName string
	// ImageTag: イメージのタグ (例: "v1.0.0")
	ImageTag string

	// ── tar 送信先 ───────────────────────────────────────────
	// UploadEndpoint: ビルド成果物 (tar) の送信先エンドポイント
	// 例: "http://10.10.11.8:8080/upload"
	UploadEndpoint string
	// UploadToken: 送信時の Bearer 認証トークン (空なら認証なし)
	UploadToken string

	// ── Kubernetes ───────────────────────────────────────────
	// Namespace: Job を作成する Kubernetes namespace
	Namespace string

	// ── リソース制限 ─────────────────────────────────────────
	// Resources: 各コンテナのリソース設定 (省略時: DefaultResourceConfig())
	Resources ResourceConfig

	// ── タイムアウト ─────────────────────────────────────────
	// Timeout: ビルド全体のタイムアウト (省略時: 10分)
	Timeout time.Duration
}

// ResourceConfig は各コンテナのリソース制限設定です。
// 省略すると DefaultResourceConfig() の値が使われます。
type ResourceConfig struct {
	// buildctl コンテナ (ビルド本体) — 重い処理のため大きめに設定
	BuildCPU    string // 例: "2"
	BuildMemory string // 例: "2Gi"
	BuildDisk   string // 例: "1Gi" (emptyDir の上限にも使用)

	// InitContainer (git-clone / railpack) — 軽量処理
	InitCPU    string // 例: "500m"
	InitMemory string // 例: "512Mi"

	// tar-push コンテナ (curl のみ) — 最小限
	PushCPU    string // 例: "100m"
	PushMemory string // 例: "128Mi"
}

// DefaultResourceConfig は一般的なビルドに適したデフォルトのリソース設定を返します。
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		BuildCPU:    "2",
		BuildMemory: "2Gi",
		BuildDisk:   "1Gi",
		InitCPU:     "500m",
		InitMemory:  "512Mi",
		PushCPU:     "100m",
		PushMemory:  "128Mi",
	}
}

// applyDefaults は省略された設定項目にデフォルト値を適用します。
func applyDefaults(cfg BuildConfig) BuildConfig {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute
	}
	if cfg.GitBranch == "" {
		cfg.GitBranch = "main"
	}
	if cfg.Subdir == "" {
		cfg.Subdir = "."
	}
	if cfg.Resources.BuildCPU == "" {
		cfg.Resources = DefaultResourceConfig()
	}
	return cfg
}
