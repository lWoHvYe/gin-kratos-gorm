export const tokens = {
  color: {
    primary: '#2563eb',
    success: '#16a34a',
    warning: '#d97706',
    danger: '#dc2626',
    text: '#1f2937',
    muted: '#6b7280',
    border: '#e5e7eb',
    background: '#f6f8fb',
  },
  radius: { sm: '4px', md: '8px', lg: '12px' },
  space: { xs: '4px', sm: '8px', md: '12px', lg: '16px', xl: '24px' },
  font: { family: "'Inter', 'PingFang SC', 'Microsoft YaHei', sans-serif" },
} as const

export function applyDesignTokens(): void {
  const root = document.documentElement
  root.style.setProperty('--mall-primary', tokens.color.primary)
  root.style.setProperty('--mall-text', tokens.color.text)
  root.style.setProperty('--mall-muted', tokens.color.muted)
  root.style.setProperty('--mall-bg', tokens.color.background)
  root.style.setProperty('--mall-border', tokens.color.border)
  root.style.setProperty('--mall-radius', tokens.radius.md)
}
