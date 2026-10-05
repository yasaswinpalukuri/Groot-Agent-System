import { Outlet } from 'react-router-dom'
import DemoBanner from './DemoBanner'
import TopBar from './TopBar'
import Sidebar from './Sidebar'

export default function Layout() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', overflow: 'hidden' }}>
      <DemoBanner />
      <TopBar />
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        <Sidebar />
        <main style={{ flex: 1, overflowY: 'auto', background: '#0D1117', padding: '24px' }}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
