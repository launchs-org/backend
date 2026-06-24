import { Handle, Position, type NodeProps } from '@xyflow/react'
import { KeyRound } from 'lucide-react'
import type { EnvVar } from '@/lib/types'

export type EnvVarNodeData = {
  envVar: EnvVar
  highlighted: boolean // マウント先からハイライトされているか
  onClick: (envVarId: string) => void
}

export function EnvVarNode({ data }: NodeProps) {
  const { envVar, highlighted, onClick } = data as EnvVarNodeData

  return (
    <div
      onClick={() => onClick(envVar.id)}
      className="bg-white rounded-xl shadow-md overflow-hidden cursor-pointer transition-all"
      style={{
        borderTopColor: highlighted ? '#8B5CF6' : '#A78BFA',
        borderTopWidth: 3,
        border: highlighted ? '2px solid #8B5CF6' : '1px solid #F3F4F6',
        width: 200,
        height: 80,
        boxShadow: highlighted ? '0 0 0 3px rgba(139,92,246,0.25)' : undefined,
      }}
    >
      <Handle type="source" position={Position.Right} style={{ opacity: 0, top: '50%' }} />

      {/* ヘッダー */}
      <div className="px-3 pt-2.5 pb-1.5 flex items-center gap-2">
        <span className="p-1 rounded-lg bg-violet-50 text-violet-500 shrink-0">
          <KeyRound className="w-3 h-3" />
        </span>
        <div>
          <p className="text-[10px] font-semibold text-violet-500 uppercase tracking-wide leading-none mb-0.5">EnvVar</p>
          {envVar.is_secret && (
            <span className="text-[9px] bg-purple-50 text-purple-400 px-1 py-0.5 rounded">secret</span>
          )}
        </div>
      </div>

      {/* キー名 */}
      <div className="px-3 pb-2">
        <p className="text-xs font-bold text-gray-900 font-mono truncate">{envVar.key}</p>
      </div>
    </div>
  )
}
