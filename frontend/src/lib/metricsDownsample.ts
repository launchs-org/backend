import type { DeploymentMetrics } from '@/lib/types'

// 表示期間ごとのサンプリング間隔（N件に1件を採用する）
const DOWNSAMPLE_STEP: Record<string, number> = {
  '1時間':  1,  // 30秒間隔 → 間引きなし（120ポイント）
  '3時間':  3,  // 30秒×3 = 90秒に1件（120ポイント）
  '24時間': 24, // 30秒×24 = 12分に1件（120ポイント）
  '3日':    72, // 30秒×72 = 36分に1件（120ポイント）
}

/**
 * downsampleMetrics はメトリクス配列を表示期間に応じた粒度に間引いて返す。
 * APIは RecordedAt DESC 順で返すため、先に昇順に並べ替えてからサンプリングし、
 * グラフ用に昇順のまま返す。
 */
export function downsampleMetrics(
  metrics: DeploymentMetrics[],
  rangeLabel: string,
): DeploymentMetrics[] {
  const step = DOWNSAMPLE_STEP[rangeLabel] ?? 1 // 未知のラベルは間引きなしとする

  if (step <= 1) {
    return metrics // 間引き不要の場合はそのまま返す
  }

  // API は RecordedAt DESC 順なので昇順に並べ替えてからサンプリングする
  const sorted = [...metrics].reverse() // 昇順に並べ替える

  // N件ごとに先頭要素を代表値として採用する
  const sampled = sorted.filter((_, itemIndex) => itemIndex % step === 0) // step件ごとに1件を採用する

  // MetricsCharts は RecordedAt DESC 順を期待しているので昇順を逆順に戻して返す
  return sampled.reverse() // DESC 順に戻して返す
}
