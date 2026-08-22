import { Link, useNavigate } from 'react-router-dom'
import StatusBadge from '../common/StatusBadge'
import TimeAgo from '../common/TimeAgo'
import type { SessionData } from '../../api/types'

const SESSION_ID_DISPLAY_LENGTH = 8
const PROMPT_MAX_LENGTH = 60

const TH_CLASS = 'px-4 py-3 text-left text-xs font-medium text-gray-500'
const TD_CLASS = 'px-4 py-3 align-middle'

function shortId(id: string): string {
  return id.length <= SESSION_ID_DISPLAY_LENGTH ? id : id.slice(0, SESSION_ID_DISPLAY_LENGTH)
}

function truncatePrompt(prompt: string): string {
  if (prompt.length <= PROMPT_MAX_LENGTH) {
    return prompt
  }
  return `${prompt.slice(0, PROMPT_MAX_LENGTH)}…`
}

function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(1)}M`
  }
  if (tokens >= 1_000) {
    return `${(tokens / 1_000).toFixed(1)}K`
  }
  return tokens.toLocaleString()
}

export interface SessionListProps {
  sessions: SessionData[]
}

function SessionRow({ session }: { session: SessionData }) {
  const navigate = useNavigate()

  return (
    <tr
      className="cursor-pointer border-b border-gray-800 transition-colors last:border-b-0 hover:bg-gray-800/60"
      onClick={() => navigate(`/sessions/${session.id}`)}
    >
      <td className={`${TD_CLASS} whitespace-nowrap`}>
        <Link
          to={`/sessions/${session.id}`}
          className="font-mono text-gray-400 transition-colors hover:text-blue-400"
          title={session.id}
        >
          #{shortId(session.id)}
        </Link>
      </td>
      <td className={TD_CLASS}>
        <StatusBadge status={session.status} />
      </td>
      <td className={`${TD_CLASS} whitespace-nowrap text-gray-300`}>{session.model}</td>
      <td className={`${TD_CLASS} max-w-[280px]`}>
        <p className="truncate text-gray-100" title={session.prompt}>
          {truncatePrompt(session.prompt)}
        </p>
      </td>
      <td className={`${TD_CLASS} whitespace-nowrap`}>
        <TimeAgo date={session.updated_at} />
      </td>
      <td className={`${TD_CLASS} text-right tabular-nums text-gray-300`}>
        {formatTokens(session.total_tokens.total_tokens)}
      </td>
      <td className={`${TD_CLASS} text-right tabular-nums text-gray-300`}>
        ${session.total_cost.toFixed(4)}
      </td>
    </tr>
  )
}

export default function SessionList({ sessions }: SessionListProps) {
  return (
    <div className="overflow-x-auto rounded-md border border-gray-800 bg-gray-900">
      <table className="w-full min-w-[768px] text-sm">
        <thead>
          <tr className="border-b border-gray-800">
            <th scope="col" className={TH_CLASS}>ID</th>
            <th scope="col" className={TH_CLASS}>Status</th>
            <th scope="col" className={TH_CLASS}>Model</th>
            <th scope="col" className={TH_CLASS}>Prompt</th>
            <th scope="col" className={TH_CLASS}>Time</th>
            <th scope="col" className={`${TH_CLASS} text-right`}>Tokens</th>
            <th scope="col" className={`${TH_CLASS} text-right`}>Cost</th>
          </tr>
        </thead>
        <tbody>
          {sessions.map((session) => (
            <SessionRow key={session.id} session={session} />
          ))}
        </tbody>
      </table>
    </div>
  )
}
