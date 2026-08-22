import { NavLink } from 'react-router-dom'

export interface NavItem {
  label: string
  path: string
}

export const NAV_ITEMS: NavItem[] = [
  { label: 'Dashboard', path: '/' },
  { label: 'Sessions', path: '/sessions' },
  { label: 'Cost Analysis', path: '/analytics/cost' },
  { label: 'Performance', path: '/analytics/performance' },
  { label: 'Tool Stats', path: '/analytics/tools' },
]

function linkClassName({ isActive }: { isActive: boolean }): string {
  const base =
    'block rounded-md px-3 py-2 text-sm font-medium transition-colors'
  return isActive
    ? `${base} bg-gray-800 text-white`
    : `${base} text-gray-400 hover:bg-gray-800/60 hover:text-gray-100`
}

export default function Sidebar() {
  return (
    <aside className="flex w-64 shrink-0 flex-col bg-gray-900">
      <div className="flex h-16 items-center gap-2 border-b border-gray-800 px-4">
        <span className="text-xl" aria-hidden="true">
          🦞
        </span>
        <span className="text-lg font-bold tracking-tight text-gray-100">
          tiny-claw
        </span>
      </div>
      <nav className="flex-1 space-y-1 overflow-y-auto p-3" aria-label="Main">
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            end={item.path === '/'}
            className={linkClassName}
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
