import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Globe } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { IngressRoute, PathRule } from '@/lib/types'

export type IngressNodeData = {
  ingress: IngressRoute
  pathRules: PathRule[]
  onSelect?: () => void // ノードクリック時のコールバック
}

export function IngressNode({ data }: NodeProps) {
  const { ingress, pathRules, onSelect } = data as IngressNodeData

  return (
    <div
      onClick={onSelect} // クリックでサイドバーを開く
      className="bg-white border border-gray-200 rounded-lg shadow-sm w-56 cursor-pointer hover:border-purple-400 hover:shadow-md transition-all"
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />

      <div className="p-3">
        {/* ヘッダー */}
        <div className="flex items-center gap-2 mb-2">
          <span className="p-1 rounded bg-purple-50 text-purple-500">
            <Globe className="w-3.5 h-3.5" />
          </span>
          <span className="text-xs text-gray-400 font-medium">IngressRoute</span>
          <StatusBadge status={ingress.status} />
        </div>

        {/* ホスト名 */}
        <p className="text-xs font-mono text-[#111827] break-all mb-2 leading-relaxed">
          {ingress.host}
        </p>

        {/* パスルール一覧 */}
        {pathRules.length > 0 && (
          <div className="border-t border-gray-100 pt-2 mt-1 space-y-1">
            {pathRules.map(pathRule => (
              <div key={pathRule.id} className="flex items-center justify-between text-xs">
                <span className="font-mono text-gray-600">{pathRule.path_prefix}</span>
                <span className={`text-[10px] ${pathRule.status === 'active' ? 'text-green-500' : pathRule.status === 'deleting' ? 'text-red-400' : 'text-amber-500'}`}>
                  {pathRule.status}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
