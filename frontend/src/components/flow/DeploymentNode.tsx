import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Container, GitBranch, Package } from 'lucide-react'
import { StatusBadge } from '@/components/StatusBadge'
import type { Deployment } from '@/lib/types'

export type DeploymentNodeData = {
  deployment: Deployment
  projectId: string
  onSelect?: (deploymentId: string) => void // サイドバー表示用コールバック
}

const TYPE_ICON = {
  image_url: Container,
  dockerfile: GitBranch,
  railpack: Package,
} as const

const TYPE_LABEL = {
  image_url: 'Image',
  dockerfile: 'Dockerfile',
  railpack: 'Railpack',
} as const

const INSTANCE_SIZE_LABEL: Record<string, string> = {
  small: 'Small',
  medium: 'Medium',
  large: 'Large',
}

const TYPE_COLOR = {
  image_url: { accent: '#00C2D1', bg: 'bg-cyan-50', text: 'text-cyan-600' },
  dockerfile: { accent: '#6366F1', bg: 'bg-indigo-50', text: 'text-indigo-600' },
  railpack: { accent: '#10B981', bg: 'bg-emerald-50', text: 'text-emerald-600' },
} as const

export function DeploymentNode({ data }: NodeProps) {
  const { deployment, projectId: _projectId, onSelect } = data as DeploymentNodeData
  const Icon = TYPE_ICON[deployment.type] ?? Container // タイプに対応するアイコンを取得する
  const color = TYPE_COLOR[deployment.type] ?? TYPE_COLOR.image_url // タイプに対応するカラーを取得する

  const hasPending = !!(
    deployment.pending_image_url ||
    deployment.pending_github_repo_url ||
    deployment.pending_replicas ||
    deployment.pending_instance_size
  ) // 保留中の変更があるかどうかを確認する

  // k8s_status から pod の ready 数を取得する
  const k8sStatus = deployment.k8s_status as Record<string, unknown> | null
  const readyReplicas = k8sStatus ? (k8sStatus.readyReplicas as number | undefined) ?? 0 : null
  const isReady = readyReplicas === deployment.replicas // k8s の Ready 状態を判定する

  const stackCount = Math.min(deployment.replicas, 3) - 1 // 背後カードの枚数（最大2枚）
  const OFFSET = 5 // カードごとのずれ量（px）

  const nodeHeight = k8sStatus ? 148 : 132 // k8s行あり:148px、なし:132px（高さを固定してエッジを水平に揃える）

  return (
    <div style={{ width: 220 + stackCount * OFFSET, height: nodeHeight, position: 'relative' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0, top: '50%' }} />
      <Handle type="source" position={Position.Right} style={{ opacity: 0, top: '50%' }} />

      {/* 背景スタックカード（後ろから順に描画する）*/}
      {Array.from({ length: stackCount }).map((_, stackIndex) => {
        const depth = stackCount - stackIndex // 一番後ろが depth 最大
        return (
          <div
            key={stackIndex}
            className="absolute rounded-xl border border-gray-200"
            style={{
              width: 220,
              top: depth * OFFSET,
              left: depth * OFFSET,
              bottom: 0,
              background: `hsl(0,0%,${96 - depth * 2}%)`,
              borderTopColor: color.accent,
              borderTopWidth: 3,
            }}
          />
        )
      })}

      {/* メインカード */}
      <div
        className="bg-white rounded-xl shadow-md cursor-pointer hover:shadow-lg transition-all overflow-hidden border border-gray-100"
        style={{ borderTopColor: hasPending ? '#D97706' : color.accent, borderTopWidth: 3, position: 'relative', width: 220 }}
        onClick={() => onSelect?.(deployment.id)} // サイドバーにデプロイメント詳細を表示する
      >

      {/* ヘッダー */}
      <div className="px-3 pt-3 pb-2 flex items-center gap-2">
        <span className={`p-1.5 rounded-lg shrink-0 ${color.bg} ${color.text}`}>
          <Icon className="w-3.5 h-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <p className={`text-[10px] font-semibold uppercase tracking-wide leading-none mb-0.5 ${color.text}`}>{TYPE_LABEL[deployment.type]}</p>
          <div className="flex items-center gap-1.5 flex-wrap">
            <StatusBadge status={deployment.status} />
            {deployment.app_status !== deployment.status && (
              <StatusBadge status={deployment.app_status} />
            )}
          </div>
        </div>
        {hasPending && (
          <span className="shrink-0 text-[10px] font-semibold text-amber-600 bg-amber-50 px-1.5 py-0.5 rounded-full border border-amber-200">保留中</span>
        )}
      </div>

      {/* デプロイメント名 */}
      <div className="px-3 pb-2">
        <p className="font-bold text-gray-900 text-sm truncate">{deployment.name}</p>
      </div>

      {/* 詳細情報 */}
      <div className="px-3 pb-3 space-y-1 border-t border-gray-50 pt-2">
        <div className="flex items-center justify-between text-xs text-gray-400">
          <span>レプリカ</span>
          <span className="font-mono text-gray-600">
            {readyReplicas !== null ? `${readyReplicas}/` : ''}{deployment.replicas}
          </span>
        </div>
        <div className="flex items-center justify-between text-xs text-gray-400">
          <span>サイズ</span>
          <span className="font-mono text-gray-600">{INSTANCE_SIZE_LABEL[deployment.instance_size] ?? (deployment.instance_size || '—')}</span>
        </div>
        {k8sStatus && (
          <div className="flex items-center justify-between text-xs text-gray-400">
            <span>k8s</span>
            <span className={`font-mono font-semibold ${isReady ? 'text-emerald-500' : 'text-amber-500'}`}>
              {isReady ? 'Ready' : 'Not Ready'}
            </span>
          </div>
        )}
      </div>
      </div>
    </div>
  )
}
