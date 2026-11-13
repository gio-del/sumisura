// oxlint-disable react/only-export-components -- test-support module: never
// imported by the app, so Fast Refresh does not apply to it.
import type { ReactElement } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import AppRoutes from '@/AppRoutes'
import { TooltipProvider } from '@/components/ui/tooltip'

// Pages read route params and search params, so they are rendered inside a
// real MemoryRouter at their real route rather than having routing stubbed —
// the URL a filter writes is part of the behaviour under test (ADR-0019).

function LocationProbe() {
  const location = useLocation()
  return (
    <>
      <span data-testid="location-pathname">{location.pathname}</span>
      <span data-testid="location-search">{location.search}</span>
    </>
  )
}

// HistoryBackProbe stands in for the browser's Back button: MemoryRouter
// keeps its own history stack, so going back is a navigate(-1) on it.
function HistoryBackProbe() {
  const navigate = useNavigate()
  return (
    <button type="button" data-testid="history-back" hidden onClick={() => navigate(-1)}>
      browser back
    </button>
  )
}

export interface RenderPageOptions {
  /** The route pattern the page is mounted at, e.g. '/jobs/:id/generate'. */
  pattern?: string
  /** The entry URL, e.g. '/jobs/acme/generate?status=sent'. */
  at?: string
}

export function renderPage(element: ReactElement, options: RenderPageOptions = {}) {
  const at = options.at ?? '/'
  const pattern = options.pattern ?? '/'
  // pointerEventsCheck is off because Radix's dialogs/selects put
  // `pointer-events: none` on the body while open, which jsdom reports
  // faithfully and user-event would otherwise refuse to click through.
  const user = userEvent.setup({ pointerEventsCheck: 0 })
  // TooltipProvider mirrors App.tsx: pages render Tooltips, and Radix needs
  // its provider above them exactly as the real app supplies it.
  const result = render(
    <TooltipProvider>
      <MemoryRouter initialEntries={[at]}>
        <LocationProbe />
        <Routes>
          <Route path={pattern} element={element} />
        </Routes>
      </MemoryRouter>
    </TooltipProvider>,
  )
  return { user, ...result }
}

/**
 * renderApp mounts the app's real route table (AppRoutes) at a URL, for
 * behaviour that crosses pages: a list row opening a Job Listing, a back
 * link returning to the list, route precedence between /jobs/new and
 * /jobs/:id (issue #94). Only the pages actually navigated to render, so a
 * test still declares just the endpoints those pages call.
 */
export function renderApp(options: { at: string }) {
  const user = userEvent.setup({ pointerEventsCheck: 0 })
  const result = render(
    <TooltipProvider>
      <MemoryRouter initialEntries={[options.at]}>
        <LocationProbe />
        <HistoryBackProbe />
        <AppRoutes />
      </MemoryRouter>
    </TooltipProvider>,
  )
  return { user, ...result }
}

/** currentPath is the pathname the app's own navigation has produced. */
export function currentPath(): string {
  return screen.getByTestId('location-pathname').textContent ?? ''
}

/** browserBack goes back one entry in the router's history, as Back would. */
export function browserBack(): void {
  fireEvent.click(screen.getByTestId('history-back'))
}

/** currentSearch is the query string the page's own navigation has produced. */
export function currentSearch(): string {
  return screen.getByTestId('location-search').textContent ?? ''
}

// openFilters expands the Job Listings filters panel (collapsed by default).
export async function openFilters() {
  const button = await screen.findByRole('button', { name: /^Filters/ })
  if (button.getAttribute('aria-expanded') !== 'true') await userEvent.click(button)
}
