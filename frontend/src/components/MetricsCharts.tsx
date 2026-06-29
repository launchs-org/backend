import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import type { DeploymentMetrics } from '@/lib/types'

// Pod ごとに割り当てる色の一覧（最大 8 Pod まで対応）
const POD_COLORS = [
  '#00C2D1',
  '#F59E0B',
  '#6366F1',
  '#10B981',
  '#EF4444',
  '#8B5CF6',
  '#F97316',
  '#06B6D4',
]

type ChartDataPoint = {
  time: string           // X 軸ラベル（HH:mm:ss 形式）
  [podName: string]: number | string  // Pod 名をキーとした CPU / メモリ値
}

type MetricsChartsProps = {
  metrics: DeploymentMetrics[]  // API から取得したメトリクス一覧（RecordedAt DESC 順）
}

// formatTime は ISO 文字列を HH:mm:ss 形式に変換する
// timeZone を明示しないとブラウザのシステム TZ で表示されるため Asia/Tokyo を固定指定する
function formatTime(isoString: string): string {
  const date = new Date(isoString)
  return date.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit', second: '2-digit', timeZone: 'Asia/Tokyo' })
}

// formatMemory はバイト数を人間が読みやすい形式に変換する
function formatMemory(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GiB`
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${bytes} B`
}

export function MetricsCharts({ metrics }: MetricsChartsProps) {
  if (metrics.length === 0) {  // メトリクスが空の場合は空状態を表示する
    return (
      <div className="flex items-center justify-center h-48 text-sm text-gray-400">
        メトリクスデータがまだありません。収集には最大 30 秒かかります。
      </div>
    )
  }

  // Pod 名の一覧を重複排除して取得する
  const podNames = Array.from(new Set(metrics.map((metric) => metric.pod_name)))

  // 時刻でグルーピングして各 Pod の値を同じデータポイントにまとめる
  // API は RecordedAt DESC 順なので reverse して昇順にする
  const sortedMetrics = [...metrics].reverse()

  // 時刻 → Pod 名 → 値 のマップを構築する
  const timeMap = new Map<string, { cpu: Map<string, number>; memory: Map<string, number>; readyReplicas: number; totalReplicas: number }>()
  for (const metric of sortedMetrics) {
    const timeKey = metric.recorded_at
    if (!timeMap.has(timeKey)) {
      timeMap.set(timeKey, { cpu: new Map(), memory: new Map(), readyReplicas: 0, totalReplicas: 0 })
    }
    const entry = timeMap.get(timeKey)!
    entry.cpu.set(metric.pod_name, metric.cpu_millicores)          // CPU 使用量を記録する
    entry.memory.set(metric.pod_name, metric.memory_bytes)         // メモリ使用量を記録する
    entry.readyReplicas = metric.ready_replicas                    // レプリカ数はどの Pod からでも同じ値
    entry.totalReplicas = metric.total_replicas                    // 合計レプリカ数を記録する
  }

  // CPU チャート用データポイントを構築する
  const cpuData: ChartDataPoint[] = Array.from(timeMap.entries()).map(([timeKey, entry]) => {
    const point: ChartDataPoint = { time: formatTime(timeKey) }
    for (const podName of podNames) {
      point[podName] = entry.cpu.get(podName) ?? 0  // 値がない場合は 0 で補完する
    }
    return point
  })

  // メモリチャート用データポイントを構築する
  const memoryData: ChartDataPoint[] = Array.from(timeMap.entries()).map(([timeKey, entry]) => {
    const point: ChartDataPoint = { time: formatTime(timeKey) }
    for (const podName of podNames) {
      point[podName] = Math.round((entry.memory.get(podName) ?? 0) / (1024 * 1024))  // MiB に変換する
    }
    return point
  })

  // レプリカチャート用データポイントを構築する（ready / total の 2 系列）
  const replicaData = Array.from(timeMap.entries()).map(([timeKey, entry]) => ({
    time: formatTime(timeKey),
    ready: entry.readyReplicas,    // Ready レプリカ数
    total: entry.totalReplicas,    // 合計レプリカ数
  }))

  return (
    <div className="space-y-8">
      {/* CPU 使用量グラフ */}
      <div className="space-y-2">
        <h3 className="text-sm font-medium text-gray-700">CPU 使用量 (millicores)</h3>
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={cpuData} margin={{ top: 4, right: 16, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="time" tick={{ fontSize: 11 }} tickLine={false} />
            <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} unit="m" />
            <Tooltip
              contentStyle={{ fontSize: 12, borderRadius: 6, border: '1px solid #e5e7eb' }}
              formatter={(value) => [`${value ?? 0}m`, '']}
            />
            {podNames.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
            {podNames.map((podName, podIndex) => (
              <Line
                key={podName}
                type="monotone"
                dataKey={podName}
                stroke={POD_COLORS[podIndex % POD_COLORS.length]}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}  // ポーリング更新時のアニメーションを無効化する
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* メモリ使用量グラフ */}
      <div className="space-y-2">
        <h3 className="text-sm font-medium text-gray-700">メモリ使用量 (MiB)</h3>
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={memoryData} margin={{ top: 4, right: 16, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="time" tick={{ fontSize: 11 }} tickLine={false} />
            <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} unit=" MiB" />
            <Tooltip
              contentStyle={{ fontSize: 12, borderRadius: 6, border: '1px solid #e5e7eb' }}
              formatter={(value, name) => [formatMemory((Number(value ?? 0)) * 1024 * 1024), name]}
            />
            {podNames.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
            {podNames.map((podName, podIndex) => (
              <Line
                key={podName}
                type="monotone"
                dataKey={podName}
                stroke={POD_COLORS[podIndex % POD_COLORS.length]}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}  // ポーリング更新時のアニメーションを無効化する
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* レプリカ数グラフ */}
      <div className="space-y-2">
        <h3 className="text-sm font-medium text-gray-700">レプリカ数</h3>
        <ResponsiveContainer width="100%" height={160}>
          <LineChart data={replicaData} margin={{ top: 4, right: 16, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="time" tick={{ fontSize: 11 }} tickLine={false} />
            <YAxis allowDecimals={false} tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
            <Tooltip contentStyle={{ fontSize: 12, borderRadius: 6, border: '1px solid #e5e7eb' }} />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            <Line
              type="stepAfter"
              dataKey="ready"
              name="Ready"
              stroke="#10B981"
              strokeWidth={1.5}
              dot={false}
              isAnimationActive={false}
            />
            <Line
              type="stepAfter"
              dataKey="total"
              name="Total"
              stroke="#9CA3AF"
              strokeWidth={1.5}
              strokeDasharray="4 2"
              dot={false}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
