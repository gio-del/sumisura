import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router-dom'
import ApplicationStatusBadges from '@/components/ApplicationStatusBadges'
import ApplicationStatusControl from '@/components/ApplicationStatusControl'
import ConflictAlert from '@/components/ConflictAlert'
import FreshnessBadge from '@/components/FreshnessBadge'
import PriorityControl, { PRIORITIES, PRIORITY_LABELS } from '@/components/PriorityControl'
import RALBadge from '@/components/RALBadge'
import {
  correctJobListing,
  exportDataUrl,
  isConflict,
  jobListingLogoUrl,
  listJobListings,
  setJobListingArchived,
  updateApplicationStatus,
} from '@/api/client'
import type {
  ApplicationStatus,
  ArchivedView,
  CloseReason,
  JobListingSummaryWithApplication,
  Priority,
} from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { statusLabel } from '@/lib/applicationStatus'
import { jobListingHeading } from '@/lib/utils'
import type { JobListingDetailLocationState } from '@/pages/JobListingDetailPage'

const ALL_STATUSES_VALUE = 'all'
const ALL_PRIORITIES_VALUE = 'all'
const DEFAULT_STALE_DAYS = 30
const DAY_MS = 24 * 60 * 60 * 1000

// Only a Job Listing still active with its Application at Saved can be
// archived in bulk (issue #303): started applications are archived one by one.
function isBulkSelectable(row: JobListingSummaryWithApplication): boolean {
  return !row.jobListing.archived && row.application.status === 'saved'
}

type RALSortChoice = 'none' | 'asc' | 'desc'

const RAL_SORT_CHOICES: RALSortChoice[] = ['none', 'asc', 'desc']

// The archived view (issue #98) as the "Show" control offers it. Active is
// the backend's exclude default, so it never appears in the URL.
const ARCHIVED_VIEW_LABELS: Record<ArchivedView, string> = {
  exclude: 'Active',
  only: 'Archived',
  all: 'All',
}
const ARCHIVED_VIEWS = Object.keys(ARCHIVED_VIEW_LABELS) as ArchivedView[]

export default function JobListingsListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const location = useLocation()
  const [listings, setListings] = useState<JobListingSummaryWithApplication[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [statusError, setStatusError] = useState<string | null>(null)
  const [updatingId, setUpdatingId] = useState<string | null>(null)
  const [archiveError, setArchiveError] = useState<string | null>(null)
  const [archivingId, setArchivingId] = useState<string | null>(null)
  const [priorityError, setPriorityError] = useState<string | null>(null)
  const [prioritizingId, setPrioritizingId] = useState<string | null>(null)
  // conflict is a Status change or archive toggle the backend refused with
  // 409 because the Application or Job Listing changed on disk since this
  // list was loaded (issue #89).
  const [conflict, setConflict] = useState<{ record: string; action: 'change' } | null>(null)
  const [reloading, setReloading] = useState(false)
  // Bulk archive (issue #303): the selected Job Listing ids, the days
  // threshold of the quick "Saved for over N days" selection and the outcome.
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [staleDays, setStaleDays] = useState(String(DEFAULT_STALE_DAYS))
  const [bulkArchiving, setBulkArchiving] = useState(false)
  const [bulkResult, setBulkResult] = useState<string | null>(null)

  // The archived view lives in the URL like the filters below (issue #98,
  // story 5), but is not one of them: Clear filters leaves it in place, since
  // it picks which set of Job Listings the filters search within.
  const archivedParam = searchParams.get('archived') as ArchivedView | null
  const archivedView: ArchivedView =
    archivedParam && ARCHIVED_VIEWS.includes(archivedParam) ? archivedParam : 'exclude'

  function setArchivedView(view: ArchivedView) {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (view === 'exclude') {
        next.delete('archived')
      } else {
        next.set('archived', view)
      }
      return next
    })
  }

  // Filter state lives in the URL (story 7: bookmarkable/reload-safe views).
  const statusFilter = (searchParams.get('status') as ApplicationStatus | null) ?? ''
  const companyFilter = searchParams.get('company') ?? ''
  const locationFilter = searchParams.get('location') ?? ''
  const queryFilter = searchParams.get('q') ?? ''
  const savedFromFilter = searchParams.get('savedFrom') ?? ''
  const savedToFilter = searchParams.get('savedTo') ?? ''
  // Priority (issue #280): the filter and the "high first" order live in the
  // URL with the other filters. The order is a view choice, not a filter, so
  // Clear filters leaves it alone.
  const priorityParam = searchParams.get('priority') as Priority | null
  const priorityFilter: Priority | '' = priorityParam && PRIORITIES.includes(priorityParam) ? priorityParam : ''
  const prioritySort = searchParams.get('prioritySort') === '1'
  const hasActiveFilter = Boolean(priorityFilter || statusFilter || companyFilter || queryFilter || savedFromFilter || savedToFilter)

  function setFilter(key: 'status' | 'priority' | 'company' | 'location' | 'q' | 'savedFrom' | 'savedTo', value: string) {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (value) {
        next.set(key, value)
      } else {
        next.delete(key)
      }
      return next
    })
  }

  // The search box keeps its own text and writes it to the URL after a
  // pause, so typing does not fire a request per keystroke (issue #245).
  const [searchInput, setSearchInput] = useState(queryFilter)
  useEffect(() => {
    setSearchInput(queryFilter)
  }, [queryFilter])
  useEffect(() => {
    if (searchInput === queryFilter) return
    const timer = setTimeout(() => setFilter('q', searchInput), 300)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchInput])

  function clearFilters() {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      for (const key of ['status', 'priority', 'company', 'location', 'q', 'savedFrom', 'savedTo']) next.delete(key)
      return next
    })
  }

  // RAL Range sort/filter (issue #51): the applied sort and bounds live in
  // the URL alongside the filters above, so a detour into one Job Listing's
  // page and back — by its back link or by browser Back — restores the view
  // as it was (issue #94, stories 5-6). Only *applied* bounds are written
  // there: the Min/Max inputs stay local until "Apply RAL filter", so typing
  // never fires a request per keystroke. A bound left blank is absent from
  // the URL and falls back to 0 / no ceiling when the request is built.
  // knownCurrencies is (re)derived only from an *unfiltered* fetch, so it
  // keeps reflecting every currency actually in use even while a
  // currency-scoped filter is active (story 8, currency-handling decision).
  const ralSortParam = searchParams.get('ralSort') as RALSortChoice | null
  const ralSort: RALSortChoice = ralSortParam && RAL_SORT_CHOICES.includes(ralSortParam) ? ralSortParam : 'none'
  const appliedRALMin = searchParams.get('ralMin') ?? ''
  const appliedRALMax = searchParams.get('ralMax') ?? ''
  const appliedRALCurrency = searchParams.get('ralCurrency') ?? ''
  const hasAppliedRALFilter = Boolean(appliedRALMin || appliedRALMax)
  const [ralMinInput, setRalMinInput] = useState(appliedRALMin)
  const [ralMaxInput, setRalMaxInput] = useState(appliedRALMax)
  const [ralCurrency, setRalCurrency] = useState(appliedRALCurrency || 'EUR')
  const [knownCurrencies, setKnownCurrencies] = useState<string[]>([])
  const [ralFilterError, setRalFilterError] = useState<string | null>(null)

  function setRalSort(choice: RALSortChoice) {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (choice === 'none') {
        next.delete('ralSort')
      } else {
        next.set('ralSort', choice)
      }
      return next
    })
  }

  function setPrioritySort(on: boolean) {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (on) {
        next.set('prioritySort', '1')
      } else {
        next.delete('prioritySort')
      }
      return next
    })
  }

  // listQuery is the request the current URL describes, shared by the
  // fetch below and the reload after a conflict.
  const listQuery = useMemo(
    () => ({
      status: statusFilter || undefined,
      company: companyFilter || undefined,
      location: locationFilter || undefined,
      q: queryFilter.trim() || undefined,
      savedFrom: savedFromFilter || undefined,
      savedTo: savedToFilter || undefined,
      archived: archivedView,
      priority: priorityFilter || undefined,
      sortByPriority: prioritySort || undefined,
      sortByRAL: ralSort === 'none' ? undefined : ralSort,
      ralMin: hasAppliedRALFilter ? Number(appliedRALMin || 0) : undefined,
      ralMax: hasAppliedRALFilter ? (appliedRALMax === '' ? Number.MAX_SAFE_INTEGER : Number(appliedRALMax)) : undefined,
      ralCurrency: hasAppliedRALFilter ? appliedRALCurrency || 'EUR' : undefined,
    }),
    [
      statusFilter,
      companyFilter,
      locationFilter,
      queryFilter,
      savedFromFilter,
      savedToFilter,
      archivedView,
      priorityFilter,
      prioritySort,
      ralSort,
      hasAppliedRALFilter,
      appliedRALMin,
      appliedRALMax,
      appliedRALCurrency,
    ],
  )

  // handleReloadAfterConflict re-reads the list with the current filters —
  // the explicit "reload the current version" action after a conflict, which
  // refreshes every row's content and version tokens (issue #89).
  async function handleReloadAfterConflict() {
    setReloading(true)
    try {
      setListings((await listJobListings(listQuery)) ?? [])
      setConflict(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setReloading(false)
    }
  }

  useEffect(() => {
    listJobListings(listQuery)
      .then((l) => {
        setListings(l ?? [])
        if (!hasAppliedRALFilter) {
          const currencies = Array.from(
            new Set(
              (l ?? [])
                .map((x) => x.jobListing.ral)
                .filter((ral) => ral.source === 'stated' || ral.source === 'estimated')
                .map((ral) => ral.currency)
                .filter((currency): currency is string => Boolean(currency)),
            ),
          ).sort()
          setKnownCurrencies(currencies)
          if (currencies.length === 1) setRalCurrency(currencies[0])
        }
      })
      .catch((e) => setError(e.message))
  }, [listQuery, hasAppliedRALFilter])

  function handleApplyRALFilter() {
    setRalFilterError(null)
    const min = ralMinInput.trim() === '' ? 0 : Number(ralMinInput)
    const max = ralMaxInput.trim() === '' ? Number.MAX_SAFE_INTEGER : Number(ralMaxInput)
    if (Number.isNaN(min) || Number.isNaN(max)) {
      setRalFilterError('Min/Max RAL must be numbers.')
      return
    }
    if (min > max) {
      setRalFilterError('Min RAL must not exceed Max RAL.')
      return
    }
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('ralMin', String(min))
      if (ralMaxInput.trim() === '') {
        next.delete('ralMax')
      } else {
        next.set('ralMax', String(max))
      }
      next.set('ralCurrency', ralCurrency || knownCurrencies[0] || 'EUR')
      return next
    })
  }

  function handleClearRALFilter() {
    setRalMinInput('')
    setRalMaxInput('')
    setRalFilterError(null)
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      for (const key of ['ralMin', 'ralMax', 'ralCurrency']) next.delete(key)
      return next
    })
  }

  // findListing returns the row as last read, whose version tokens every
  // write on it presents (issue #89).
  function findListing(jobListingId: string): JobListingSummaryWithApplication | undefined {
    return listings?.find((l) => l.jobListing.id === jobListingId)
  }

  async function handleStatusChange(jobListingId: string, status: ApplicationStatus, closeReason?: CloseReason) {
    setStatusError(null)
    setConflict(null)
    setUpdatingId(jobListingId)
    try {
      const application = await updateApplicationStatus(jobListingId, status, findListing(jobListingId)?.application.version, closeReason)
      setListings((prev) =>
        prev ? prev.map((l) => (l.jobListing.id === jobListingId ? { ...l, application } : l)) : prev,
      )
    } catch (err) {
      if (isConflict(err)) {
        setConflict({ record: 'Application', action: 'change' })
      } else {
        setStatusError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      setUpdatingId(null)
    }
  }

  // No confirmation: archiving loses nothing and is undone in one click
  // (issue #98, story 7). A row that no longer belongs in the current view
  // leaves it; in the All view it stays and just flips.
  async function handleArchiveToggle(jobListingId: string, archived: boolean) {
    setArchiveError(null)
    setConflict(null)
    setArchivingId(jobListingId)
    try {
      const updated = await setJobListingArchived(jobListingId, archived, findListing(jobListingId)?.jobListing.version)
      setListings((prev) => {
        if (!prev) return prev
        if (archivedView === 'all') {
          return prev.map((l) =>
            l.jobListing.id === jobListingId
              ? { ...l, jobListing: { ...l.jobListing, archived: updated.archived, version: updated.version } }
              : l,
          )
        }
        const keepArchived = archivedView === 'only'
        return prev.filter((l) => l.jobListing.id !== jobListingId || updated.archived === keepArchived)
      })
    } catch (err) {
      if (isConflict(err)) {
        setConflict({ record: 'Job Listing', action: 'change' })
      } else {
        setArchiveError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      setArchivingId(null)
    }
  }

  // Priority is a personal annotation: it is written alone, with the Job
  // Listing's version token, and leaves the row where it is until the list
  // is next fetched (issue #280).
  async function handlePriorityChange(jobListingId: string, priority: Priority | '') {
    setPriorityError(null)
    setConflict(null)
    setPrioritizingId(jobListingId)
    try {
      const updated = await correctJobListing(jobListingId, { priority }, findListing(jobListingId)?.jobListing.version)
      setListings((prev) =>
        prev
          ? prev.map((l) =>
              l.jobListing.id === jobListingId
                ? { ...l, jobListing: { ...l.jobListing, priority: updated.priority, version: updated.version } }
                : l,
            )
          : prev,
      )
    } catch (err) {
      if (isConflict(err)) {
        setConflict({ record: 'Job Listing', action: 'change' })
      } else {
        setPriorityError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      setPrioritizingId(null)
    }
  }

  function toggleSelected(jobListingId: string, on: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (on) next.add(jobListingId)
      else next.delete(jobListingId)
      return next
    })
  }

  // Selects every Saved row whose Status has not moved for more than N days
  // (statusUpdatedAt, falling back to savedAt for a record that has none).
  function handleSelectStale() {
    const days = Number(staleDays)
    if (!Number.isFinite(days) || days < 0) return
    const cutoff = Date.now() - days * DAY_MS
    const ids = (listings ?? [])
      .filter(isBulkSelectable)
      .filter((l) => new Date(l.application.statusUpdatedAt ?? l.jobListing.savedAt).getTime() < cutoff)
      .map((l) => l.jobListing.id)
    setBulkResult(ids.length === 0 ? `No Saved Job Listings older than ${days} days.` : null)
    setSelected(new Set(ids))
  }

  // Archives the selected rows one after the other, each with its own
  // version token: a row refused with 409 or failing does not stop the rest,
  // and stays selected.
  async function handleBulkArchive() {
    const ids = (listings ?? []).filter((l) => selected.has(l.jobListing.id) && isBulkSelectable(l)).map((l) => l.jobListing.id)
    if (ids.length === 0) return
    setArchiveError(null)
    setConflict(null)
    setBulkResult(null)
    setBulkArchiving(true)
    const done: string[] = []
    let conflicts = 0
    let failures = 0
    let lastFailure = ''
    for (const id of ids) {
      try {
        await setJobListingArchived(id, true, findListing(id)?.jobListing.version)
        done.push(id)
      } catch (err) {
        if (isConflict(err)) {
          conflicts++
        } else {
          failures++
          lastFailure = err instanceof Error ? err.message : String(err)
        }
      }
    }
    if (done.length > 0) {
      const doneSet = new Set(done)
      setListings((prev) => {
        if (!prev) return prev
        if (archivedView === 'all') {
          return prev.map((l) => (doneSet.has(l.jobListing.id) ? { ...l, jobListing: { ...l.jobListing, archived: true } } : l))
        }
        return archivedView === 'only' ? prev : prev.filter((l) => !doneSet.has(l.jobListing.id))
      })
      setSelected((prev) => new Set([...prev].filter((id) => !doneSet.has(id))))
    }
    if (conflicts > 0) setConflict({ record: 'Job Listing', action: 'change' })
    if (failures > 0) setArchiveError(lastFailure)
    const skipped = conflicts + failures
    setBulkResult(
      `Archived ${done.length} Job Listing${done.length === 1 ? '' : 's'}` +
        (skipped > 0 ? `, ${skipped} not archived.` : '.'),
    )
    setBulkArchiving(false)
  }

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!listings) return <p>Loading…</p>

  return (
    <>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="mb-0">Job Listings</h1>
        <div className="flex items-center gap-2">
          <Button asChild variant="outline">
            <a
              href={exportDataUrl()}
              download
              title="Zip backup of your Master Data (profile, experience, projects, cover letter snippets), Job Listings, Applications and Pending Captures"
            >
              Export backup
            </a>
          </Button>
          <Button asChild>
            <Link to="/jobs/new">+ Save Job Listing</Link>
          </Button>
        </div>
      </div>

      {conflict && (
        <ConflictAlert
          record={conflict.record}
          action={conflict.action}
          keepsEdits={false}
          onReload={handleReloadAfterConflict}
          reloading={reloading}
        />
      )}

      {statusError && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {statusError}
        </p>
      )}
      {archiveError && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {archiveError}
        </p>
      )}

      {priorityError && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {priorityError}
        </p>
      )}

      <div className="sticky top-0 z-10 mb-4 flex flex-wrap items-end gap-3 rounded-xl border border-border bg-card/95 p-3 backdrop-blur">
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-archived" className="text-xs font-medium text-muted-foreground">
            Show
          </label>
          <Select value={archivedView} onValueChange={(value) => setArchivedView(value as ArchivedView)}>
            <SelectTrigger id="filter-archived" size="sm" className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ARCHIVED_VIEWS.map((view) => (
                <SelectItem key={view} value={view}>
                  {ARCHIVED_VIEW_LABELS[view]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-status" className="text-xs font-medium text-muted-foreground">
            Status
          </label>
          <Select
            value={statusFilter || ALL_STATUSES_VALUE}
            onValueChange={(value) => setFilter('status', value === ALL_STATUSES_VALUE ? '' : value)}
          >
            <SelectTrigger id="filter-status" size="sm" className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_STATUSES_VALUE}>All statuses</SelectItem>
              {(Object.keys(statusLabel) as ApplicationStatus[]).map((status) => (
                <SelectItem key={status} value={status}>
                  {statusLabel[status]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-priority" className="text-xs font-medium text-muted-foreground">
            Priority
          </label>
          <Select
            value={priorityFilter || ALL_PRIORITIES_VALUE}
            onValueChange={(value) => setFilter('priority', value === ALL_PRIORITIES_VALUE ? '' : value)}
          >
            <SelectTrigger id="filter-priority" size="sm" className="w-36">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_PRIORITIES_VALUE}>All priorities</SelectItem>
              {PRIORITIES.map((p) => (
                <SelectItem key={p} value={p}>
                  {PRIORITY_LABELS[p]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-search" className="text-xs font-medium text-muted-foreground">
            Search
          </label>
          <Input
            id="filter-search"
            className="h-9 w-56"
            placeholder="Title, company or description"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-company" className="text-xs font-medium text-muted-foreground">
            Company
          </label>
          <Input
            id="filter-company"
            className="h-9 w-48"
            placeholder="e.g. acme"
            value={companyFilter}
            onChange={(e) => setFilter('company', e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-location" className="text-xs font-medium text-muted-foreground">
            Location
          </label>
          <Input
            id="filter-location"
            className="h-9 w-48"
            placeholder="e.g. Milan, or Remote"
            value={locationFilter}
            onChange={(e) => setFilter('location', e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-saved-from" className="text-xs font-medium text-muted-foreground">
            Saved from
          </label>
          <Input
            id="filter-saved-from"
            type="date"
            className="h-9"
            value={savedFromFilter}
            onChange={(e) => setFilter('savedFrom', e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="filter-saved-to" className="text-xs font-medium text-muted-foreground">
            Saved to
          </label>
          <Input
            id="filter-saved-to"
            type="date"
            className="h-9"
            value={savedToFilter}
            onChange={(e) => setFilter('savedTo', e.target.value)}
          />
        </div>
        {hasActiveFilter && (
          <Button size="sm" variant="ghost" onClick={clearFilters}>
            Clear filters
          </Button>
        )}
      </div>

      <div className="mb-4 flex flex-wrap items-end gap-3 rounded-xl border border-border bg-card p-3">
        <Button
          size="sm"
          variant={prioritySort ? 'default' : 'outline'}
          aria-pressed={prioritySort}
          onClick={() => setPrioritySort(!prioritySort)}
          title="Show high priority Job Listings first. Ignored while a RAL sort is applied."
        >
          High priority first
        </Button>
        <div className="flex flex-col gap-1">
          <label htmlFor="ral-sort" className="text-xs font-medium text-muted-foreground">
            Sort by RAL Range
          </label>
          <Select value={ralSort} onValueChange={(value) => setRalSort(value as RALSortChoice)}>
            <SelectTrigger id="ral-sort" size="sm" className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Default (newest first)</SelectItem>
              <SelectItem value="desc">RAL: high to low</SelectItem>
              <SelectItem value="asc">RAL: low to high</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="ral-min" className="text-xs font-medium text-muted-foreground">
            Min RAL
          </label>
          <Input
            id="ral-min"
            type="number"
            inputMode="numeric"
            className="w-28"
            placeholder="e.g. 40000"
            value={ralMinInput}
            onChange={(e) => setRalMinInput(e.target.value)}
          />
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="ral-max" className="text-xs font-medium text-muted-foreground">
            Max RAL
          </label>
          <Input
            id="ral-max"
            type="number"
            inputMode="numeric"
            className="w-28"
            placeholder="e.g. 60000"
            value={ralMaxInput}
            onChange={(e) => setRalMaxInput(e.target.value)}
          />
        </div>

        {knownCurrencies.length > 1 && (
          <div className="flex flex-col gap-1">
            <label htmlFor="ral-currency" className="text-xs font-medium text-muted-foreground">
              Currency
            </label>
            <Select value={ralCurrency} onValueChange={setRalCurrency}>
              <SelectTrigger id="ral-currency" size="sm" className="w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {knownCurrencies.map((currency) => (
                  <SelectItem key={currency} value={currency}>
                    {currency}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        <Button
          size="sm"
          variant="outline"
          onClick={handleApplyRALFilter}
          disabled={ralMinInput.trim() === '' && ralMaxInput.trim() === ''}
        >
          Apply RAL filter
        </Button>

        {hasAppliedRALFilter && (
          <Button size="sm" variant="ghost" onClick={handleClearRALFilter}>
            Clear RAL filter
          </Button>
        )}

        {ralFilterError && (
          <p role="alert" className="text-sm font-medium text-destructive">
            {ralFilterError}
          </p>
        )}
      </div>

      {archivedView !== 'only' && (
        <div className="mb-4 flex flex-wrap items-end gap-3 rounded-xl border border-border bg-card p-3">
          <div className="flex flex-col gap-1">
            <label htmlFor="stale-days" className="text-xs font-medium text-muted-foreground">
              Saved for over (days)
            </label>
            <Input
              id="stale-days"
              type="number"
              min={0}
              inputMode="numeric"
              className="h-9 w-24"
              value={staleDays}
              onChange={(e) => setStaleDays(e.target.value)}
            />
          </div>
          <Button size="sm" variant="outline" onClick={handleSelectStale}>
            Select Saved for over {staleDays || '…'} days
          </Button>
          <Button
            size="sm"
            onClick={handleBulkArchive}
            disabled={bulkArchiving || selected.size === 0}
          >
            Archive selected ({selected.size})
          </Button>
          {selected.size > 0 && (
            <Button size="sm" variant="ghost" onClick={() => setSelected(new Set())}>
              Clear selection
            </Button>
          )}
          {bulkResult && (
            <p role="status" className="text-sm font-medium">
              {bulkResult}
            </p>
          )}
        </div>
      )}

      {listings.length === 0 && (hasActiveFilter || hasAppliedRALFilter) && (
        <p>No Job Listings match the current filters.</p>
      )}
      {listings.length === 0 && !hasActiveFilter && !hasAppliedRALFilter && (
        <p>
          {archivedView === 'only'
            ? 'No archived Job Listings.'
            : archivedView === 'exclude'
              ? 'No active Job Listings. Archived ones are under Show: Archived.'
              : 'No Job Listings saved yet.'}
        </p>
      )}

      <ul className="flex flex-col gap-3">
        {listings.map(({ jobListing, application }) => {
          const needsResolve = jobListing.ral.source === 'unresolved' || application.method.kind === 'unresolved'
          const detailState: JobListingDetailLocationState = { from: location.pathname + location.search }
          return (
            <li
              key={jobListing.id}
              data-archived={jobListing.archived}
              className={`rounded-xl border border-border px-4 py-3 ${
                jobListing.archived ? 'border-dashed bg-muted/40 opacity-70' : 'bg-card'
              }`}
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="flex min-w-0 items-center gap-2">
                  {isBulkSelectable({ jobListing, application }) && (
                    <Checkbox
                      aria-label={`Select ${jobListingHeading(jobListing)}`}
                      checked={selected.has(jobListing.id)}
                      disabled={bulkArchiving}
                      onCheckedChange={(on) => toggleSelected(jobListing.id, on === true)}
                    />
                  )}
                  {jobListing.logo && (
                    <img
                      src={jobListingLogoUrl(jobListing.id)}
                      alt=""
                      className="h-8 w-8 shrink-0 rounded object-contain"
                    />
                  )}
                  <Link
                    to={`/jobs/${encodeURIComponent(jobListing.id)}`}
                    state={detailState}
                    className="break-words font-semibold no-underline hover:underline"
                  >
                    {jobListingHeading(jobListing)}
                  </Link>
                </span>
                <div className="flex flex-wrap items-center gap-2">
                  {jobListing.archived && <Badge variant="outline">Archived</Badge>}
                  <ApplicationStatusBadges
                    application={application}
                    needsAttention={
                      needsResolve && (
                        <Badge variant="outline" className="border-unresolved text-unresolved">
                          Needs attention
                        </Badge>
                      )
                    }
                  />
                  <ApplicationStatusControl
                    company={jobListing.company}
                    status={application.status}
                    disabled={updatingId === jobListing.id}
                    onMove={(next, closeReason) => handleStatusChange(jobListing.id, next, closeReason)}
                  />
                </div>
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-sm">
                <span className="text-muted-foreground">Saved {new Date(jobListing.savedAt).toLocaleDateString()}</span>
                {jobListing.url && <FreshnessBadge status={jobListing.freshnessStatus} />}
                <span className="ml-auto">
                  <PriorityControl
                    value={jobListing.priority}
                    label={jobListing.company}
                    disabled={prioritizingId === jobListing.id}
                    onChange={(next) => handlePriorityChange(jobListing.id, next)}
                  />
                </span>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={archivingId === jobListing.id}
                  onClick={() => handleArchiveToggle(jobListing.id, !jobListing.archived)}
                >
                  {jobListing.archived ? 'Unarchive' : 'Archive'}
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={`/jobs/${jobListing.id}/generate`}>
                    {application.generations?.length ? 'Regenerate CV' : 'Generate CV'}
                  </Link>
                </Button>
              </div>
              <div className="mt-2">
                <RALBadge ral={jobListing.ral} />
              </div>
            </li>
          )
        })}
      </ul>
    </>
  )
}
