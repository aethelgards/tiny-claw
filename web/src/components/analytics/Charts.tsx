import {
  ResponsiveContainer,
  LineChart as RLineChart,
  Line,
  BarChart as RBarChart,
  Bar,
  PieChart as RPieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from 'recharts'
import type { NameType, ValueType } from 'recharts/types/component/DefaultTooltipContent'

// ── Design tokens (consistent with the existing dark theme) ──────────

const AXIS_STYLE = { fontSize: 12, fill: '#9ca3af' } // gray-400
const GRID_STROKE = '#374151' // gray-700
const TOOLTIP_BG = '#111827' // gray-900
const TOOLTIP_BORDER = '#1f2937' // gray-800

export const CHART_COLORS = [
  '#3b82f6', // blue-500
  '#10b981', // emerald-500
  '#f59e0b', // amber-500
  '#8b5cf6', // violet-500
  '#ef4444', // red-500
  '#06b6d4', // cyan-500
  '#ec4899', // pink-500
  '#14b8a6', // teal-500
]

const CARD_CLASS =
  'rounded-md border border-gray-800 bg-gray-900 p-5'

// ── Shared tooltip ───────────────────────────────────────────────────

interface ChartTooltipProps {
  formatter?: (value: ValueType, name: NameType) => [string, string]
  labelFormatter?: (label: string) => string
}

function ChartTooltip({ formatter, labelFormatter }: ChartTooltipProps) {
  return (
    <Tooltip
      contentStyle={{
        backgroundColor: TOOLTIP_BG,
        border: `1px solid ${TOOLTIP_BORDER}`,
        borderRadius: 6,
        fontSize: 13,
        color: '#f3f4f6', // gray-100
      }}
      itemStyle={{ color: '#d1d5db' }} // gray-300
      formatter={formatter}
      labelFormatter={labelFormatter}
    />
  )
}

// ── Chart card wrapper ───────────────────────────────────────────────

interface ChartCardProps {
  title: string
  children: React.ReactNode
  className?: string
}

export function ChartCard({ title, children, className = '' }: ChartCardProps) {
  return (
    <div className={`${CARD_CLASS} ${className}`}>
      <h3 className="mb-4 text-sm font-medium text-gray-300">{title}</h3>
      {children}
    </div>
  )
}

// ── LineChart ────────────────────────────────────────────────────────

export interface LineSeries {
  dataKey: string
  name: string
  color?: string
  strokeWidth?: number
  dot?: boolean
}

interface AppLineChartProps {
  data: unknown[]
  xKey: string
  series: LineSeries[]
  height?: number
  tooltipFormatter?: (value: ValueType, name: NameType) => [string, string]
  tooltipLabelFormatter?: (label: string) => string
  yAxisFormatter?: (value: number) => string
}

export function AppLineChart({
  data,
  xKey,
  series,
  height = 280,
  tooltipFormatter,
  tooltipLabelFormatter,
  yAxisFormatter,
}: AppLineChartProps) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <RLineChart data={data} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_STROKE} />
        <XAxis dataKey={xKey} tick={AXIS_STYLE} tickLine={false} axisLine={false} />
        <YAxis tick={AXIS_STYLE} tickLine={false} axisLine={false} tickFormatter={yAxisFormatter} />
        <ChartTooltip formatter={tooltipFormatter} labelFormatter={tooltipLabelFormatter} />
        {series.map((s) => (
          <Line
            key={s.dataKey}
            type="monotone"
            dataKey={s.dataKey}
            name={s.name}
            stroke={s.color ?? CHART_COLORS[series.indexOf(s) % CHART_COLORS.length]}
            strokeWidth={s.strokeWidth ?? 2}
            dot={s.dot ?? false}
            activeDot={{ r: 4 }}
          />
        ))}
      </RLineChart>
    </ResponsiveContainer>
  )
}

// ── BarChart ─────────────────────────────────────────────────────────

export interface BarSeries {
  dataKey: string
  name: string
  color?: string
}

interface AppBarChartProps {
  data: unknown[]
  xKey: string
  series: BarSeries[]
  height?: number
  tooltipFormatter?: (value: ValueType, name: NameType) => [string, string]
  tooltipLabelFormatter?: (label: string) => string
  yAxisFormatter?: (value: number) => string
}

export function AppBarChart({
  data,
  xKey,
  series,
  height = 280,
  tooltipFormatter,
  tooltipLabelFormatter,
  yAxisFormatter,
}: AppBarChartProps) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <RBarChart data={data} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_STROKE} />
        <XAxis dataKey={xKey} tick={AXIS_STYLE} tickLine={false} axisLine={false} />
        <YAxis tick={AXIS_STYLE} tickLine={false} axisLine={false} tickFormatter={yAxisFormatter} />
        <ChartTooltip formatter={tooltipFormatter} labelFormatter={tooltipLabelFormatter} />
        {series.map((s) => (
          <Bar
            key={s.dataKey}
            dataKey={s.dataKey}
            name={s.name}
            fill={s.color ?? CHART_COLORS[series.indexOf(s) % CHART_COLORS.length]}
            radius={[4, 4, 0, 0]}
          />
        ))}
      </RBarChart>
    </ResponsiveContainer>
  )
}

// ── PieChart ─────────────────────────────────────────────────────────

interface AppPieChartProps {
  data: { name: string; value: number }[]
  height?: number
  innerRadius?: number
  outerRadius?: number
  tooltipFormatter?: (value: ValueType, name: NameType) => [string, string]
}

export function AppPieChart({
  data,
  height = 280,
  innerRadius = 60,
  outerRadius = 100,
  tooltipFormatter,
}: AppPieChartProps) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <RPieChart>
        <Pie
          data={data}
          cx="50%"
          cy="50%"
          innerRadius={innerRadius}
          outerRadius={outerRadius}
          paddingAngle={2}
          dataKey="value"
        >
          {data.map((_, index) => (
            <Cell key={index} fill={CHART_COLORS[index % CHART_COLORS.length]} />
          ))}
        </Pie>
        <ChartTooltip formatter={tooltipFormatter} />
        <Legend
          wrapperStyle={{ fontSize: 12, color: '#9ca3af' }}
          formatter={(value: string) => (
            <span style={{ color: '#d1d5db' }}>{value}</span>
          )}
        />
      </RPieChart>
    </ResponsiveContainer>
  )
}
