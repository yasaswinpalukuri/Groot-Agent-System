import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Overview   from './pages/Overview'
import Chat       from './pages/Chat'
import Approvals  from './pages/Approvals'
import JobSearch  from './pages/JobSearch'
import DailyLog from './pages/DailyLog'
import Operations from './pages/Operations'
import Developer  from './pages/Developer'
import Monitoring from './pages/Monitoring'
import RAGStore   from './pages/RAGStore'
import Security   from './pages/Security'
import AgentGroot    from './pages/AgentGroot'
import AgentEinstein from './pages/AgentEinstein'
import AgentSiva     from './pages/AgentSiva'
import TaskQueue    from './pages/TaskQueue'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/overview" replace />} />
          <Route path="overview"       element={<Overview />} />
          <Route path="chat"           element={<Chat />} />
          <Route path="approvals"      element={<Approvals />} />
          <Route path="jobs"           element={<JobSearch />} />
          <Route path="dailylog"          element={<DailyLog />} />
          <Route path="operations"     element={<Operations />} />
          <Route path="developer"      element={<Developer />} />
          <Route path="monitoring"     element={<Monitoring />} />
          <Route path="rag"            element={<RAGStore />} />
          <Route path="security"       element={<Security />} />
          <Route path="agent/groot"    element={<AgentGroot />} />
          <Route path="agent/einstein" element={<AgentEinstein />} />
          <Route path="agent/siva"     element={<AgentSiva />} />
          <Route path="queue"           element={<TaskQueue />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
