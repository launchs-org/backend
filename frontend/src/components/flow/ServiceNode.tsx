import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Network } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { K8sService } from '@/lib/types'

export type ServiceNodeData = {
  service: K8sService
}

export function ServiceNode({ data }: NodeProps) {
  const { service } = data as ServiceNodeData

  return (
    <div className="bg-white border border-gray-200 rounded-lg shadow-sm w-44">
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />

      <div className="p-3">
        {/* ヘッダー */}
        <div className="flex items-center gap-2 mb-2">
          <span className="p-1 rounded bg-blue-50 text-blue-500">
            <Network className="w-3.5 h-3.5" />
          </span>
          <span className="text-xs text-gray-400 font-medium">Service</span>
        </div>

        {/* ポート情報（port=0 は pending 値を表示する）*/}
        <p className="text-sm font-mono text-[#111827] mb-2">
          :{service.port !== 0 ? service.port : service.pending_port} → :{service.target_port !== 0 ? service.target_port : service.pending_target_port}
        </p>

        {/* ステータスとタイプ */}
        <div className="flex items-center gap-2 flex-wrap">
          <StatusBadge status={service.status} />
          <span className="text-xs text-gray-400">{service.type}</span>
        </div>
      </div>
    </div>
  )
}
