import { Link, useNavigate } from 'react-router-dom'
import { Cloud, Settings, LogOut, ChevronRight } from 'lucide-react'
import { logout } from '@/lib/api'

type BreadcrumbItem = { label: string; href?: string; sub?: string }

type LayoutProps = {
  children: React.ReactNode
  breadcrumbs?: BreadcrumbItem[]
  actions?: React.ReactNode
  fullWidth?: boolean
}

export function Layout({ children, breadcrumbs, actions, fullWidth }: LayoutProps) {
  const navigate = useNavigate() // ナビゲーションフックを取得する
  const isEmbedded = window.self !== window.top // iframeで表示されているかどうかを判定する

  const handleLogout = async () => {
    await logout() // ログアウトしてリダイレクトする
  }

  return (
    <div className="min-h-screen bg-[#F0F2F5]">
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
      <main className={fullWidth ? '' : 'max-w-7xl mx-auto px-4 py-6'}>
        {children}
      </main>
    </div>
  )
}
