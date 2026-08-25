import { useStats } from '../../hooks/useStats'
import { AppLineChart, AppPieChart, ChartCard } from '../../components/analytics/Charts'

function formatCost(value: number): string {
  return `$${value.toFixed(4)}`
}

function formatShortDate(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

export default function CostAnalytics() {
  const { daily, models, loading, error } = useStats()

  const totalCost = daily.reduce((sum, d) => sum + d.cost, 0)

  const topSessions = [...daily]
    .sort((a, b) => b.cost - a.cost)
    .slice(0, 10)

  const pieData = models.map((m) => ({ name: m.model, value: m.cost }))

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-gray-100">Cost Analysis</h1>
        <span className="text-sm text-gray-400">
          Total: {formatCost(totalCost)}
        </span>
      </div>

      {error && (
        <p role="alert" className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          加载数据失败：{error.message}
        </p>
      )}

      {loading ? (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {[0, 1].map((i) => (
            <div key={i} className="h-[340px] animate-pulse rounded-md border border-gray-800 bg-gray-900" />
          ))}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {/* Daily Cost Trend */}
            <ChartCard title="Daily Cost Trend" className="lg:col-span-2">
              <AppLineChart
                data={daily}
                xKey="date"
                series={[{ dataKey: 'cost', name: 'Cost', color: '#3b82f6' }]}
                tooltipFormatter={(v) => [formatCost(v as number), 'Cost']}
                tooltipLabelFormatter={(l) => String(l)}
              />
            </ChartCard>
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {/* Model Cost Distribution */}
            <ChartCard title="Model Cost Distribution">
              {pieData.length > 0 ? (
                <AppPieChart
                  data={pieData}
                  tooltipFormatter={(v) => [formatCost(v as number), 'Cost']}
                />
              ) : (
                <p className="py-8 text-center text-sm text-gray-500">No model data</p>
              )}
            </ChartCard>

            {/* Top Sessions by Cost */}
            <ChartCard title="Top Sessions by Cost">
              {topSessions.length > 0 ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead>
                      <tr className="border-b border-gray-800 text-gray-400">
                        <th className="pb-2 font-medium">Date</th>
                        <th className="pb-2 font-medium">Cost</th>
                        <th className="pb-2 font-medium">Sessions</th>
                        <th className="pb-2 font-medium">Tokens</th>
                      </tr>
                    </thead>
                    <tbody>
                      {topSessions.map((s) => (
                        <tr key={s.date} className="border-b border-gray-800/50 text-gray-300">
                          <td className="py-2 tabular-nums">{formatShortDate(s.date)}</td>
                          <td className="py-2 tabular-nums text-gray-100">{formatCost(s.cost)}</td>
                          <td className="py-2 tabular-nums">{s.sessions}</td>
                          <td className="py-2 tabular-nums">{s.tokens.toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="py-8 text-center text-sm text-gray-500">No session data</p>
              )}
            </ChartCard>
          </div>
        </>
      )}
    </div>
  )
}
