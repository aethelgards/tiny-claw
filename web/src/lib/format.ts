const MS_PER_SECOND = 1000
const MS_PER_MINUTE = 60 * 1000

export function formatDuration(ms: number): string {
  if (ms < MS_PER_SECOND) {
    return `${ms} ms`
  }
  if (ms < MS_PER_MINUTE) {
    return `${(ms / MS_PER_SECOND).toFixed(ms < 10 * MS_PER_SECOND ? 2 : 1)} s`
  }
  const minutes = Math.floor(ms / MS_PER_MINUTE)
  const seconds = Math.round((ms % MS_PER_MINUTE) / MS_PER_SECOND)
  return `${minutes}m ${seconds}s`
}

export function formatClock(timestamp: string): string {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return timestamp
  }
  return date.toLocaleTimeString('zh-CN', { hour12: false })
}

export function formatTokens(count: number): string {
  return count.toLocaleString()
}
