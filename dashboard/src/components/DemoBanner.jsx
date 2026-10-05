export default function DemoBanner() {
  return (
    <div style={{
      background: '#2D2208',
      borderBottom: '1px solid #D29922',
      height: '28px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: '16px',
    }}>
      <span style={{ color: '#D29922', fontSize: '11px', fontFamily: 'Inter, sans-serif' }}>
        Demo build · Lenovo M720Q · 6 of 8 agents · Production Mini PC coming soon
      </span>
      <span style={{ color: '#D29922', fontSize: '11px', fontFamily: 'JetBrains Mono, monospace' }}>
        ●●●○○
      </span>
    </div>
  )
}
