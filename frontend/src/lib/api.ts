export const API_BASE = '/app/api/v1'

const REFRESH_PATH = '/auth/token'
const CACHE_DURATION = 5 * 60 * 1000 // 5分

// ── トークン管理 ──────────────────────────────────────────────

const getRefreshToken = (): string | null => localStorage.getItem('token') // リフレッシュトークンを取得する

const getAccessToken = (): string | null => sessionStorage.getItem('access_token') // アクセストークンを取得する

const setAccessToken = (token: string): void => {
  sessionStorage.setItem('access_token', token) // アクセストークンを保存する
  sessionStorage.setItem('access_token_timestamp', String(Date.now())) // タイムスタンプを保存する
}

const clearTokens = (): void => {
  localStorage.removeItem('token') // リフレッシュトークンを削除する
  sessionStorage.removeItem('access_token') // アクセストークンを削除する
  sessionStorage.removeItem('access_token_timestamp') // タイムスタンプを削除する
}

const isAccessTokenFresh = (): boolean => {
  const ts = parseInt(sessionStorage.getItem('access_token_timestamp') ?? '0', 10) // タイムスタンプを取得する
  return !!getAccessToken() && Date.now() - ts < CACHE_DURATION // キャッシュ有効期限内かどうかを確認する
}

let refreshPromise: Promise<void> | null = null // リフレッシュ中の重複呼び出しを防止する

/** リフレッシュトークンでアクセストークンを取得・キャッシュする */
async function refreshAccessToken(): Promise<void> {
  const refreshToken = getRefreshToken() // リフレッシュトークンを取得する
  if (!refreshToken) {
    throw new Error('no_refresh_token') // リフレッシュトークンがない場合はエラー
  }

  const res = await fetch(REFRESH_PATH, {
    method: 'GET',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Authorization: refreshToken, // リフレッシュトークンをヘッダーに設定する
    },
  })

  if (!res.ok) {
    throw new Error(`refresh_failed:${res.status}`) // リフレッシュ失敗時はエラーをスローする
  }

  const data = await res.json() // レスポンスを解析する
  if (data?.token) {
    setAccessToken(data.token) // アクセストークンを保存する
  } else {
    throw new Error('refresh_no_token') // トークンがない場合はエラーをスローする
  }
}

/** アクセストークンを取得（必要に応じてリフレッシュ） */
async function ensureAccessToken(): Promise<string | null> {
  if (isAccessTokenFresh()) {
    return getAccessToken() // キャッシュが有効な場合はそのまま返す
  }

  if (!refreshPromise) {
    refreshPromise = refreshAccessToken().finally(() => {
      refreshPromise = null // リフレッシュ完了後にプロミスをクリアする
    })
  }
  try {
    await refreshPromise // リフレッシュを待機する
  } catch {
    // リフレッシュ失敗 → 401ハンドラが処理する
  }

  return getAccessToken() // アクセストークンを返す
}

// ── 認証状態チェック（起動時用） ──────────────────────────────

/** リフレッシュトークンが存在してアクセストークンが取得できるか確認する */
export async function checkAuth(): Promise<boolean> {
  if (!getRefreshToken()) return false // リフレッシュトークンがない場合は未認証
  try {
    await refreshAccessToken() // アクセストークンを取得する
    return true // 認証成功
  } catch {
    return false // 認証失敗
  }
}

/** ログアウト：トークンをクリアして /auth/login へリダイレクトする */
export async function logout(): Promise<void> {
  const refreshToken = getRefreshToken() // リフレッシュトークンを取得する
  if (refreshToken) {
    fetch('/auth/logout', {
      method: 'POST',
      credentials: 'include',
      headers: { Authorization: refreshToken }, // ベストエフォートでサーバー側ログアウトする
    }).catch(() => {})
  }
  clearTokens() // トークンをクリアする
  window.location.href = '/auth/login' // ログインページへリダイレクトする
}

// ── エラークラス ──────────────────────────────────────────────

export class ApiError extends Error {
  code: string
  status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.code = code
    this.status = status
    this.name = 'ApiError'
  }
}

export class QuotaExceededApiError extends Error {
  resource: string
  current: number
  limit: number

  constructor(resource: string, current: number, limit: number) {
    const resourceLabel: Record<string, string> = {
      projects: 'プロジェクト',
      deployments: 'デプロイメント',
      replicas: 'レプリカ数',
      volumes: 'ボリューム数',
      volume_size_mb: 'ボリュームサイズ',
      total_volume_mb: 'ボリューム総容量',
    }
    const label = resource.startsWith('instance:')
      ? `インスタンス（${resource.replace('instance:', '')}）`
      : (resourceLabel[resource] ?? resource)
    super(`${label}の上限（${limit}）に達しています（現在: ${current}）`)
    this.resource = resource
    this.current = current
    this.limit = limit
    this.name = 'QuotaExceededApiError'
  }
}

// ── fetch ラッパー ────────────────────────────────────────────

async function buildHeaders(): Promise<Record<string, string>> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json', // JSONコンテンツタイプを設定する
  }
  const token = await ensureAccessToken() // アクセストークンを取得する
  if (token) {
    headers['Authorization'] = token // Authorizationヘッダーを設定する
  }
  return headers
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    clearTokens() // トークンをクリアする
    window.location.href = '/auth/login' // ログインページへリダイレクトする
    throw new ApiError('UNAUTHORIZED', 'Unauthorized', 401)
  }

  if (res.status === 204) {
    return undefined as unknown as T // No Content の場合はundefinedを返す
  }

  const json = await res.json() // レスポンスをJSONとして解析する

  if (!res.ok) {
    if (res.status === 403 && json?.error === 'quota_exceeded') {
      throw new QuotaExceededApiError(json.resource, json.current, json.limit) // quota 超過エラーを専用クラスで throw する
    }
    const errField = json?.error
    // error フィールドが文字列の場合はそのままメッセージとして使う
    const errMessage = typeof errField === 'string' ? errField : (errField?.message ?? `HTTP ${res.status}`)
    const errCode = typeof errField === 'object' ? (errField?.code ?? 'UNKNOWN') : 'UNKNOWN'
    throw new ApiError(errCode, errMessage, res.status)
  }

  return json as T // レスポンスデータを返す
}

export async function get<T>(path: string, params?: Record<string, string>): Promise<T> {
  const url = new URL(`${API_BASE}${path}`, window.location.origin) // URLを構築する
  if (params) {
    Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v)) // クエリパラメータを設定する
  }
  const res = await fetch(url.toString(), {
    method: 'GET',
    credentials: 'include',
    headers: await buildHeaders(), // ヘッダーを構築する
  })
  return handleResponse<T>(res) // レスポンスを処理する
}

export async function post<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: await buildHeaders(), // ヘッダーを構築する
    body: body !== undefined ? JSON.stringify(body) : undefined, // ボディをJSON文字列に変換する
  })
  return handleResponse<T>(res) // レスポンスを処理する
}

export async function put<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'PUT',
    credentials: 'include',
    headers: await buildHeaders(), // ヘッダーを構築する
    body: body !== undefined ? JSON.stringify(body) : undefined, // ボディをJSON文字列に変換する
  })
  return handleResponse<T>(res) // レスポンスを処理する
}

export async function patch<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'PATCH',
    credentials: 'include',
    headers: await buildHeaders(), // ヘッダーを構築する
    body: body !== undefined ? JSON.stringify(body) : undefined, // ボディをJSON文字列に変換する
  })
  return handleResponse<T>(res) // レスポンスを処理する
}

export async function postMultipart<T>(path: string, formData: FormData): Promise<T> {
  const token = await ensureAccessToken() // アクセストークンを取得する（Content-Typeはブラウザにboundary付きで自動設定させるため手動指定しない）
  const headers: Record<string, string> = {}
  if (token) {
    headers['Authorization'] = token // Authorizationヘッダーを設定する
  }
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers,
    body: formData,
  })
  return handleResponse<T>(res) // レスポンスを処理する
}

export async function del<T = void>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'DELETE',
    credentials: 'include',
    headers: await buildHeaders(), // ヘッダーを構築する
    body: body !== undefined ? JSON.stringify(body) : undefined, // ボディをJSON文字列に変換する
  })
  return handleResponse<T>(res) // レスポンスを処理する
}
