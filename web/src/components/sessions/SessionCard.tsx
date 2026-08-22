import { Link } from 'react-router-dom'
import StatusBadge from '../common/StatusBadge'
import TimeAgo from '../common/TimeAgo'
import type { SessionData } from '../../api/types'

const PROMPT_SUMMARY_MAX_LENGTH = 100
const SESSION_ID_DISPLAY_LENGTH = 8

function truncatePrompt(prompt: string): string {
  if (prompt.length <= PROMPT_SUMMARY_MAX_LENGTH) {
    return prompt
  }
  return `${prompt.slice(0, PROMPT_SUMMARY_MAX_LENGTH)}…`
}

function shortId(id: string): string {
  return id.length <= SESSION_ID_DISPLAY_LENGTH ? id : id.slice(0, SESSION_ID_DISPLAY_LENGTH)
}

export interface SessionCardProps {
  session: SessionData
}

export default function SessionCard({ session }: SessionCardProps) {
  return (
    <Link
      to={`/sessions/${session.id}`}
      className="block rounded-md border border-gray-800 bg-gray-900 p-4 transition-colors hover:border-gray-700 hover:bg-gray-800/60"
    >
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 truncate text-sm text-gray-100" title={session.prompt}>
          {truncatePrompt(session.prompt)}
        </p>
        <StatusBadge status={session.status} />
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500">
        <span className="font-mono text-gray-400" title={session.id}>
          #{shortId(session.id)}
        </span>
        <span>{session.model}</span>
        <TimeAgo date={session.updated_at} />
        <span className="tabular-nums">{session.total_tokens.total_tokens.toLocaleString()} tokens</span>
        <span className="tabular-nums">${session.total_cost.toFixed(4)}</span>
      </div>
    </Link>
  )
}
