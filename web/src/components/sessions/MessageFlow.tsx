import type { ReactElement } from 'react'

const BUBBLE_MAX_WIDTH = 'max-w-[85%] sm:max-w-[75%]'
const CONTENT_PREVIEW_LENGTH = 160

export interface FlowMessage {
  id: string
  role: 'user' | 'assistant'
  kind: 'text' | 'tool'
  content: string
  timestamp: string
  toolName?: string
  isError?: boolean
  durationMs?: number
}

export interface MessageFlowProps {
  messages: FlowMessage[]
}

function firstLine(text: string): string {
  const line = text.split('\n', 1)[0] ?? ''
  if (line.length <= CONTENT_PREVIEW_LENGTH) {
    return line
  }
  return `${line.slice(0, CONTENT_PREVIEW_LENGTH)}…`
}

function RoleAvatar({ role }: { role: FlowMessage['role'] }): ReactElement {
  const isUser = role === 'user'
  return (
    <span
      aria-hidden="true"
      className={`mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full ring-1 ring-inset ${
        isUser ? 'bg-blue-500/10 text-blue-400 ring-blue-500/30' : 'bg-gray-800 text-gray-400 ring-gray-700'
      }`}
    >
      {isUser ? (
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5">
          <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" />
          <circle cx="12" cy="7" r="4" />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5">
          <rect x="4" y="4" width="16" height="16" rx="2" />
          <rect x="9" y="9" width="6" height="6" />
        </svg>
      )}
    </span>
  )
}

function Timestamp({ value }: { value: string }): ReactElement {
  const time = new Date(value)
  const label = Number.isNaN(time.getTime()) ? value : time.toLocaleTimeString('zh-CN', { hour12: false })
  return (
    <time dateTime={value} className="text-[11px] tabular-nums text-gray-500">
      {label}
    </time>
  )
}

function TextBubble({ message }: { message: FlowMessage }): ReactElement {
  const isUser = message.role === 'user'
  return (
    <div className={`min-w-0 ${isUser ? 'items-end' : 'items-start'} flex flex-col gap-1 ${BUBBLE_MAX_WIDTH}`}>
      <div
        className={`whitespace-pre-wrap break-words rounded-md border px-3.5 py-2.5 text-sm leading-relaxed ${
          isUser ? 'border-blue-500/30 bg-blue-500/10 text-gray-100' : 'border-gray-700 bg-gray-800/60 text-gray-100'
        }`}
      >
        {message.content}
      </div>
      <Timestamp value={message.timestamp} />
    </div>
  )
}

function ToolBubble({ message }: { message: FlowMessage }): ReactElement {
  return (
    <div className={`flex min-w-0 flex-col items-start gap-1 ${BUBBLE_MAX_WIDTH}`}>
      <div
        className={`w-full rounded-md border px-3 py-2 font-mono text-xs ${
          message.isError ? 'border-red-500/30 bg-red-500/10' : 'border-gray-700 bg-gray-900'
        }`}
      >
        <div className="flex items-center gap-2">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden="true">
            <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
          </svg>
          <span className={message.isError ? 'text-red-400' : 'text-gray-200'}>{message.toolName ?? 'tool'}</span>
          {typeof message.durationMs === 'number' && (
            <span className="ml-auto shrink-0 tabular-nums text-gray-500">{message.durationMs} ms</span>
          )}
        </div>
        <p className="mt-1 truncate text-gray-500" title={message.content}>
          {firstLine(message.content) || '（无输出）'}
        </p>
      </div>
      <Timestamp value={message.timestamp} />
    </div>
  )
}

export default function MessageFlow({ messages }: MessageFlowProps) {
  if (messages.length === 0) {
    return (
      <p className="px-4 py-8 text-center text-sm text-gray-500">暂无消息记录</p>
    )
  }

  return (
    <div className="space-y-4" role="log" aria-label="会话消息流">
      {messages.map((message) => (
        <div key={message.id} className={`flex gap-2.5 ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}>
          {message.role === 'assistant' && <RoleAvatar role="assistant" />}
          {message.kind === 'tool' ? <ToolBubble message={message} /> : <TextBubble message={message} />}
          {message.role === 'user' && <RoleAvatar role="user" />}
        </div>
      ))}
    </div>
  )
}
