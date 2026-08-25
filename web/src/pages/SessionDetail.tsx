import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import type { ReactElement, ReactNode } from 'react'
import StatusBadge from '../components/common/StatusBadge'
import MessageFlow, { type FlowMessage } from '../components/sessions/MessageFlow'
import ToolCallTree from '../components/sessions/ToolCallTree'
import TraceWaterfall from '../components/sessions/TraceWaterfall'
import { api } from '../api/client'
import type { SessionData, SpanNode, ToolCallRecord } from '../api/types'
import { useTraces } from '../hooks/useTraces'
import { formatDuration, formatTokens } from '../lib/format'

const SESSION_ID_DISPLAY_LENGTH = 8
const LLM_ACTION_SPAN_NAME = 'LLM.Action'
const UNRECORDED_ASSISTANT_CONTENT = '（该轮次未记录文本输出）'
const SKELETON_SECTION_COUNT = 3

function shortId(id: string): string {
  return id.length <= SESSION_ID_DISPLAY_LENGTH ? id : id.slice(0, SESSION_ID_DISPLAY_LENGTH)
}

function spanAttributeText(span: SpanNode, key: string): string | undefined {
  const value = span.attributes?.[key]
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

/** Depth-first walk collecting one assistant message per LLM.Action span. */
function collectAssistantMessages(nodes: readonly SpanNode[], out: FlowMessage[]): void {
  for (const node of nodes) {
    if (node.name === LLM_ACTION_SPAN_NAME) {
      out.push({
        id: node.span_id,
        role: 'assistant',
        kind: 'text',
        content: spanAttributeText(node, 'content') ?? UNRECORDED_ASSISTANT_CONTENT,
        timestamp: node.start_time,
      })
    }
    collectAssistantMessages(node.children, out)
  }
}

/**
 * Rebuilds the chat flow from observability data: the user prompt comes from
 * the session record, assistant turns from LLM.Action spans and tool calls
 * from the tool call records. Merged chronologically.
 */
export function buildFlowMessages(
  session: SessionData,
  traces: readonly SpanNode[],
  tools: readonly ToolCallRecord[],
): FlowMessage[] {
  const messages: FlowMessage[] = []

  if (session.prompt) {
    messages.push({
      id: `user-${session.id}`,
      role: 'user',
      kind: 'text',
      content: session.prompt,
      timestamp: session.created_at,
    })
  }

  collectAssistantMessages(traces, messages)

  for (const tool of tools) {
    messages.push({
      id: `tool-${tool.span_id}`,
      role: 'assistant',
      kind: 'tool',
      content: tool.result,
      timestamp: tool.start_time,
      toolName: tool.tool_name,
      isError: tool.is_error,
      durationMs: tool.duration_ms,
    })
  }

  return messages.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
}

interface MetaItemProps {
  label: string
  children: ReactNode
}

function MetaItem({ label, children }: MetaItemProps): ReactElement {
  return (
    <div className="min-w-0">
      <p className="text-xs text-gray-500">{label}</p>
      <div className="mt-1 truncate text-sm tabular-nums text-gray-100">{children}</div>
    </div>
  )
}

interface SectionProps {
  label: string
  title: string
  children: ReactNode
}

function Section({ label, title, children }: SectionProps): ReactElement {
  return (
    <section aria-label={label}>
      <h2 className="text-base font-semibold text-gray-100">{title}</h2>
      <div className="mt-3 rounded-md border border-gray-800 bg-gray-900 p-4">{children}</div>
    </section>
  )
}

function DetailSkeleton(): ReactElement {
  return (
    <div className="space-y-6" aria-hidden="true">
      <div className="h-36 animate-pulse rounded-md border border-gray-800 bg-gray-900" />
      {Array.from({ length: SKELETON_SECTION_COUNT }, (_, i) => (
        <div key={i} className="h-48 animate-pulse rounded-md border border-gray-800 bg-gray-900" />
      ))}
    </div>
  )
}

export default function SessionDetail(): ReactElement {
  const { id } = useParams()
  const tracesState = useTraces(id)

  const [session, setSession] = useState<SessionData | null>(null)
  const [sessionLoading, setSessionLoading] = useState(true)
  const [sessionError, setSessionError] = useState<Error | null>(null)

  useEffect(() => {
    if (!id) {
      setSession(null)
      setSessionLoading(false)
      return
    }

    let cancelled = false
    setSessionLoading(true)
    setSessionError(null)
    setSession(null)

    api.sessions
      .get(id)
      .then((data) => {
        if (!cancelled) setSession(data)
      })
      .catch((err: unknown) => {
        if (!cancelled) setSessionError(err instanceof Error ? err : new Error(String(err)))
      })
      .finally(() => {
        if (!cancelled) setSessionLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [id])

  const messages = useMemo(
    () => (session ? buildFlowMessages(session, tracesState.traces, tracesState.tools) : []),
    [session, tracesState.traces, tracesState.tools],
  )

  const error = sessionError ?? tracesState.error
  const loading = sessionLoading || (tracesState.loading && !session)

  if (loading && !error) {
    return <DetailSkeleton />
  }

  if (error) {
    return (
      <div className="space-y-4">
        <p role="alert" className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          加载会话失败：{error.message}
        </p>
        <Link to="/sessions" className="text-sm text-gray-400 transition-colors hover:text-gray-200">
          ← 返回会话列表
        </Link>
      </div>
    )
  }

  if (!session) {
    return (
      <p className="rounded-md border border-gray-800 bg-gray-900 px-4 py-8 text-center text-sm text-gray-500">
        会话不存在
      </p>
    )
  }

  return (
    <div className="space-y-6">
      <header className="rounded-md border border-gray-800 bg-gray-900 p-5">
        <Link to="/sessions" className="text-xs text-gray-500 transition-colors hover:text-gray-300">
          ← 返回会话列表
        </Link>
        <h1 className="text-lg font-semibold leading-snug text-gray-100" title={session.prompt}>
          {session.prompt}
        </h1>
        <div className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-6">
          <MetaItem label="会话 ID">
            <span className="font-mono" title={session.id}>
              #{shortId(session.id)}
            </span>
          </MetaItem>
          <MetaItem label="状态">
            <StatusBadge status={session.status} />
          </MetaItem>
          <MetaItem label="模型">{session.model}</MetaItem>
          <MetaItem label="耗时">{formatDuration(session.duration_ms)}</MetaItem>
          <MetaItem label="Token">{formatTokens(session.total_tokens.total_tokens)}</MetaItem>
          <MetaItem label="Cost">${session.total_cost.toFixed(4)}</MetaItem>
        </div>
      </header>

      <Section label="消息流" title="消息流">
        <MessageFlow messages={messages} />
      </Section>

      <Section label="工具调用" title="工具调用">
        <ToolCallTree tools={tracesState.tools} />
      </Section>

      <Section label="Span 瀑布图" title="Span 瀑布图">
        <TraceWaterfall traces={tracesState.traces} />
      </Section>
    </div>
  )
}
