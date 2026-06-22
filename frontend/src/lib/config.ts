// ポーリング間隔（ms）
export const POLL_INTERVAL_NORMAL = 5000 // 通常時のポーリング間隔
export const POLL_INTERVAL_FAST = 3000 // 操作後の高速ポーリング間隔
export const POLL_INTERVAL_BUILDS = 10000 // ビルド一覧のポーリング間隔
export const FAST_POLL_DURATION = 30000 // 高速ポーリングを維持する時間（ms）

// サイドバー設定
export const SIDEBAR_INITIAL_WIDTH = 700 // サイドバーの初期幅（px）
export const SIDEBAR_MIN_WIDTH = 700 // サイドバーの最小幅（px）
export const SIDEBAR_MAX_WIDTH = 1200 // サイドバーの最大幅（px）

// フロービュー設定
export const FLOW_ROW_HEIGHT = 200 // ノード行の高さ（px）

// デプロイメント設定
export const REPLICAS_MIN = 0 // レプリカ数の最小値
export const REPLICAS_MAX = 5 // レプリカ数の最大値
export const INSTANCE_SIZES = ['small', 'medium', 'large'] as const // 選択可能なインスタンスサイズ

// GitHub API 設定
export const GITHUB_BRANCHES_PER_PAGE = 100 // ブランチ一覧の取得件数
export const GITHUB_COMMITS_PER_PAGE = 30 // コミット一覧の取得件数
export const GITHUB_COMMIT_MESSAGE_MAX_LENGTH = 60 // コミットメッセージの表示最大文字数

// ログビューアー設定
export const LOG_VIEWER_BG = '#0D1117' // ログビューアーの背景色
export const LOG_VIEWER_FG = '#E6EDF3' // ログビューアーの前景色
export const LOG_VIEWER_TOOLBAR_BG = '#161B22' // ツールバーの背景色
export const LOG_VIEWER_BORDER = '#30363D' // ボーダーカラー
export const LOG_VIEWER_MUTED = '#8B949E' // ミュートテキストカラー
export const LOG_VIEWER_HOVER = '#21262D' // ホバー背景色
export const LOG_VIEWER_HIGHLIGHT_BG = '#F59E0B' // 検索ハイライト色
export const LOG_VIEWER_DEFAULT_POLL_INTERVAL = 5_000 // デフォルトのポーリング間隔（ms）
export const LOG_VIEWER_SCROLL_THRESHOLD = 50 // 最下部判定のピクセル閾値
export const LOG_VIEWER_COPY_RESET_DELAY = 2_000 // コピー後にリセットするまでの時間（ms）
