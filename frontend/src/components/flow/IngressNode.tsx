import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Globe, Route } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { IngressRoute, PathRule } from '@/lib/types'

export type IngressNodeData = {
  ingress: IngressRoute
  pathRules: PathRule[]
  onSelect?: () => void // ノードクリック時のコールバック
}

export function IngressNode({ data }: NodeProps) {
  const { ingress, pathRules, onSelect } = data as IngressNodeData

  const activePathCount = pathRules.filter(pr => pr.status !== 'deleting').length // 有効なパスルール数を計算する

  return (
    <div
      onClick={onSelect} // クリックでサイドバーを開く
      className="bg-white rounded-xl shadow-md w-56 cursor-pointer hover:shadow-lg transition-all overflow-hidden border border-gray-100"
      style={{ borderTopColor: '#7C3AED', borderTopWidth: 3, height: 116 }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0, top: '50%' }} /> {/* Internet からの接続を受け取る */}
      <Handle type="source" position={Position.Right} style={{ opacity: 0, top: '50%' }} /> {/* Service へ接続する */}

      {/* カラーヘッダー */}
      <div className="px-3 pt-3 pb-2 flex items-center gap-2">
        <span className="p-1.5 rounded-lg bg-purple-100 text-purple-600 shrink-0">
          <Globe className="w-3.5 h-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-[10px] font-semibold text-purple-500 uppercase tracking-wide leading-none mb-0.5">IngressRoute</p>
          <StatusBadge status={ingress.status} />
        </div>
      </div>

      {/* 名前・ホスト名 */}
      <div className="px-3 pb-2">
        <p className="text-xs font-semibold text-gray-800 truncate" title={ingress.name}>
          {ingress.name || '(名前未設定)'}
        </p>
        <p className="text-[10px] font-mono text-gray-400 truncate" title={ingress.host}>
          {ingress.host}
        </p>
      </div>

      {/* パスルール件数バッジ */}
      <div className="px-3 pb-3">
        <div className="flex items-center gap-1.5 text-xs text-gray-400">
          <Route className="w-3 h-3 shrink-0" />
          <span>{activePathCount} {activePathCount === 1 ? 'path' : 'paths'}</span>
        </div>
      </div>
    </div>
  )
}
