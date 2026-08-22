import { useEffect, useState } from 'react'

const REFRESH_INTERVAL_MS = 30_000

function formatTimeAgo(date: Date, now: Date): string {
  const diffMs = now.getTime() - date.getTime()
  if (diffMs < 0) {
    return '刚刚'
  }

  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) {
    return '刚刚'
  }

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) {
    return `${minutes} 分钟前`
  }

  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours} 小时前`
  }

  const days = Math.floor(hours / 24)
  if (days < 30) {
    return `${days} 天前`
  }

  return date.toLocaleString()
}

export function timeAgo(date: string, now: Date = new Date()): string {
  const parsed = new Date(date)
  if (Number.isNaN(parsed.getTime())) {
    return date
  }
  return formatTimeAgo(parsed, now)
}

export interface TimeAgoProps {
  date: string
}

export default function TimeAgo({ date }: TimeAgoProps) {
  const [now, setNow] = useState(() => new Date())

  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), REFRESH_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [])

  return (
    <time dateTime={date} className="text-sm text-gray-400" title={date}>
      {timeAgo(date, now)}
    </time>
  )
}
