type StatusBadgeProps = {
  status: string
  size?: 'sm' | 'md'
}

const STATUS_CONFIG: Record<string, { label: string; color: string; dot: string }> = {
  // Deployment status
  running: { label: '稼働中', color: 'bg-green-50 text-green-700 border-green-200', dot: 'bg-green-500' },
  active: { label: 'アクティブ', color: 'bg-green-50 text-green-700 border-green-200', dot: 'bg-green-500' },
  succeeded: { label: '成功', color: 'bg-green-50 text-green-700 border-green-200', dot: 'bg-green-500' },
  bound: { label: 'バインド済み', color: 'bg-green-50 text-green-700 border-green-200', dot: 'bg-green-500' },
  pending: { label: '保留中', color: 'bg-amber-50 text-amber-700 border-amber-200', dot: 'bg-amber-500' },
  building: { label: 'ビルド中', color: 'bg-amber-50 text-amber-700 border-amber-200', dot: 'bg-amber-500 animate-pulse' },
  deploying: { label: 'デプロイ中', color: 'bg-amber-50 text-amber-700 border-amber-200', dot: 'bg-amber-500 animate-pulse' },
  provisioning: { label: 'プロビジョニング中', color: 'bg-blue-50 text-blue-700 border-blue-200', dot: 'bg-blue-500 animate-pulse' },
  failed: { label: '失敗', color: 'bg-red-50 text-red-700 border-red-200', dot: 'bg-red-500' },
  error: { label: 'エラー', color: 'bg-red-50 text-red-700 border-red-200', dot: 'bg-red-500' },
  deleting: { label: '削除中', color: 'bg-gray-100 text-gray-500 border-gray-200', dot: 'bg-gray-400' },
  cancelled: { label: 'キャンセル済み', color: 'bg-gray-100 text-gray-500 border-gray-200', dot: 'bg-gray-400' },
}

export function StatusBadge({ status, size = 'sm' }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status] ?? { label: status, color: 'bg-gray-100 text-gray-500 border-gray-200', dot: 'bg-gray-400' }

  const sizeClass = size === 'sm'
    ? 'text-xs px-2 py-0.5 gap-1.5'
    : 'text-sm px-2.5 py-1 gap-2'

  const dotSize = size === 'sm' ? 'w-1.5 h-1.5' : 'w-2 h-2'

  return (
    <span className={`inline-flex items-center rounded-full border font-medium ${sizeClass} ${config.color}`}>
      <span className={`rounded-full shrink-0 ${dotSize} ${config.dot}`} />
      {config.label}
    </span>
  )
}
