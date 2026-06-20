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

export function DeploymentNode({ data }: NodeProps) {
  const { deployment, projectId: _projectId, onSelect } = data as DeploymentNodeData
  const Icon = TYPE_ICON[deployment.type] ?? Container // タイプに対応するアイコンを取得する

  const hasPending = !!(
    deployment.pending_image_url ||
    deployment.pending_github_repo_url ||
    deployment.pending_replicas ||
    deployment.pending_instance_size
  ) // 保留中の変更があるかどうかを確認する

  // k8s_status から pod の ready 数を取得する
  const k8sStatus = deployment.k8s_status as Record<string, unknown> | null
  const readyReplicas = k8sStatus ? (k8sStatus.readyReplicas as number | undefined) ?? 0 : null

  return (
    <div
      className="bg-white border rounded-lg shadow-sm cursor-pointer hover:shadow-md transition-shadow"
      style={{ borderColor: hasPending ? '#D97706' : '#E5E7EB', width: '220px' }}
      onClick={() => onSelect?.(deployment.id)} // サイドバーにデプロイメント詳細を表示する
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />

      <div className="p-3">
        {/* ヘッダー */}
        <div className="flex items-center gap-2 mb-2">
          <span className="p-1 rounded bg-gray-50 text-gray-500">
            <Icon className="w-3.5 h-3.5" />
          </span>
          <span className="text-xs text-gray-400 font-medium">{TYPE_LABEL[deployment.type]}</span>
          {hasPending && (
            <span className="ml-auto text-xs text-amber-600 font-medium">保留中</span>
          )}
        </div>

        {/* デプロイメント名 */}
        <p className="font-semibold text-[#111827] text-sm truncate mb-2">{deployment.name}</p>

        {/* ステータス */}
        <div className="flex items-center gap-2 flex-wrap mb-2">
          <StatusBadge status={deployment.status} />
          {deployment.app_status !== deployment.status && (
            <StatusBadge status={deployment.app_status} />
          )}
        </div>

        {/* 詳細情報 */}
        <div className="border-t border-gray-100 pt-2 mt-1 space-y-1">
          <div className="flex items-center justify-between text-xs text-gray-400">
            <span>レプリカ</span>
            <span className="font-mono">
              {readyReplicas !== null ? `${readyReplicas}/` : ''}{deployment.replicas}
            </span>
          </div>
          <div className="flex items-center justify-between text-xs text-gray-400">
            <span>サイズ</span>
            <span className="font-mono">{INSTANCE_SIZE_LABEL[deployment.instance_size] ?? (deployment.instance_size || '—')}</span>
          </div>
          {k8sStatus && (
            <div className="flex items-center justify-between text-xs text-gray-400">
              <span>k8s</span>
              <span className={`font-mono ${readyReplicas === deployment.replicas ? 'text-green-500' : 'text-amber-500'}`}>
                {readyReplicas === deployment.replicas ? 'Ready' : 'Not Ready'}
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
