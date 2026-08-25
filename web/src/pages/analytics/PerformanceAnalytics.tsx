import { useMemo } from 'react'
import { useStats } from '../../hooks/useStats'
import { formatDuration } from '../../lib/format'
import { AppLineChart, AppBarChart, ChartCard } from '../../components/analytics/Charts'

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

/** Build duration buckets from daily data for a histogram-like bar chart. */
function buildDurationBuckets(daily: { avg_duration_ms: number }[]) {
  const buckets = [
    { label: '<1s', min: 0, max: 1000, count: 0 },
    { label: '1-5s', min: 1000, max: 5000, count: 0 },
    { label: '5-15s', min: 5000, max: 15_000, count: 0 },
    { label: '15-30s', min: 15_000, max: 30_000, count: 0 },
    { label: '30-60s', min: 30_000, max: 60_000, count: 0 },
    { label: '>60s', min: 60_000, max: Infinity, count: 0 },
  ]

  for (const d of daily) {
    for (const b of buckets) {
      if (d.avg_duration_ms >= b.min && d.avg_duration_ms < b.max) {
        b.count++
        break
      }
    }
  }

  return buckets.map((b) => ({ range: b.label, sessions: b.count }))
}

export default function PerformanceAnalytics() {
  const { daily, loading, error } = useStats()

  const durationBuckets = useMemo(() => buildDurationBuckets(daily), [daily])

  return (
    <div className="space-y-6">
      <h1 className="text-lg font-semibold text-gray-100">Performance Analytics</h1>

      {error && (
        <p role="alert" className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          加载数据失败：{error.message}
        </p>
      )}

      {loading ? (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-[340px] animate-pulse rounded-md border border-gray-800 bg-gray-900" />
          ))}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {/* Avg Duration Trend */}
            <ChartCard title="Avg Duration Trend (daily)">
              <AppLineChart
                data={daily}
                xKey="date"
                series={[{ dataKey: 'avg_duration_ms', name: 'Avg Duration', color: '#10b981' }]}
                tooltipFormatter={(v) => [formatDuration(v as number), 'Duration']}
                tooltipLabelFormatter={(l) => String(l)}
              />
            </ChartCard>

            {/* Success Rate Trend */}
            <ChartCard title="Success Rate Trend (daily)">
              <AppLineChart
                data={daily}
                xKey="date"
                series={[{ dataKey: 'success_rate', name: 'Success Rate', color: '#3b82f6' }]}
                tooltipFormatter={(v) => [formatPercent(v as number), 'Rate']}
                tooltipLabelFormatter={(l) => String(l)}
                yAxisFormatter={(v) => formatPercent(v)}
              />
            </ChartCard>
          </div>

          {/* Duration Distribution Histogram */}
          <ChartCard title="Duration Distribution">
            <AppBarChart
              data={durationBuckets}
              xKey="range"
              series={[{ dataKey: 'sessions', name: 'Sessions', color: '#8b5cf6' }]}
              tooltipFormatter={(v) => [String(v), 'Sessions']}
              tooltipLabelFormatter={(l) => `Range: ${l}`}
            />
          </ChartCard>
        </>
      )}
    </div>
  )
}
