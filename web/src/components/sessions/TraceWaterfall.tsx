import { useMemo, useState } from 'react'
import type { ReactElement } from 'react'
import type { SpanNode } from '../../api/types'
import { formatDuration } from '../../lib/format'

const DEPTH_INDENT_PX = 16
const NAME_COLUMN_CLASS = 'w-44 shrink-0 sm:w-56'
const MIN_BAR_WIDTH_PERCENT = 0.5
const RULER_TICKS = [0, 25, 50, 75, 100] as const

export interface TraceWaterfallProps {
  traces: SpanNode[]
}

interface FlatSpan {
  span: SpanNode
  depth: number
  hasChildren: boolean
}

/** Depth-first flatten; skips subtrees whose ancestor is collapsed. */
function flattenVisible(
  nodes: readonly SpanNode[],
  depth: number,
  collapsed: ReadonlySet<string>,
  out: FlatSpan[],
): void {
  for (const node of nodes) {
    const isCollapsed = collapsed.has(node.span_id)
    out.push({ span: node, depth, hasChildren: node.children.length > 0 })
    if (!isCollapsed) {
      flattenVisible(node.children, depth + 1, collapsed, out)
    }
  }
}

interface TimeWindow {
  startMs: number
  totalMs: number
}

/**
 * Computes the global time window across the whole forest. Falls back to a
 * 1 ms window when all spans are instantaneous so bars stay visible.
 */
function timeWindow(nodes: readonly SpanNode[]): TimeWindow | null {
  let startMs = Number.POSITIVE_INFINITY
  let endMs = Number.NEGATIVE_INFINITY

  const walk = (list: readonly SpanNode[]): void => {
    for (const node of list) {
      startMs = Math.min(startMs, new Date(node.start_time).getTime())
      endMs = Math.max(endMs, new Date(node.end_time).getTime())
      walk(node.children)
    }
  }
  walk(nodes)

  if (!Number.isFinite(startMs) || !Number.isFinite(endMs)) {
    return null
  }
  return { startMs, totalMs: Math.max(endMs - startMs, 1) }
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
      className={`h-3.5 w-3.5 shrink-0 text-gray-500 transition-transform ${open ? 'rotate-90' : ''}`}
    >
      <path d="m9 18 6-6-6-6" />
    </svg>
  )
}

interface SpanRowProps {
  flat: FlatSpan
  window: TimeWindow
  collapsed: boolean
  onToggle: () => void
}

function SpanRow({ flat, window, collapsed, onToggle }: SpanRowProps): ReactElement {
  const { span, depth, hasChildren } = flat
  const isError = span.status === 'error'

  const startOffsetMs = Math.max(new Date(span.start_time).getTime() - window.startMs, 0)
  const leftPercent = (startOffsetMs / window.totalMs) * 100
  const widthPercent = Math.max((span.duration_ms / window.totalMs) * 100, MIN_BAR_WIDTH_PERCENT)

  const tooltip = `${span.name} · ${formatDuration(span.duration_ms)}${
    isError && span.error ? ` · ${span.error}` : ''
  }`

  return (
    <div
      className={`flex items-center gap-3 border-b border-gray-800 px-3 py-1.5 last:border-b-0 ${
        isError ? 'border-l-2 border-l-red-500/60' : 'border-l-2 border-l-transparent'
      }`}
    >
      <div
        className={`flex min-w-0 items-center gap-1 ${NAME_COLUMN_CLASS}`}
        style={{ paddingLeft: depth * DEPTH_INDENT_PX }}
      >
        {hasChildren ? (
          <button
            type="button"
            onClick={onToggle}
            aria-expanded={!collapsed}
            aria-label={collapsed ? `展开 ${span.name}` : `收起 ${span.name}`}
            className="-m-1 rounded p-1 transition-colors hover:bg-gray-800 hover:text-gray-200"
          >
            <Chevron open={!collapsed} />
          </button>
        ) : (
          <span className="w-[22px] shrink-0" aria-hidden="true" />
        )}
        <span
          className={`h-1.5 w-1.5 shrink-0 rounded-full ${isError ? 'bg-red-400' : 'bg-blue-400'}`}
          aria-hidden="true"
        />
        <span className="truncate font-mono text-xs text-gray-200" title={span.name}>
          {span.name}
        </span>
      </div>

      <div className="relative h-4 min-w-0 flex-1 rounded bg-gray-900" title={tooltip}>
        <div
          className={`absolute inset-y-0 rounded-sm ${isError ? 'bg-red-500/70' : 'bg-blue-500/60'}`}
          style={{ left: `${leftPercent}%`, width: `${widthPercent}%` }}
        />
      </div>

      <span className="w-16 shrink-0 text-right text-xs tabular-nums text-gray-500">
        {formatDuration(span.duration_ms)}
      </span>
    </div>
  )
}

function TimeRuler({ window }: { window: TimeWindow }): ReactElement {
  return (
    <div className="flex items-center gap-3 border-b border-gray-800 px-3 py-1 text-[11px] tabular-nums text-gray-600">
      <div className={`${NAME_COLUMN_CLASS} shrink-0`} />
      <div className="relative h-4 min-w-0 flex-1">
        {RULER_TICKS.map((tick) => (
          <span
            key={tick}
            className="absolute top-1/2 -translate-y-1/2"
            style={{ left: `${tick}%` }}
          >
            {formatDuration((window.totalMs * tick) / 100)}
          </span>
        ))}
      </div>
      <span className="w-16 shrink-0" aria-hidden="true" />
    </div>
  )
}

export default function TraceWaterfall({ traces }: TraceWaterfallProps) {
  const window = useMemo(() => timeWindow(traces), [traces])
  const [collapsedIds, setCollapsedIds] = useState<ReadonlySet<string>>(new Set())

  const rows = useMemo(() => {
    if (!window) return []
    const out: FlatSpan[] = []
    flattenVisible(traces, 0, collapsedIds, out)
    return out
  }, [traces, window, collapsedIds])

  if (!window || rows.length === 0) {
    return <p className="px-4 py-8 text-center text-sm text-gray-500">暂无追踪数据</p>
  }

  const toggle = (spanId: string) => {
    setCollapsedIds((prev) => {
      const next = new Set(prev)
      if (next.has(spanId)) {
        next.delete(spanId)
      } else {
        next.add(spanId)
      }
      return next
    })
  }

  return (
    <div role="img" aria-label="Span 瀑布图">
      <TimeRuler window={window} />
      {rows.map((flat) => (
        <SpanRow
          key={flat.span.span_id}
          flat={flat}
          window={window}
          collapsed={collapsedIds.has(flat.span.span_id)}
          onToggle={() => toggle(flat.span.span_id)}
        />
      ))}
    </div>
  )
}
