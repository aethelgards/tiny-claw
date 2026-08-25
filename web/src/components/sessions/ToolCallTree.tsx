import { useMemo, useState } from 'react'
import type { ReactElement } from 'react'
import type { ToolCallRecord } from '../../api/types'
import { formatClock, formatDuration } from '../../lib/format'

const RESULT_MAX_HEIGHT_CLASS = 'max-h-64'

export interface ToolCallTreeProps {
  tools: ToolCallRecord[]
}

function sortedByStartTime(tools: ToolCallRecord[]): ToolCallRecord[] {
  return [...tools].sort(
    (a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime(),
  )
}

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function Chevron({ open }: { open: boolean }): ReactElement {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={`h-4 w-4 shrink-0 text-gray-500 transition-transform ${open ? 'rotate-90' : ''}`}
    >
      <path d="m9 18 6-6-6-6" />
    </svg>
  )
}

interface ToolRowProps {
  tool: ToolCallRecord
  open: boolean
  onToggle: () => void
}

function ToolRow({ tool, open, onToggle }: ToolRowProps): ReactElement {
  const isError = tool.is_error
  return (
    <li
      className={`border-b border-gray-800 last:border-b-0 ${
        isError ? 'border-l-2 border-l-red-500/60' : 'border-l-2 border-l-transparent'
      }`}
    >
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left transition-colors hover:bg-gray-800/60"
      >
        <Chevron open={open} />
        <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${isError ? 'bg-red-400' : 'bg-green-400'}`} aria-hidden="true" />
        <span className="truncate font-mono text-sm text-gray-100">{tool.tool_name}</span>
        <span className="ml-auto flex shrink-0 items-center gap-3 text-xs tabular-nums text-gray-500">
          <time dateTime={tool.start_time}>{formatClock(tool.start_time)}</time>
          <span>{formatDuration(tool.duration_ms)}</span>
          <span
            className={`rounded-full px-2 py-0.5 font-medium ring-1 ring-inset ${
              isError ? 'bg-red-500/10 text-red-400 ring-red-500/30' : 'bg-green-500/10 text-green-400 ring-green-500/30'
            }`}
          >
            {isError ? '错误' : '成功'}
          </span>
        </span>
      </button>

      {open && (
        <div className="space-y-3 px-3 pb-3 pl-9">
          <div>
            <p className="mb-1 text-xs font-medium text-gray-500">参数</p>
            <pre className={`${RESULT_MAX_HEIGHT_CLASS} overflow-auto rounded-md border border-gray-800 bg-gray-950 p-3 font-mono text-xs leading-relaxed text-gray-300`}>
              {prettyJSON(tool.arguments) || '（无参数）'}
            </pre>
          </div>
          <div>
            <p className="mb-1 text-xs font-medium text-gray-500">结果</p>
            <pre
              className={`${RESULT_MAX_HEIGHT_CLASS} overflow-auto rounded-md border p-3 font-mono text-xs leading-relaxed ${
                isError ? 'border-red-500/30 bg-red-500/5 text-red-300' : 'border-gray-800 bg-gray-950 text-gray-300'
              }`}
            >
              {tool.result || '（无输出）'}
            </pre>
          </div>
        </div>
      )}
    </li>
  )
}

export default function ToolCallTree({ tools }: ToolCallTreeProps) {
  const ordered = useMemo(() => sortedByStartTime(tools), [tools])
  const [expandedIds, setExpandedIds] = useState<ReadonlySet<string>>(new Set())

  const allExpanded = ordered.length > 0 && ordered.every((tool) => expandedIds.has(tool.span_id))

  const toggleOne = (spanId: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(spanId)) {
        next.delete(spanId)
      } else {
        next.add(spanId)
      }
      return next
    })
  }

  const toggleAll = () => {
    setExpandedIds(allExpanded ? new Set() : new Set(ordered.map((tool) => tool.span_id)))
  }

  if (ordered.length === 0) {
    return <p className="px-4 py-8 text-center text-sm text-gray-500">暂无工具调用</p>
  }

  return (
    <div>
      <div className="flex items-center justify-between border-b border-gray-800 px-3 py-2">
        <span className="text-xs text-gray-500">
          共 <span className="tabular-nums">{ordered.length}</span> 次调用
        </span>
        <button
          type="button"
          onClick={toggleAll}
          className="text-xs text-gray-400 transition-colors hover:text-gray-200"
        >
          {allExpanded ? '全部收起' : '全部展开'}
        </button>
      </div>
      <ul aria-label="工具调用列表">
        {ordered.map((tool) => (
          <ToolRow
            key={tool.span_id}
            tool={tool}
            open={expandedIds.has(tool.span_id)}
            onToggle={() => toggleOne(tool.span_id)}
          />
        ))}
      </ul>
    </div>
  )
}
