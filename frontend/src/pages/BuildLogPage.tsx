import { useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, X } from 'lucide-react'
import { LogViewer } from '@/components/LogViewer'
import { get, del } from '@/lib/api'
import type { BuildLogsResponse } from '@/lib/types'

export function BuildLogPage() {
  const { buildId } = useParams<{ buildId: string }>()
  const navigate = useNavigate()

  const [cancelling, setCancelling] = useState(false) // キャンセル中フラグ

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
      alert('キャンセルに失敗しました')
    } finally {
      setCancelling(false)
    }
  }

  return (
    <div className="h-screen flex flex-col bg-[#0D1117] text-[#E6EDF3] overflow-hidden">
      {/* ヘッダー */}
      <header className="border-b border-[#30363D] px-4 py-3 flex items-center gap-3 shrink-0" style={{ background: '#161B22' }}>
        <button
          onClick={() => navigate(-1)}
          className="p-1.5 rounded hover:bg-[#21262D] text-[#8B949E] hover:text-[#E6EDF3] transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>

        <div className="flex-1">
          <span className="font-mono text-sm">Build #{buildId?.slice(0, 8)}</span>
        </div>

        {/* キャンセルボタン */}
        <button
          onClick={() => void handleCancel()}
          disabled={cancelling}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-md border border-red-800 text-red-400 hover:bg-red-900/30 transition-colors disabled:opacity-50"
        >
          <X className="w-3.5 h-3.5" />
          {cancelling ? 'キャンセル中...' : 'Cancel Build'}
        </button>
      </header>

      {/* ビルドログ */}
      <div className="flex-1 p-4 overflow-hidden">
        <LogViewer
          fetchLogs={fetchBuildLogs}
          title={`Build Log — #${buildId?.slice(0, 8)}`}
          pollInterval={3_000}
          initialLive={true}
          autoStopLive={false}
        />
      </div>
    </div>
  )
}
