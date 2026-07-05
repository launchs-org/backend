import { Play } from 'lucide-react'

interface ProjectApplyBarProps {
  pendingDeploymentCount: number // pending中のDeployment件数
  pendingIngressRouteCount: number // pending中のIngressRoute件数
  applying: boolean // 一括Apply実行中フラグ
  progress: { done: number; total: number } | null // 完了待機の進捗（完了件数/対象件数）
  onApply: () => void // Applyボタン押下時のハンドラー
  onShowDetails: () => void // Detailsボタン押下時のハンドラー
}

// ProjectApplyBar は画面上部に浮かぶプロジェクト一括Applyバー
export function ProjectApplyBar({
  pendingDeploymentCount,
  pendingIngressRouteCount,
  applying,
  progress,
  onApply,
  onShowDetails,
}: ProjectApplyBarProps) {
  const totalChanges = pendingDeploymentCount + pendingIngressRouteCount // 保留中の変更件数の合計

  return (
    <div className="fixed top-16 left-1/2 -translate-x-1/2 z-[60] animate-in fade-in slide-in-from-top-4 duration-300">
      <div className="flex items-center gap-3 rounded-2xl border border-[#00C2D1]/30 bg-white/95 backdrop-blur-md px-5 py-3 shadow-2xl shadow-cyan-900/10">
        <span className="text-sm font-medium text-[#00C2D1]">
          {progress ? `適用完了を待機中... (${progress.done}/${progress.total})` : `${totalChanges}件の変更を適用`}
        </span>
        <button
          onClick={onShowDetails}
          disabled={applying}
          className="text-sm text-gray-600 border border-gray-200 rounded-lg px-3 py-1.5 hover:bg-gray-50 transition-colors disabled:opacity-50"
        >
          詳細
        </button>
        <button
          onClick={onApply}
          disabled={applying}
          className="flex items-center gap-2 text-sm font-medium text-white bg-[#00C2D1] hover:bg-[#00aebd] rounded-lg px-4 py-1.5 transition-colors disabled:opacity-50"
        >
          <Play className="w-3.5 h-3.5" />
          {applying ? '適用中...' : 'Apply'}
        </button>
      </div>
    </div>
  )
}
