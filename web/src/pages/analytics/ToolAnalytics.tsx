import { useStats } from '../../hooks/useStats'
import { AppBarChart, ChartCard } from '../../components/analytics/Charts'

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms.toFixed(0)} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

export default function ToolAnalytics() {
  const { tools, loading, error } = useStats()

  const callData = tools.map((t) => ({ name: t.tool_name, calls: t.calls }))
  const durationData = tools.map((t) => ({ name: t.tool_name, 'Avg Duration (ms)': t.avg_duration_ms }))

  return (
    <div className="space-y-6">
      <h1 className="text-lg font-semibold text-gray-100">Tool Analytics</h1>

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
            {/* Tool Call Counts */}
            <ChartCard title="Tool Call Counts">
              {callData.length > 0 ? (
                <AppBarChart
                  data={callData}
                  xKey="name"
                  series={[{ dataKey: 'calls', name: 'Calls', color: '#3b82f6' }]}
                  tooltipFormatter={(v) => [String(v), 'Calls']}
                />
              ) : (
                <p className="py-8 text-center text-sm text-gray-500">No tool data</p>
              )}
            </ChartCard>

            {/* Tool Avg Duration */}
            <ChartCard title="Tool Avg Duration">
              {durationData.length > 0 ? (
                <AppBarChart
                  data={durationData}
                  xKey="name"
                  series={[{ dataKey: 'Avg Duration (ms)', name: 'Avg Duration', color: '#10b981' }]}
                  tooltipFormatter={(v) => [formatDuration(v as number), 'Duration']}
                />
              ) : (
                <p className="py-8 text-center text-sm text-gray-500">No tool data</p>
              )}
            </ChartCard>
          </div>

          {/* Tool Error Rate Table */}
          <ChartCard title="Tool Error Rate">
            {tools.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-gray-800 text-gray-400">
                      <th className="pb-2 font-medium">Tool</th>
                      <th className="pb-2 font-medium">Calls</th>
                      <th className="pb-2 font-medium">Errors</th>
                      <th className="pb-2 font-medium">Error Rate</th>
                      <th className="pb-2 font-medium">Avg Duration</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tools.map((t) => {
                      const errorRate = t.calls > 0 ? t.errors / t.calls : 0
                      return (
                        <tr key={t.tool_name} className="border-b border-gray-800/50 text-gray-300">
                          <td className="py-2 font-medium text-gray-100">{t.tool_name}</td>
                          <td className="py-2 tabular-nums">{t.calls.toLocaleString()}</td>
                          <td className="py-2 tabular-nums">{t.errors.toLocaleString()}</td>
                          <td className="py-2 tabular-nums">
                            <span
                              className={
                                errorRate > 0.1
                                  ? 'text-red-400'
                                  : errorRate > 0
                                    ? 'text-amber-400'
                                    : 'text-gray-400'
                              }
                            >
                              {(errorRate * 100).toFixed(1)}%
                            </span>
                          </td>
                          <td className="py-2 tabular-nums">{formatDuration(t.avg_duration_ms)}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="py-8 text-center text-sm text-gray-500">No tool data</p>
            )}
          </ChartCard>
        </>
      )}
    </div>
  )
}
