import SessionCard from '../components/sessions/SessionCard'
import { useSessions } from '../hooks/useSessions'
import { useStats } from '../hooks/useStats'

const RECENT_SESSIONS_COUNT = 5

function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(1)}M`
  }
  if (tokens >= 1_000) {
    return `${(tokens / 1_000).toFixed(1)}K`
  }
  return tokens.toLocaleString()
}

interface StatCardProps {
  label: string
  value: string
}

const STAT_CARD_CLASS =
  'rounded-md border border-gray-800 bg-gray-900 p-5 transition-colors hover:border-gray-700'

function StatCard({ label, value }: StatCardProps) {
  return (
    <div className={STAT_CARD_CLASS}>
      <p className="text-sm text-gray-400">{label}</p>
      <p className="mt-2 text-2xl font-semibold tabular-nums text-gray-100">{value}</p>
    </div>
  )
}

function StatCardSkeleton() {
  return (
    <div className={`${STAT_CARD_CLASS} animate-pulse`} aria-hidden="true">
      <div className="h-4 w-20 rounded bg-gray-800" />
      <div className="mt-3 h-7 w-24 rounded bg-gray-800" />
    </div>
  )
}

export default function Dashboard() {
  const stats = useStats()
  const sessions = useSessions({ page: 1, limit: RECENT_SESSIONS_COUNT })

  const overview = stats.overview

  return (
    <div className="space-y-6">
      <section aria-label="统计概览">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {stats.loading && overview === null ? (
            <>
              <StatCardSkeleton />
              <StatCardSkeleton />
              <StatCardSkeleton />
              <StatCardSkeleton />
            </>
          ) : (
            <>
              <StatCard label="Total Sessions" value={(overview?.total_sessions ?? 0).toLocaleString()} />
              <StatCard label="Total Cost" value={`$${(overview?.total_cost ?? 0).toFixed(4)}`} />
              <StatCard label="Total Tokens" value={formatTokens(overview?.total_tokens ?? 0)} />
              <StatCard
                label="Success Rate"
                value={`${((overview?.success_rate ?? 0) * 100).toFixed(1)}%`}
              />
            </>
          )}
        </div>
      </section>

      {(stats.error || sessions.error) && (
        <p role="alert" className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          加载数据失败：{(stats.error ?? sessions.error)?.message}
        </p>
      )}

      <section aria-label="最近会话">
        <h2 className="text-base font-semibold text-gray-100">最近会话</h2>
        {sessions.loading && sessions.sessions.length === 0 ? (
          <div className="mt-3 space-y-3" aria-hidden="true">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-[76px] animate-pulse rounded-md border border-gray-800 bg-gray-900" />
            ))}
          </div>
        ) : sessions.sessions.length === 0 ? (
          <p className="mt-3 text-sm text-gray-500">暂无会话记录</p>
        ) : (
          <div className="mt-3 space-y-3">
            {sessions.sessions.map((session) => (
              <SessionCard key={session.id} session={session} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
