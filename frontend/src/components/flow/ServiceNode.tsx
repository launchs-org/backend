import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Network } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { K8sService } from '@/lib/types'

export type ServiceNodeData = {
  service: K8sService
}

export function ServiceNode({ data }: NodeProps) {
  const { service } = data as ServiceNodeData

  const displayPort = service.port !== 0 ? service.port : service.pending_port // 表示するポート番号を決定する
  const displayTargetPort = service.target_port !== 0 ? service.target_port : service.pending_target_port // 表示するターゲットポートを決定する

  return (
    <div
      className="bg-white rounded-xl shadow-md w-44 overflow-hidden border border-gray-100"
      style={{ borderTopColor: '#3B82F6', borderTopWidth: 3, height: 98 }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0, top: '50%' }} />
      <Handle type="source" position={Position.Right} style={{ opacity: 0, top: '50%' }} />

      {/* ヘッダー */}
      <div className="px-3 pt-3 pb-2 flex items-center gap-2">
        <span className="p-1.5 rounded-lg bg-blue-50 text-blue-600 shrink-0">
          <Network className="w-3.5 h-3.5" />
        </span>
        <div>
          <p className="text-[10px] font-semibold text-blue-500 uppercase tracking-wide leading-none mb-0.5">Service</p>
          <StatusBadge status={service.status} />
        </div>
      </div>

      {/* ポート情報（port=0 は pending 値を表示する）*/}
      <div className="px-3 pb-3">
        <p className="text-sm font-mono font-semibold text-gray-800">
          :{displayPort} → :{displayTargetPort}
        </p>
        <p className="text-[10px] text-gray-400 mt-0.5">{service.type}</p>
      </div>
    </div>
  )
}
