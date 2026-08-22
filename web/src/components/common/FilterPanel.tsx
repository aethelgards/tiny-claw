import SearchBar from './SearchBar'

export type SessionStatusFilter = 'all' | 'running' | 'completed' | 'failed'

export interface FilterPanelProps {
  status: SessionStatusFilter
  search: string
  onStatusChange: (status: SessionStatusFilter) => void
  onSearchChange: (search: string) => void
}

const STATUS_OPTIONS: { value: SessionStatusFilter; label: string }[] = [
  { value: 'all', label: '全部状态' },
  { value: 'running', label: 'Running' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
]

const SELECT_CLASS =
  'rounded-md border border-gray-800 bg-gray-900 px-3 py-2 text-sm text-gray-100 transition-colors hover:border-gray-700 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500'

export default function FilterPanel({
  status,
  search,
  onStatusChange,
  onSearchChange,
}: FilterPanelProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
      <select
        value={status}
        onChange={(event) => onStatusChange(event.target.value as SessionStatusFilter)}
        aria-label="按状态过滤"
        className={`${SELECT_CLASS} sm:w-44`}
      >
        {STATUS_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <div className="sm:w-72">
        <SearchBar value={search} onChange={onSearchChange} placeholder="搜索会话..." />
      </div>
    </div>
  )
}
