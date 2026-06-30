import { Download } from 'lucide-react'
import type { DeploymentMetrics } from '@/lib/types'

type MetricsDownloadButtonProps = {
  metrics: DeploymentMetrics[]  // ダウンロード対象のメトリクス一覧（間引きなしの生データ）
  deploymentId: string          // ファイル名に使用するデプロイメントID
}

// buildCsv はメトリクス配列をCSV文字列に変換する
function buildCsv(metrics: DeploymentMetrics[]): string {
  const header = 'recorded_at,pod_name,cpu_millicores,memory_bytes,ready_replicas,total_replicas' // CSVヘッダーを定義する
  const rows = metrics.map((metric) =>
    [
      metric.recorded_at,
      metric.pod_name,
      metric.cpu_millicores,
      metric.memory_bytes,
      metric.ready_replicas,
      metric.total_replicas,
    ].join(',') // 各フィールドをカンマ区切りで結合する
  )
  return [header, ...rows].join('\n') // ヘッダーとデータ行を改行で結合する
}

export function MetricsDownloadButton({ metrics, deploymentId }: MetricsDownloadButtonProps) {
  const handleDownload = () => {
    const csvContent = buildCsv(metrics)                                          // CSVデータを構築する
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })      // Blob オブジェクトを生成する
    const url = URL.createObjectURL(blob)                                          // オブジェクトURLを生成する
    const anchor = document.createElement('a')                                    // ダウンロード用リンク要素を生成する
    anchor.href = url                                                              // URLを設定する
    anchor.download = `metrics-${deploymentId.slice(0, 8)}.csv`                  // ファイル名を設定する
    anchor.click()                                                                 // クリックしてダウンロードを開始する
    URL.revokeObjectURL(url)                                                       // オブジェクトURLを解放する
  }

  return (
    <button
      onClick={handleDownload}
      disabled={metrics.length === 0}  // メトリクスがない場合は無効化する
      className="flex items-center gap-1.5 text-xs text-gray-400 hover:text-gray-600 border border-gray-200 rounded-md px-2.5 py-1 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
    >
      <Download className="w-3.5 h-3.5" />
      CSVダウンロード
    </button>
  )
}
