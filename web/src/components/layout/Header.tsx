import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { NAV_ITEMS } from './Sidebar'

function pageTitle(pathname: string): string {
  const match = NAV_ITEMS.find(
    (item) =>
      item.path === pathname ||
      (item.path !== '/' && pathname.startsWith(item.path)),
  )
  return match?.label ?? 'Dashboard'
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

export default function Header() {
  const location = useLocation()
  const [now, setNow] = useState(() => new Date())

  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  return (
    <header className="flex h-16 shrink-0 items-center justify-between border-b border-gray-800 bg-gray-900 px-6">
      <h1 className="text-base font-semibold text-gray-100">
        {pageTitle(location.pathname)}
      </h1>
      <time
        dateTime={now.toISOString()}
        className="text-sm tabular-nums text-gray-400"
      >
        {formatTime(now)}
      </time>
    </header>
  )
}
