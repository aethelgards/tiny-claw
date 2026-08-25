import { BrowserRouter, Route, Routes } from 'react-router-dom'
import Layout from './components/layout/Layout'
import Dashboard from './pages/Dashboard'
import Sessions from './pages/Sessions'
import SessionDetail from './pages/SessionDetail'
import CostAnalytics from './pages/analytics/CostAnalytics'
import PerformanceAnalytics from './pages/analytics/PerformanceAnalytics'
import ToolAnalytics from './pages/analytics/ToolAnalytics'

function Placeholder() {
  return (
    <div className="flex h-full items-center justify-center">
      <p className="text-sm text-gray-500">页面开发中</p>
    </div>
  )
}

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="sessions" element={<Sessions />} />
          <Route path="sessions/:id" element={<SessionDetail />} />
          <Route path="analytics/cost" element={<CostAnalytics />} />
          <Route path="analytics/performance" element={<PerformanceAnalytics />} />
          <Route path="analytics/tools" element={<ToolAnalytics />} />
          <Route path="*" element={<Placeholder />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
