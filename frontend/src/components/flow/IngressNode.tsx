import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Globe, Lock } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { IngressRoute } from '@/lib/types'

export type IngressNodeData = {
  ingress: IngressRoute
}

export function IngressNode({ data }: NodeProps) {
  const { ingress } = data as IngressNodeData

  return (
    <div className="bg-white border border-gray-200 rounded-lg shadow-sm w-52">
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />

      <div className="p-3">
        {/* ヘッダー */}
        <div className="flex items-center gap-2 mb-2">
          <span className="p-1 rounded bg-purple-50 text-purple-500">
            <Globe className="w-3.5 h-3.5" />
          </span>
          <span className="text-xs text-gray-400 font-medium">Ingress</span>
          {ingress.tls_enabled && (
            <Lock className="w-3 h-3 text-green-500 ml-auto" />
          )}
        </div>

        {/* ホスト名 */}
        <p className="text-xs font-mono text-[#111827] break-all mb-2 leading-relaxed">
          {ingress.tls_enabled ? 'https://' : 'http://'}{ingress.host}{ingress.path_prefix}
        </p>

        {/* ステータス */}
        <StatusBadge status={ingress.status} />
      </div>
    </div>
  )
}
