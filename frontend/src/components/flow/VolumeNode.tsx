import { Handle, Position, type NodeProps } from '@xyflow/react'
import { HardDrive } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { Volume } from '@/lib/types'

export type VolumeNodeData = {
  volume: Volume
}

export function VolumeNode({ data }: NodeProps) {
  const { volume } = data as VolumeNodeData

  return (
    <div
      className="bg-white rounded-xl shadow-md overflow-hidden border border-gray-100"
      style={{ borderTopColor: '#F59E0B', borderTopWidth: 3, width: 220, height: 114 }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0, top: '50%' }} /> {/* Deployment からの接続を受け取る */}

      {/* ヘッダー */}
      <div className="px-3 pt-3 pb-2 flex items-center gap-2">
        <span className="p-1.5 rounded-lg bg-amber-50 text-amber-600 shrink-0">
          <HardDrive className="w-3.5 h-3.5" />
        </span>
        <div>
          <p className="text-[10px] font-semibold text-amber-500 uppercase tracking-wide leading-none mb-0.5">Volume</p>
          <StatusBadge status={volume.status} />
        </div>
      </div>

      {/* ボリューム名 */}
      <div className="px-3 pb-2">
        <p className="text-sm font-bold text-gray-900 truncate">{volume.name}</p>
      </div>

      {/* サイズ */}
      <div className="px-3 pb-3">
        <p className="text-xs text-gray-400 font-mono">{volume.size_mb} MB</p>
      </div>
    </div>
  )
}
