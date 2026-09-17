import { NavLink } from 'react-router-dom'
import { ForgetDeviceButton } from '@/components/AccessGate'
import UsageIndicator from '@/components/UsageIndicator'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/', label: 'Master Data', end: true },
  { to: '/tags', label: 'Tag Lint', end: false },
  { to: '/profile', label: 'Profile', end: false },
  { to: '/snippets', label: 'Cover Letter Snippets', end: false },
  { to: '/generate', label: 'Generate', end: false },
  { to: '/generations', label: 'Generated CVs', end: false },
  { to: '/jobs', label: 'Job Listings', end: false },
  { to: '/inbox', label: 'To complete', end: false },
  { to: '/applications', label: 'Applications', end: false },
  { to: '/ats', label: 'Browse ATS Boards', end: false },
  { to: '/stats', label: 'Stats', end: false },
]

export default function AppNav() {
  return (
    <nav
      className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-border bg-card px-6 py-4"
      aria-label="Primary"
    >
      <NavLink to="/" className="mr-auto flex items-center">
        {/* Two files rather than one: the wordmark is navy on light surfaces
            and chalk on dark ones (brand/palette.md). */}
        <img src="/logo-lockup.svg" alt="Sumisura" className="h-7 w-auto dark:hidden" />
        <img src="/logo-lockup-dark.svg" alt="" aria-hidden className="hidden h-7 w-auto dark:block" />
      </NavLink>
      {navItems.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive }) =>
            cn(
              'border-b-2 border-transparent py-1 font-medium text-muted-foreground no-underline hover:text-foreground',
              isActive && 'border-primary text-primary',
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
      <UsageIndicator />
      <ForgetDeviceButton />
    </nav>
  )
}
