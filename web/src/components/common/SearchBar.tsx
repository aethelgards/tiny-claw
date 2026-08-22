import { useEffect, useRef, useState, type ChangeEvent } from 'react'

const DEBOUNCE_MS = 300

export interface SearchBarProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export default function SearchBar({ value, onChange, placeholder }: SearchBarProps) {
  const [inputValue, setInputValue] = useState(value)
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const latestValueRef = useRef(value)
  latestValueRef.current = value

  // Keep the visible input in sync when the parent resets or clears the value.
  useEffect(() => {
    setInputValue(value)
  }, [value])

  // Never fire a stale onChange after unmount.
  useEffect(
    () => () => {
      clearTimeout(timerRef.current)
    },
    [],
  )

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const next = event.target.value
    setInputValue(next)

    clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      // Skip when the parent already echoed this value back via the value prop.
      if (next !== latestValueRef.current) {
        onChange(next)
      }
    }, DEBOUNCE_MS)
  }

  return (
    <div className="relative">
      <svg
        className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.35-4.35" />
      </svg>
      <input
        type="search"
        value={inputValue}
        onChange={handleChange}
        placeholder={placeholder}
        aria-label={placeholder}
        className="w-full rounded-md border border-gray-800 bg-gray-900 py-2 pl-9 pr-3 text-sm text-gray-100 transition-colors placeholder:text-gray-500 hover:border-gray-700 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
    </div>
  )
}
