import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Plus, FolderOpen, Clock } from 'lucide-react'
import { Layout } from '@/components/Layout'
import { StatusBadge } from '@/components/StatusBadge'
import { get } from '@/lib/api'
import type { Project, Quota } from '@/lib/types'

export function DashboardPage() {
  const navigate = useNavigate()
  const [projectList, setProjectList] = useState<Project[]>([]) // プロジェクト一覧を管理する
  const [quota, setQuota] = useState<Quota | null>(null) // クォータ情報を管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する
  const [error, setError] = useState<string | null>(null) // エラー状態を管理する

  const fetchData = async () => {
    try {
      const [projectsData, quotaData] = await Promise.all([
        get<Project[]>('/projects'), // プロジェクト一覧を取得する
        get<Quota>('/users/quota'), // クォータ情報を取得する
      ])
      setProjectList(projectsData ?? [])
      setQuota(quotaData)
    } catch (fetchError) {
      setError('データの取得に失敗しました')
      console.error(fetchError)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void fetchData() // 初回データ取得

    const intervalId = setInterval(() => {
      void fetchData() // 30秒ごとにポーリングする
    }, 30_000)

    return () => clearInterval(intervalId) // クリーンアップ
  }, [])

  const formatRelativeTime = (dateStr: string) => {
    if (!dateStr) return '—' // 日付が未設定の場合はダッシュを返す
    const timestamp = new Date(dateStr).getTime() // 日付をタイムスタンプに変換する
    if (isNaN(timestamp)) return '—' // 無効な日付の場合はダッシュを返す
    const diff = Date.now() - timestamp // 経過時間を計算する
    const minutes = Math.floor(diff / 60_000)
    if (minutes < 60) return `${minutes}分前`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}時間前`
    return `${Math.floor(hours / 24)}日前`
  }

  return (
    <Layout
      actions={
        <button
          onClick={() => navigate('/projects/new')}
          className="flex items-center gap-1.5 bg-[#111827] text-white text-sm px-3 py-1.5 rounded-md hover:bg-gray-800 transition-colors"
        >
          <Plus className="w-3.5 h-3.5" />
          New Project
        </button>
      }
    >
      <div className="space-y-6">
        {/* ページタイトル */}
        <div>
          <h1 className="text-xl font-semibold text-[#111827]">Projects</h1>
          {quota && (
            <p className="text-sm text-gray-500 mt-1">
              {quota.current_projects} / {quota.max_projects} projects used
            </p>
          )}
        </div>

        {/* クォータバー */}
        {quota && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <h2 className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-3">リソース使用状況</h2>
            <div className="grid grid-cols-3 gap-4">
              <QuotaBar label="プロジェクト" current={quota.current_projects} max={quota.max_projects} />
              <QuotaBar label="デプロイメント" current={quota.current_deployments} max={quota.max_deployments} />
              <QuotaBar label="ボリューム" current={quota.current_volume_mb} max={quota.max_volume_mb} />
            </div>
          </div>
        )}

        {/* エラー表示 */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-700">
            {error} — <button onClick={() => void fetchData()} className="underline">再試行</button>
          </div>
        )}

        {/* ローディング */}
        {loading && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {[...Array(3)].map((_, skeletonIndex) => (
              <div key={skeletonIndex} className="bg-white rounded-lg border border-gray-200 p-4 animate-pulse">
                <div className="h-4 bg-gray-200 rounded w-2/3 mb-2" />
                <div className="h-3 bg-gray-100 rounded w-1/3" />
              </div>
            ))}
          </div>
        )}

        {/* プロジェクト一覧 */}
        {!loading && projectList.length === 0 && !error && (
          <div className="text-center py-16 bg-white rounded-lg border border-dashed border-gray-200">
            <FolderOpen className="w-10 h-10 text-gray-300 mx-auto mb-3" />
            <p className="text-sm font-medium text-gray-500">まだプロジェクトがありません</p>
            <p className="text-xs text-gray-400 mt-1 mb-4">最初のプロジェクトを作成してデプロイを始めましょう</p>
            <button
              onClick={() => navigate('/projects/new')}
              className="inline-flex items-center gap-1.5 bg-[#111827] text-white text-sm px-4 py-2 rounded-md hover:bg-gray-800 transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              New Project
            </button>
          </div>
        )}

        {!loading && projectList.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {projectList.map((project, projectIndex) => (
              <Link
                key={project.id || String(projectIndex)}
                to={`/projects/${project.id}`}
                className="group bg-white rounded-lg border border-gray-200 p-4 hover:border-[#00C2D1] hover:shadow-sm transition-all"
              >
                <div className="flex items-start justify-between mb-3">
                  <h2 className="font-medium text-[#111827] group-hover:text-[#00C2D1] transition-colors truncate">
                    {project.name}
                  </h2>
                  <StatusBadge status={project.status} />
                </div>
                <div className="flex items-center gap-1 text-xs text-gray-400">
                  <Clock className="w-3 h-3" />
                  {formatRelativeTime(project.updated_at)}
                </div>
                <div className="mt-2 text-xs text-gray-400 font-mono truncate">
                  {project.namespace}
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </Layout>
  )
}

function QuotaBar({ label, current, max }: { label: string; current: number; max: number }) {
  const pct = max > 0 ? Math.min((current / max) * 100, 100) : 0 // 使用率を計算する
  const isWarning = pct >= 80 // 80%以上で警告色にする

  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span className="text-gray-500">{label}</span>
        <span className={isWarning ? 'text-amber-600 font-medium' : 'text-gray-400'}>{current}/{max}</span>
      </div>
      <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${isWarning ? 'bg-amber-400' : 'bg-[#00C2D1]'}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}
