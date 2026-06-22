import { useState, useEffect } from 'react'
import { Layout } from '@/components/Layout'
import { get } from '@/lib/api'
import type { Quota } from '@/lib/types'

export function SettingsPage() {
  const [quota, setQuota] = useState<Quota | null>(null) // クォータ情報を管理する
  const [loading, setLoading] = useState(true) // ローディング状態を管理する

  useEffect(() => {
    get<Quota>('/users/quota')
      .then(setQuota)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  return (
    <Layout breadcrumbs={[{ label: 'Settings' }]}>
      <div className="max-w-2xl space-y-6">
        <h1 className="text-xl font-semibold text-[#111827]">設定</h1>

        {/* クォータ情報 */}
        <div className="bg-white rounded-lg border border-gray-200 p-5">
          <h2 className="text-sm font-semibold text-[#111827] mb-4">リソースクォータ</h2>

          {loading ? (
            <div className="space-y-3 animate-pulse">
              {[...Array(4)].map((_, skeletonIndex) => (
                <div key={skeletonIndex} className="h-8 bg-gray-100 rounded" />
              ))}
            </div>
          ) : quota ? (
            <div className="space-y-4">
              <QuotaRow
                label="プロジェクト"
                current={quota.current_projects}
                max={quota.max_projects}
              />
              <QuotaRow
                label="デプロイメント"
                current={quota.current_deployments}
                max={quota.max_deployments}
              />
              <QuotaRow
                label="最大レプリカ数 / デプロイ"
                current={0}
                max={quota.max_replicas_per_deployment}
              />
              <QuotaRow
                label="ボリューム数"
                current={quota.current_volumes}
                max={quota.max_volumes}
              />
              <QuotaRow
                label="ボリューム総容量"
                current={quota.current_total_volume_mb}
                max={quota.max_total_volume_mb}
                unit="MB"
              />
              <QuotaRow
                label="1ボリューム最大サイズ"
                current={0}
                max={quota.max_volume_size_mb}
                unit="MB"
              />
            </div>
          ) : (
            <p className="text-sm text-gray-400">クォータ情報を取得できませんでした</p>
          )}
        </div>
      </div>
    </Layout>
  )
}

function QuotaRow({
  label,
  current,
  max,
  unit = '',
}: {
  label: string
  current: number
  max: number
  unit?: string
}) {
  const pct = max > 0 ? Math.min((current / max) * 100, 100) : 0 // 使用率を計算する
  const isWarning = pct >= 80 // 80%以上で警告色にする

  return (
    <div>
      <div className="flex justify-between text-sm mb-1.5">
        <span className="text-gray-600">{label}</span>
        <span className={isWarning ? 'text-amber-600 font-medium' : 'text-gray-400'}>
          {current}{unit} / {max}{unit}
        </span>
      </div>
      <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${isWarning ? 'bg-amber-400' : 'bg-[#00C2D1]'}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}
