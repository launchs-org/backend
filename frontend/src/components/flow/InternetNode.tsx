import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Globe2 } from 'lucide-react'

export function InternetNode(_props: NodeProps) {
  return (
    <div
      className="flex flex-col items-center justify-center gap-1.5 w-20 h-20 rounded-full border-2 border-dashed border-sky-300 bg-sky-50 shadow-sm"
    >
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} /> {/* 右側から Ingress へ接続する */}

      <Globe2 className="w-7 h-7 text-sky-400" /> {/* インターネットを表すアイコン */}
      <span className="text-[10px] font-semibold text-sky-500 uppercase tracking-wide">Internet</span>
    </div>
  )
}
