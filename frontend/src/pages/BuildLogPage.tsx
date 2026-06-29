import { useState, useCallback, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, X, GitBranch, GitCommit, Clock, CheckCircle2, XCircle, AlertCircle, Ban } from 'lucide-react'
import { LogViewer } from '@/components/LogViewer'
import { get, del } from '@/lib/api'
import { toast } from 'sonner' // トースト通知をインポートする
import type { Build, BuildLogsResponse, BuildStatus } from '@/lib/types'
import { POLL_INTERVAL_NORMAL, POLL_INTERVAL_FAST } from '@/lib/config'

// ステータスごとの表示メタデータ
const BUILD_STATUS_META: Record<BuildStatus, { label: string; icon: React.ReactNode; badge: string; bannerBg: string }> = {
  pending:   { label: '待機中',     icon: <Clock        className="w-4 h-4" />, badge: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30', bannerBg: '' },
  building:  { label: 'ビルド中',   icon: <AlertCircle  className="w-4 h-4 animate-pulse" />, badge: 'bg-blue-500/20 text-blue-400 border-blue-500/30', bannerBg: '' },
  succeeded: { label: '成功',       icon: <CheckCircle2 className="w-4 h-4" />, badge: 'bg-green-500/20 text-green-400 border-green-500/30', bannerBg: 'bg-green-900/40 border-green-700/50 text-green-300' },
  failed:    { label: '失敗',       icon: <XCircle      className="w-4 h-4" />, badge: 'bg-red-500/20 text-red-400 border-red-500/30', bannerBg: 'bg-red-900/40 border-red-700/50 text-red-300' },
  cancelled: { label: 'キャンセル', icon: <Ban          className="w-4 h-4" />, badge: 'bg-gray-500/20 text-gray-400 border-gray-500/30', bannerBg: 'bg-gray-800/60 border-gray-600/50 text-gray-300' },
}

const TERMINAL_STATUSES: BuildStatus[] = ['succeeded', 'failed', 'cancelled'] // 終了ステータスの定義

export function BuildLogPage() {
  const { buildId } = useParams<{ buildId: string }>()
  const navigate = useNavigate()

  const [buildData, setBuildData] = useState<Build | null>(null) // ビルド情報を管理する
  const [cancelling, setCancelling] = useState(false) // キャンセル中フラグ
  const [isLiveActive, setIsLiveActive] = useState(true) // LogViewer の Live 状態を外部から制御するフラグ

  // ビルド情報をポーリング取得する
  useEffect(() => {
    if (!buildId) return

    const fetchBuild = async () => {
      try {
        const data = await get<Build>(`/builds/${buildId}`) // ビルド情報を取得する
        setBuildData(data)
        if (TERMINAL_STATUSES.includes(data.status)) {
          setIsLiveActive(false) // 終了ステータスになったら Live を停止する
        }
      } catch (fetchError) {
        console.error('ビルド情報取得エラー:', fetchError)
      }
    }

    void fetchBuild() // 初回取得
    const intervalId = setInterval(() => void fetchBuild(), POLL_INTERVAL_NORMAL) // 定期的にポーリングする
    return () => clearInterval(intervalId)
  }, [buildId])

  const fetchBuildLogs = useCallback(async (since?: string) => {
    if (!buildId) return { logs: '', lastTimestamp: null }
    const params: Record<string, string> = {}
    if (since) params.since = since // since パラメータを設定する
    const result = await get<BuildLogsResponse>(`/builds/${buildId}/logs`, params)
    return { logs: result.logs ?? '', lastTimestamp: result.last_timestamp ?? null } // last_timestamp を差分ポーリングに使う
  }, [buildId])

  const handleCancel = async () => {
    if (!buildId) return
    setCancelling(true)
    try {
      await del(`/builds/${buildId}`) // ビルドをキャンセルする
      navigate(-1) // 前のページへ戻る
    } catch (cancelError) {
      console.error(cancelError)
      toast.error(cancelError instanceof Error ? cancelError.message : 'キャンセルに失敗しました') // エラーをトーストで表示する
    } finally {
      setCancelling(false)
    }
  }

  const statusMeta = buildData ? BUILD_STATUS_META[buildData.status] : null // ステータスメタデータを取得する
  const isTerminal = buildData ? TERMINAL_STATUSES.includes(buildData.status) : false // 終了状態かどうかを判定する

  const isEmbedded = window.self !== window.top // iframe 内で表示されているかどうかを判定する

  return (
    <div className={`${isEmbedded ? 'h-full' : 'h-screen'} flex flex-col bg-[#0D1117] text-[#E6EDF3] overflow-hidden`}>
      {/* ヘッダー */}
      <header className="border-b border-[#30363D] px-4 py-3 flex items-center gap-3 shrink-0" style={{ background: '#161B22' }}>
        <button
          onClick={() => navigate(-1)}
          className="p-1.5 rounded hover:bg-[#21262D] text-[#8B949E] hover:text-[#E6EDF3] transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>

        {/* ビルドID + ステータス */}
        <div className="flex items-center gap-2 flex-1 min-w-0">
          <span className="font-mono text-sm shrink-0">Build #{buildId?.slice(0, 8)}</span>
          {statusMeta && (
            <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border ${statusMeta.badge}`}>
              {statusMeta.icon}
              {statusMeta.label}
            </span>
          )}
          {/* ブランチ */}
          {buildData?.branch && (
            <span className="flex items-center gap-1 text-xs text-[#8B949E] shrink-0">
              <GitBranch className="w-3.5 h-3.5" />
              {buildData.branch}
            </span>
          )}
          {/* コミットSHA */}
          {buildData?.commit_sha && (
            <span className="flex items-center gap-1 text-xs text-[#8B949E] truncate min-w-0">
              <GitCommit className="w-3.5 h-3.5 shrink-0" />
              <span className="font-mono">{buildData.commit_sha.slice(0, 7)}</span>
              {buildData.commit_message && (
                <span className="truncate text-[#6E7681]">{buildData.commit_message}</span>
              )}
            </span>
          )}
        </div>

        {/* キャンセルボタン（終了ステータスでなければ表示）*/}
        {!isTerminal && (
          <button
            onClick={() => void handleCancel()}
            disabled={cancelling}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-md border border-red-800 text-red-400 hover:bg-red-900/30 transition-colors disabled:opacity-50 shrink-0"
          >
            <X className="w-3.5 h-3.5" />
            {cancelling ? 'キャンセル中...' : 'Cancel Build'}
          </button>
        )}
      </header>

      {/* 完了/失敗バナー */}
      {isTerminal && statusMeta?.bannerBg && (
        <div className={`flex items-center gap-2 px-4 py-2 border-b text-sm font-medium shrink-0 ${statusMeta.bannerBg}`}>
          {statusMeta.icon}
          <span>
            ビルドが{statusMeta.label}しました
            {buildData?.finished_at && (
              <span className="font-normal text-xs ml-2 opacity-70">
                {new Date(buildData.finished_at).toLocaleString('ja-JP')}
              </span>
            )}
          </span>
        </div>
      )}

      {/* ビルドログ */}
      <div className="flex-1 p-4 overflow-hidden">
        <LogViewer
          fetchLogs={fetchBuildLogs}
          title={`Build Log — #${buildId?.slice(0, 8)}`}
          pollInterval={POLL_INTERVAL_FAST}
          initialLive={true}
          autoStopLive={false}
          externalLive={isLiveActive}
          onLiveChange={setIsLiveActive}
        />
      </div>
    </div>
  )
}
