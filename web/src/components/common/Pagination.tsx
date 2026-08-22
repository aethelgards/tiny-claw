export interface PaginationProps {
  page: number
  totalPages: number
  onPageChange: (page: number) => void
}

const buttonBase =
  'rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-40'

export default function Pagination({ page, totalPages, onPageChange }: PaginationProps) {
  const hasPrev = page > 1
  const hasNext = page < totalPages

  return (
    <nav className="flex items-center justify-center gap-3" aria-label="Pagination">
      <button
        type="button"
        className={`${buttonBase} border border-gray-800 bg-gray-900 text-gray-300 hover:bg-gray-800/60 hover:text-gray-100`}
        onClick={() => hasPrev && onPageChange(page - 1)}
        disabled={!hasPrev}
        aria-label="上一页"
      >
        上一页
      </button>
      <span className="text-sm tabular-nums text-gray-400">
        第 <span className="text-gray-100">{page}</span> / {totalPages} 页
      </span>
      <button
        type="button"
        className={`${buttonBase} border border-gray-800 bg-gray-900 text-gray-300 hover:bg-gray-800/60 hover:text-gray-100`}
        onClick={() => hasNext && onPageChange(page + 1)}
        disabled={!hasNext}
        aria-label="下一页"
      >
        下一页
      </button>
    </nav>
  )
}
