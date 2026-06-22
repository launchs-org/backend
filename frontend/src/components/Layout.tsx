import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Cloud, Settings, LogOut, ChevronRight } from 'lucide-react'
import { logout, get } from '@/lib/api'
import type { Quota } from '@/lib/types'

type BreadcrumbItem = { label: string; href?: string; sub?: string }

type LayoutProps = {
  children: React.ReactNode
  breadcrumbs?: BreadcrumbItem[]
  actions?: React.ReactNode
  fullWidth?: boolean
}

const INSTANCE_SIZE_ORDER = ['small', 'medium', 'large'] // 表示順を固定する

export function Layout({ children, breadcrumbs, actions, fullWidth }: LayoutProps) {
  const navigate = useNavigate() // ナビゲーションフックを取得する
  const isEmbedded = window.self !== window.top // iframeで表示されているかどうかを判定する
  const [quota, setQuota] = useState<Quota | null>(null) // フッター表示用 quota

  useEffect(() => {
    if (isEmbedded) return // iframe 内ではフッターを表示しないので取得不要
    get<Quota>('/users/quota').then(setQuota).catch(() => {}) // quota を取得する（失敗は無視）
  }, [isEmbedded])

  const handleLogout = async () => {
    await logout() // ログアウトしてリダイレクトする
  }

  // instance_limits に含まれるサイズを決まった順序で並べる
  const instanceSizeList = quota
    ? [
        ...INSTANCE_SIZE_ORDER.filter((size) => size in quota.instance_limits),
        ...Object.keys(quota.instance_limits).filter((size) => !INSTANCE_SIZE_ORDER.includes(size)),
      ]
    : []

  return (
    <div className="bg-[#F0F2F5]">
      {/* ヘッダー */}
      <header className="h-12 bg-white border-b border-gray-200 flex items-center px-4 gap-4 sticky top-0 z-50">
        {!isEmbedded && (
          <Link to="/" className="flex items-center gap-1.5 text-[#111827] font-semibold text-sm">
            <Cloud className="w-4 h-4 text-[#00C2D1]" />
            launchs
          </Link>
        )}

        {/* パンくずリスト（iframe内では非表示）*/}
        {!isEmbedded && breadcrumbs && breadcrumbs.length > 0 && (
          <nav className="flex items-center gap-1 text-sm text-gray-500">
            {breadcrumbs.map((crumb, crumbIndex) => (
              <span key={crumbIndex} className="flex items-center gap-1">
                <ChevronRight className="w-3 h-3" />
                {crumb.href ? (
                  <Link to={crumb.href} className="hover:text-[#111827] transition-colors">
                    {crumb.label}
                  </Link>
                ) : (
                  <span className="flex items-center gap-2">
                    <span className="text-[#111827] font-medium">{crumb.label}</span>
                    {crumb.sub && <span className="text-gray-400 font-mono text-xs">{crumb.sub}</span>}
                  </span>
                )}
              </span>
            ))}
          </nav>
        )}

        {/* iframe内ではデプロイメント名のみ表示 */}
        {isEmbedded && breadcrumbs && breadcrumbs.length > 0 && (
          <span className="text-sm font-medium text-[#111827]">
            {breadcrumbs[breadcrumbs.length - 1].label}
          </span>
        )}

        <div className="ml-auto flex items-center gap-2">
          {actions}
          {!isEmbedded && (
            <>
              <button
                onClick={() => navigate('/settings')}
                className="p-1.5 rounded hover:bg-gray-100 text-gray-500 hover:text-gray-700 transition-colors"
                aria-label="設定"
              >
                <Settings className="w-4 h-4" />
              </button>
              <button
                onClick={handleLogout}
                className="p-1.5 rounded hover:bg-gray-100 text-gray-500 hover:text-gray-700 transition-colors"
                aria-label="ログアウト"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </>
          )}
        </div>
      </header>

      {/* メインコンテンツ */}
      <main className={`min-h-screen ${fullWidth ? '' : 'max-w-7xl mx-auto px-4 py-6'}`}>
        {children}
      </main>

      {/* フッター：インスタンスサイズ別使用状況 */}
      {!isEmbedded && quota && instanceSizeList.length > 0 && (
        <footer className="h-8 bg-white border-t border-gray-200 flex items-center px-4 gap-4 sticky bottom-0 z-40">
          <span className="text-xs text-gray-400 shrink-0">instances</span>
          <div className="flex items-center gap-3">
            {instanceSizeList.map((size) => {
              const current = quota.current_instances[size] ?? 0 // 現在数を取得する
              const limit = quota.instance_limits[size] // 上限数を取得する
              const isWarning = limit > 0 && current / limit >= 0.8 // 80% 以上で警告色にする
              return (
                <span
                  key={size}
                  className={`text-xs font-mono ${isWarning ? 'text-amber-600 font-medium' : 'text-gray-500'}`}
                >
                  {size}: {current}/{limit}
                </span>
              )
            })}
          </div>
          <div className="w-px h-3 bg-gray-200 mx-1 shrink-0" />
          <span className="text-xs text-gray-400 shrink-0">volumes</span>
          <div className="flex items-center gap-3">
            {(() => {
              const volWarning = quota.max_volumes > 0 && quota.current_volumes / quota.max_volumes >= 0.8 // ボリューム数の警告判定
              const totalWarning = quota.max_total_volume_mb > 0 && quota.current_total_volume_mb / quota.max_total_volume_mb >= 0.8 // 総容量の警告判定
              return (
                <>
                  <span className={`text-xs font-mono ${volWarning ? 'text-amber-600 font-medium' : 'text-gray-500'}`}>
                    count: {quota.current_volumes}/{quota.max_volumes}
                  </span>
                  <span className={`text-xs font-mono ${totalWarning ? 'text-amber-600 font-medium' : 'text-gray-500'}`}>
                    total: {quota.current_total_volume_mb}/{quota.max_total_volume_mb}MB
                  </span>
                  <span className="text-xs font-mono text-gray-500">
                    max: {quota.max_volume_size_mb}MB/vol
                  </span>
                </>
              )
            })()}
          </div>
        </footer>
      )}
    </div>
  )
}
