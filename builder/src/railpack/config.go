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

	// ── アーカイブソース（zip/tar.gzアップロード）──────────────
	// SourceType: "git" または "archive"。省略時は ArchiveURL の有無から自動判定
	SourceType string
	// ArchiveURL: file.io のダウンロードリンク（GitRepo の代わりに使用）
	ArchiveURL string
	// ArchiveEncKeyHex: AES-256-CBC復号鍵（16進エンコード）
	ArchiveEncKeyHex string
	// ArchiveSHA256Hex: 暗号文全体のSHA256ハッシュ（16進、破損・改竄検知用）
	ArchiveSHA256Hex string

	// ── 成果物 ──────────────────────────────────────────────
	// ImageName: プッシュ先のイメージ名 (例: "my-app")
	ImageName string
	// ImageTag: イメージのタグ (例: "v1.0.0")
	ImageTag string

	// ── レジストリ (直接プッシュ) ────────────────────────────
	// RegistryHost: プッシュ先レジストリホスト (例: "harbor.main-harbor")
	RegistryHost string
	// RegistryProject: Harbor のプロジェクト名 (例: "buildkit")
	RegistryProject string
	// RegistryUsername: buildkit が使う Harbor ユーザー名
	RegistryUsername string
	// RegistryPassword: buildkit が使う Harbor パスワード
	RegistryPassword string
	// RegistryInsecure: true にすると HTTP / 自己署名証明書を許可
	RegistryInsecure bool

	// ── Kubernetes ───────────────────────────────────────────
	// Namespace: Job を作成する Kubernetes namespace
	Namespace string

	// ── リソース制限 ─────────────────────────────────────────
	// Resources: 各コンテナのリソース設定 (省略時: DefaultResourceConfig())
	Resources ResourceConfig
	
	// ジョブを識別するID
	JobID string 

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
	if cfg.SourceType == "" {
		if cfg.ArchiveURL != "" { // ArchiveURLが指定されている場合はarchiveソースとみなす
			cfg.SourceType = "archive"
		} else {
			cfg.SourceType = "git"
		}
	}
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
	if cfg.RegistryHost == "" {
		cfg.RegistryHost = "harbor.main-harbor"
	}
	if cfg.RegistryProject == "" {
		cfg.RegistryProject = "buildkit"
	}
	return cfg
}
