export type SessionStatus = 'running' | 'completed' | 'failed'

interface StatusStyle {
  badge: string
  dot: string
}

const STATUS_STYLES: Record<string, StatusStyle> = {
  running: {
    badge: 'bg-blue-500/10 text-blue-400 ring-blue-500/30',
    dot: 'bg-blue-400 animate-pulse',
  },
  completed: {
    badge: 'bg-green-500/10 text-green-400 ring-green-500/30',
    dot: 'bg-green-400',
  },
  failed: {
    badge: 'bg-red-500/10 text-red-400 ring-red-500/30',
    dot: 'bg-red-400',
  },
}

const NEUTRAL_STYLE: StatusStyle = {
  badge: 'bg-gray-500/10 text-gray-400 ring-gray-500/30',
  dot: 'bg-gray-400',
}

function styleFor(status: string): StatusStyle {
  return STATUS_STYLES[status] ?? NEUTRAL_STYLE
}

export interface StatusBadgeProps {
  status: string
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  const { badge, dot } = styleFor(status)
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${badge}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${dot}`} aria-hidden="true" />
      {status}
    </span>
  )
}
