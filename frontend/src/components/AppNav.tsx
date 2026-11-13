import { useEffect, useState, useSyncExternalStore } from 'react'
import { Briefcase, Database, Ellipsis, Search, Sparkles, type LucideIcon } from 'lucide-react'
import { NavLink, useLocation } from 'react-router-dom'
import { ForgetDeviceButton } from '@/components/AccessGate'
import UsageIndicator from '@/components/UsageIndicator'
import { cn } from '@/lib/utils'

type NavItem = { to: string; label: string; end?: boolean }

// The sidebar groups (computer). Every page of the app is listed exactly once.
const groups: { title: string; items: NavItem[] }[] = [
  {
    title: 'Job search',
    items: [
      { to: '/', label: 'Today', end: true },
      { to: '/jobs', label: 'Job Listings' },
      { to: '/inbox', label: 'To complete' },
      { to: '/applications', label: 'Applications' },
      { to: '/offers', label: 'Offers' },
      { to: '/ats', label: 'Browse ATS Boards' },
    ],
  },
  {
    title: 'My data',
    items: [
      { to: '/entries', label: 'Master Data', end: true },
      { to: '/profile', label: 'Profile' },
      { to: '/snippets', label: 'Cover Letter Snippets' },
      { to: '/import', label: 'Import' },
      { to: '/trash', label: 'Trash' },
    ],
  },
  {
    title: 'Tools',
    items: [
      { to: '/generate', label: 'Generate' },
      { to: '/generations', label: 'Generated CVs' },
      { to: '/gaps', label: 'Gaps' },
      { to: '/tags', label: 'Tag Lint' },
      { to: '/stats', label: 'Stats' },
    ],
  },
]

// The phone's bottom bar: four areas plus "More", which holds everything else.
const bottomItems: (NavItem & { icon: LucideIcon })[] = [
  { to: '/applications', label: 'Applications', icon: Briefcase },
  { to: '/jobs', label: 'Job Listings', icon: Search },
  { to: '/generate', label: 'Generate', icon: Sparkles },
  { to: '/entries', label: 'Master Data', icon: Database, end: true },
]
const bottomPaths = new Set(bottomItems.map((i) => i.to))
const moreItems: NavItem[] = groups.flatMap((g) => g.items).filter((i) => !bottomPaths.has(i.to))

const DESKTOP_QUERY = '(min-width: 768px)'

// Without matchMedia (tests, very old browsers) the sidebar is shown.
function useIsDesktop(): boolean {
  return useSyncExternalStore(
    (notify) => {
      if (typeof window.matchMedia !== 'function') return () => {}
      const mq = window.matchMedia(DESKTOP_QUERY)
      mq.addEventListener('change', notify)
      return () => mq.removeEventListener('change', notify)
    },
    () => (typeof window.matchMedia === 'function' ? window.matchMedia(DESKTOP_QUERY).matches : true),
    () => true,
  )
}

const linkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'flex min-h-11 items-center rounded-md px-3 font-medium text-muted-foreground no-underline hover:bg-accent hover:text-foreground',
    isActive && 'bg-accent text-primary',
  )

function Sidebar() {
  return (
    <nav
      className="sticky top-0 flex h-screen w-60 shrink-0 flex-col gap-4 overflow-y-auto border-r border-border bg-card px-3 py-4"
      aria-label="Primary"
    >
      <NavLink to="/" className="flex items-center px-3">
        {/* Two files rather than one: the wordmark is navy on light surfaces
            and chalk on dark ones (brand/palette.md). */}
        <img src="/logo-lockup.svg" alt="Sumisura" className="h-7 w-auto dark:hidden" />
        <img src="/logo-lockup-dark.svg" alt="" aria-hidden className="hidden h-7 w-auto dark:block" />
      </NavLink>
      {groups.map((group) => (
        <div key={group.title} className="flex flex-col gap-1">
          <h2 className="px-3 text-xs font-semibold tracking-wide text-muted-foreground uppercase">{group.title}</h2>
          {group.items.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={linkClass}>
              {item.label}
            </NavLink>
          ))}
        </div>
      ))}
      <div className="mt-auto flex flex-col gap-2 px-3">
        <UsageIndicator />
        <ForgetDeviceButton />
      </div>
    </nav>
  )
}

function BottomBar() {
  const { pathname } = useLocation()
  // The panel is open for the page it was opened on: navigating anywhere
  // closes it without an effect.
  const [openOn, setOpenOn] = useState<string | null>(null)
  const open = openOn === pathname
  const close = () => setOpenOn(null)

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpenOn(null)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  const moreActive = moreItems.some((i) => (i.end ? pathname === i.to : pathname.startsWith(i.to) && i.to !== '/'))
  const tab = (active: boolean) =>
    cn(
      'flex min-h-11 flex-1 flex-col items-center justify-center gap-0.5 text-xs font-medium text-muted-foreground no-underline',
      active && 'text-primary',
    )

  return (
    <>
      {open && <button type="button" aria-label="Close menu" className="fixed inset-0 z-30 cursor-default" onClick={close} />}
      <nav
        className="fixed inset-x-0 bottom-0 z-40 flex h-14 items-stretch border-t border-border bg-card pb-[env(safe-area-inset-bottom)] box-content"
        aria-label="Primary"
      >
        {bottomItems.map(({ to, label, icon: Icon, end }) => (
          <NavLink key={to} to={to} end={end} className={({ isActive }) => tab(isActive)}>
            <Icon aria-hidden className="size-5" />
            {label}
          </NavLink>
        ))}
        <button
          type="button"
          className={tab(moreActive || open)}
          aria-expanded={open}
          aria-controls="more-menu"
          onClick={() => setOpenOn(open ? null : pathname)}
        >
          <Ellipsis aria-hidden className="size-5" />
          More
        </button>
      </nav>
      {open && (
        <div
          id="more-menu"
          className="fixed inset-x-2 bottom-[calc(3.75rem+env(safe-area-inset-bottom))] z-40 max-h-[70vh] overflow-y-auto rounded-lg border border-border bg-popover p-2 text-popover-foreground shadow-md"
        >
          {moreItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={linkClass}>
              {item.label}
            </NavLink>
          ))}
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 px-3 py-2">
            <UsageIndicator />
            <ForgetDeviceButton />
          </div>
        </div>
      )}
    </>
  )
}

export default function AppNav() {
  return useIsDesktop() ? <Sidebar /> : <BottomBar />
}
