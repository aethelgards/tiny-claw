import { useCallback } from 'react'
import FilterPanel, { type SessionStatusFilter } from '../components/common/FilterPanel'
import Pagination from '../components/common/Pagination'
import SessionList from '../components/sessions/SessionList'
import { useSessions } from '../hooks/useSessions'

const SKELETON_ROW_COUNT = 8

function SessionListSkeleton() {
  return (
    <div className="overflow-hidden rounded-md border border-gray-800 bg-gray-900" aria-hidden="true">
      {Array.from({ length: SKELETON_ROW_COUNT }, (_, i) => (
        <div
          key={i}
          className={`h-[49px] animate-pulse ${i < SKELETON_ROW_COUNT - 1 ? 'border-b border-gray-800' : ''}`}
        />
      ))}
    </div>
  )
}

export default function Sessions() {
  const {
    sessions,
    total,
    page,
    limit,
    status,
    search,
    loading,
    error,
    setPage,
    setStatus,
    setSearch,
  } = useSessions()

  const totalPages = Math.max(1, Math.ceil(total / limit))

  const handleStatusChange = useCallback(
    (next: SessionStatusFilter) => {
      setStatus(next === 'all' ? undefined : next)
      setPage(1)
    },
    [setStatus, setPage],
  )

  const handleSearchChange = useCallback(
    (next: string) => {
      setSearch(next)
      setPage(1)
    },
    [setSearch, setPage],
  )

  return (
    <section aria-label="会话列表" className="space-y-4">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-lg font-semibold text-gray-100">
          会话列表
          <span className="ml-2 text-sm font-normal tabular-nums text-gray-500">
            {total.toLocaleString()} 条
          </span>
        </h1>
        <FilterPanel
          status={(status ?? 'all') as SessionStatusFilter}
          search={search}
          onStatusChange={handleStatusChange}
          onSearchChange={handleSearchChange}
        />
      </div>

      {error && (
        <p role="alert" className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          加载会话失败：{error.message}
        </p>
      )}

      {loading && sessions.length === 0 ? (
        <SessionListSkeleton />
      ) : sessions.length === 0 ? (
        <p className="rounded-md border border-gray-800 bg-gray-900 px-4 py-8 text-center text-sm text-gray-500">
          暂无会话记录
        </p>
      ) : (
        <SessionList sessions={sessions} />
      )}

      {total > 0 && (
        <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
      )}
    </section>
  )
}
