import type {
  AddedTrackedBoard,
  AddTrackedBoardRequest,
  Application,
  ApplicationGroups,
  ApplicationMailto,
  ApplicationMethod,
  ApplicationStats,
  ApplicationStatus,
  ArchivedView,
  AtsListing,
  AddPendingCaptureRequest,
  AddPendingCaptureResult,
  AtsProvider,
  AuthStatus,
  CaptureHints,
  CompletePendingCaptureRequest,
  Contact,
  Entry,
  EntryInput,
  GenerateRequest,
  GenerateResult,
  IndexedGeneration,
  JobListing,
  JobListingResponse,
  JobListingSummaryWithApplication,
  JobListingWithApplication,
  PendingCapture,
  Profile,
  RALListQuery,
  RecordGenerationRequest,
  RenderRequest,
  RenderResult,
  SaveJobListingRequest,
  SaveJobListingResult,
  Snippet,
  SnippetInput,
  TagLintReport,
  TrackedBoard,
  UsageSummary,
} from './types'

// ApiError is what request() throws for a non-2xx response: the message is
// still the backend's body (what every page shows today), and status lets a
// page branch on the status rather than on message text — the Job Listing
// detail page's not-found state (issue #94), and 409, a write refused
// because the record changed on disk since it was read (issue #89).
export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// isConflict reports whether err is a write the backend refused because
// its version token no longer matches the file on disk.
export function isConflict(err: unknown): boolean {
  return err instanceof ApiError && err.status === 409
}

// UNAUTHORIZED_EVENT is dispatched on window whenever the backend answers
// 401: in token mode that means this device's access is missing or was
// revoked by a token rotation, and AccessGate swaps the app for the token
// screen (issue #181). Pages keep handling the thrown ApiError as before.
export const UNAUTHORIZED_EVENT = 'sumisura:unauthorized'

async function ensureOk(res: Response, fallback: string): Promise<void> {
  if (res.status === 401) {
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
  }
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new ApiError(body || `${fallback} (${res.status})`, res.status)
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  await ensureOk(res, `Request to ${path} failed`)
  return res.json()
}

// jsonHeaders builds a JSON write's headers, adding If-Match when the
// caller holds a version token from its read. Every FE write path passes
// the token it read — the header is optional only on the wire, for non-FE
// callers such as the tailor-cv skill (issue #89).
function jsonHeaders(version?: string): HeadersInit {
  return version ? { 'Content-Type': 'application/json', 'If-Match': version } : { 'Content-Type': 'application/json' }
}

function versionHeaders(version?: string): HeadersInit {
  return version ? { 'If-Match': version } : {}
}

export function listEntries(): Promise<Entry[]> {
  return request('/api/master-data/entries')
}

export function getEntry(id: string): Promise<Entry> {
  return request(`/api/master-data/entries/${id}`)
}

export function updateEntry(id: string, input: EntryInput, version: string | undefined): Promise<Entry> {
  return request(`/api/master-data/entries/${id}`, {
    method: 'PUT',
    headers: jsonHeaders(version),
    body: JSON.stringify(input),
  })
}

export function createEntry(input: EntryInput): Promise<Entry> {
  return request('/api/master-data/entries', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function deleteEntry(id: string, version: string | undefined): Promise<void> {
  const res = await fetch(`/api/master-data/entries/${id}`, { method: 'DELETE', headers: versionHeaders(version) })
  await ensureOk(res, 'Delete failed')
}

export function getTagLint(): Promise<TagLintReport> {
  return request('/api/master-data/tag-lint')
}

export function getProfile(): Promise<Profile> {
  return request('/api/master-data/profile')
}

export function updateProfile(profile: Profile): Promise<Profile> {
  const { version, ...body } = profile
  return request('/api/master-data/profile', {
    method: 'PUT',
    headers: jsonHeaders(version),
    body: JSON.stringify(body),
  })
}

export function listSnippets(): Promise<Snippet[]> {
  return request('/api/master-data/cover-letter-snippets')
}

export function getSnippet(id: string): Promise<Snippet> {
  return request(`/api/master-data/cover-letter-snippets/${id}`)
}

export function createSnippet(input: SnippetInput): Promise<Snippet> {
  return request('/api/master-data/cover-letter-snippets', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function updateSnippet(id: string, input: SnippetInput, version: string | undefined): Promise<Snippet> {
  return request(`/api/master-data/cover-letter-snippets/${id}`, {
    method: 'PUT',
    headers: jsonHeaders(version),
    body: JSON.stringify(input),
  })
}

export async function deleteSnippet(id: string, version: string | undefined): Promise<void> {
  const res = await fetch(`/api/master-data/cover-letter-snippets/${id}`, {
    method: 'DELETE',
    headers: versionHeaders(version),
  })
  await ensureOk(res, 'Delete failed')
}

export function createGeneration(req: GenerateRequest): Promise<GenerateResult> {
  return request('/api/generations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export function previewGeneration(req: GenerateRequest): Promise<GenerateResult> {
  return request('/api/generations/preview', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export function renderGeneration(req: RenderRequest): Promise<RenderResult> {
  return request('/api/generations/render', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export function generationFileUrl(slug: string, file: string): string {
  return `/api/generations/${encodeURIComponent(slug)}/${encodeURIComponent(file)}`
}

// generationFileOnDisk reports whether a Generation's file is still served.
// output/ is derived and may be cleared at any time (ADR-0008), so a 404 is
// an expected state, not an error. Anything other than a definite 404
// (network failure, 5xx) resolves true, so the UI never claims files are
// gone when it simply couldn't check.
export async function generationFileOnDisk(slug: string, file: string): Promise<boolean> {
  try {
    const res = await fetch(generationFileUrl(slug, file), { method: 'HEAD' })
    return res.status !== 404
  } catch {
    return true
  }
}

export interface JobListingsFilter {
  status?: ApplicationStatus
  company?: string
  savedFrom?: string
  savedTo?: string
  archived?: ArchivedView
}

// listJobListings passes filter's status/company/savedFrom/savedTo (issue
// #45), archived (issue #98) and sortByRAL/ralMin/ralMax/ralCurrency (issue #51) through to GET
// /api/job-listings's matching optional query params — omitted entirely
// when not given, matching the endpoint's own unfiltered/unsorted default.
// Each row's Job Listing is a summary without the Job Description text
// (issue #97); getJobListing is where that text comes from.
export function listJobListings(
  filter?: JobListingsFilter & RALListQuery,
): Promise<JobListingSummaryWithApplication[]> {
  const params = new URLSearchParams()
  if (filter?.status) params.set('status', filter.status)
  if (filter?.company) params.set('company', filter.company)
  if (filter?.savedFrom) params.set('savedFrom', filter.savedFrom)
  if (filter?.savedTo) params.set('savedTo', filter.savedTo)
  // exclude is the backend's own default, so it is left off the request.
  if (filter?.archived && filter.archived !== 'exclude') params.set('archived', filter.archived)
  if (filter?.sortByRAL) {
    params.set('sort', 'ral')
    params.set('order', filter.sortByRAL)
  }
  if (filter?.ralMin != null) params.set('ral_min', String(filter.ralMin))
  if (filter?.ralMax != null) params.set('ral_max', String(filter.ralMax))
  if (filter?.ralCurrency) params.set('ral_currency', filter.ralCurrency)
  const qs = params.toString()
  return request(`/api/job-listings${qs ? `?${qs}` : ''}`)
}

export function exportDataUrl(): string {
  return '/api/export'
}

// listApplications reads the Applications view (issue #95): every tracked
// Application grouped under its Status, server-ordered. archived mirrors
// listJobListings's, and exclude, the backend's default, is left off.
export function listApplications(archived?: ArchivedView): Promise<ApplicationGroups> {
  const qs = archived && archived !== 'exclude' ? `?archived=${archived}` : ''
  return request(`/api/applications${qs}`)
}

// listGenerations reads every CV generated so far (issue #173): the
// Generations recorded against Applications, merged with the output/
// directories nothing recorded.
export function listGenerations(): Promise<IndexedGeneration[]> {
  return request('/api/generations')
}

// deleteGeneration clears one Generation's output/<slug>/ directory. It
// deletes derived files only: a Generation recorded against an Application
// keeps its record, and the row then reports its files as missing.
export async function deleteGeneration(slug: string): Promise<void> {
  const res = await fetch(`/api/generations/${encodeURIComponent(slug)}`, { method: 'DELETE' })
  await ensureOk(res, 'Delete failed')
}

export function getApplicationsStats(): Promise<ApplicationStats> {
  return request('/api/applications/stats')
}

// getJobListing returns a Job Listing paired with its Application — the
// shape listJobListings returns per row, stale-Entry information included
// (issue #94), but with the whole Job Listing, Job Description text
// included (issue #97), so the Job Listing detail page needs one request.
export function getJobListing(id: string): Promise<JobListingWithApplication> {
  return request(`/api/job-listings/${encodeURIComponent(id)}`)
}

export function jobListingLogoUrl(id: string): string {
  return `/api/job-listings/${encodeURIComponent(id)}/logo`
}

export function saveJobListing(req: SaveJobListingRequest): Promise<SaveJobListingResult> {
  return request('/api/job-listings', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

// deleteJobListing removes the Job Listing and its Application together,
// so it presents both tokens: the Job Listing's as If-Match and the
// Application's as Application-If-Match (issue #89, story 19).
export async function deleteJobListing(
  id: string,
  jobListingVersion: string | undefined,
  applicationVersion: string | undefined,
): Promise<void> {
  const headers: Record<string, string> = {}
  if (jobListingVersion) headers['If-Match'] = jobListingVersion
  if (applicationVersion) headers['Application-If-Match'] = applicationVersion
  const res = await fetch(`/api/job-listings/${encodeURIComponent(id)}`, { method: 'DELETE', headers })
  await ensureOk(res, 'Delete failed')
}

export function suggestContact(jobListingId: string): Promise<Contact> {
  return request(`/api/job-listings/${encodeURIComponent(jobListingId)}/suggest-contact`, { method: 'POST' })
}

export function resolveJobListing(jobListingId: string): Promise<JobListingWithApplication> {
  return request(`/api/job-listings/${encodeURIComponent(jobListingId)}/resolve`, { method: 'POST' })
}

export async function checkJobListingFreshness(jobListingId: string): Promise<JobListing> {
  const result = await request<JobListingResponse>(
    `/api/job-listings/${encodeURIComponent(jobListingId)}/check-freshness`,
    { method: 'POST' },
  )
  return result.jobListing
}

// setJobListingArchived archives (true) or unarchives (false) a Job Listing
// (issue #98). Both calls are idempotent and never touch the Application.
// They rewrite the Job Listing file, so they present its version token
// (issue #89) and answer with the fresh one.
export async function setJobListingArchived(
  jobListingId: string,
  archived: boolean,
  version: string | undefined,
): Promise<JobListing> {
  const result = await request<JobListingResponse>(
    `/api/job-listings/${encodeURIComponent(jobListingId)}/${archived ? 'archive' : 'unarchive'}`,
    { method: 'POST', headers: versionHeaders(version) },
  )
  return result.jobListing
}

export function updateApplicationStatus(id: string, status: ApplicationStatus, version: string | undefined): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/status`, {
    method: 'PATCH',
    headers: jsonHeaders(version),
    body: JSON.stringify({ status }),
  })
}

export function updateApplicationMethod(id: string, method: ApplicationMethod, version: string | undefined): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/method`, {
    method: 'PATCH',
    headers: jsonHeaders(version),
    body: JSON.stringify(method),
  })
}

export function getApplicationMailto(id: string): Promise<ApplicationMailto> {
  return request(`/api/applications/${encodeURIComponent(id)}/mailto`)
}

export function updateApplicationContact(id: string, contact: Contact, version: string | undefined): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/contact`, {
    method: 'PATCH',
    headers: jsonHeaders(version),
    body: JSON.stringify(contact),
  })
}

export function recordApplicationGeneration(id: string, req: RecordGenerationRequest): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/generations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

// addApplicationNote writes a Note (issue #96) and answers with the whole
// updated Application, as every other Application action does. Every Note
// write rewrites the Application file, so each presents the Application's
// version token (issue #89).
export function addApplicationNote(id: string, body: string, version: string | undefined): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/notes`, {
    method: 'POST',
    headers: jsonHeaders(version),
    body: JSON.stringify({ body }),
  })
}

// editApplicationNote corrects a Note's body; its createdAt never changes.
export function editApplicationNote(
  id: string,
  noteId: string,
  body: string,
  version: string | undefined,
): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/notes/${encodeURIComponent(noteId)}`, {
    method: 'PATCH',
    headers: jsonHeaders(version),
    body: JSON.stringify({ body }),
  })
}

// deleteApplicationNote hard-deletes one Note, answering with the updated
// Application.
export function deleteApplicationNote(id: string, noteId: string, version: string | undefined): Promise<Application> {
  return request(`/api/applications/${encodeURIComponent(id)}/notes/${encodeURIComponent(noteId)}`, {
    method: 'DELETE',
    headers: versionHeaders(version),
  })
}

export function listAtsListings(provider: AtsProvider, boardSlug: string): Promise<AtsListing[]> {
  return request(`/api/ats/${encodeURIComponent(provider)}/${encodeURIComponent(boardSlug)}/listings`)
}

export function listTrackedBoards(): Promise<TrackedBoard[]> {
  return request('/api/ats/tracked-boards')
}

// addTrackedBoard answers with the stored board, which has no newCount: only
// listTrackedBoards computes one.
export function addTrackedBoard(req: AddTrackedBoardRequest): Promise<AddedTrackedBoard> {
  return request('/api/ats/tracked-boards', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export async function removeTrackedBoard(id: string): Promise<void> {
  const res = await fetch(`/api/ats/tracked-boards/${encodeURIComponent(id)}`, { method: 'DELETE' })
  await ensureOk(res, 'Delete failed')
}

export function getUsageSummary(): Promise<UsageSummary> {
  return request('/api/usage')
}

export function listPendingCaptures(): Promise<PendingCapture[]> {
  return request('/api/pending-captures')
}

export function addPendingCapture(req: AddPendingCaptureRequest): Promise<AddPendingCaptureResult> {
  return request('/api/pending-captures', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export function completePendingCapture(id: string, req: CompletePendingCaptureRequest): Promise<SaveJobListingResult> {
  return request(`/api/pending-captures/${encodeURIComponent(id)}/complete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export function suggestPendingCaptureHints(id: string, jobDescription: string): Promise<CaptureHints> {
  return request(`/api/pending-captures/${encodeURIComponent(id)}/hints`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jobDescription }),
  })
}

export async function deletePendingCapture(id: string): Promise<void> {
  const res = await fetch(`/api/pending-captures/${encodeURIComponent(id)}`, { method: 'DELETE' })
  await ensureOk(res, 'Could not dismiss this link')
}

export function getAuthStatus(): Promise<AuthStatus> {
  return request('/api/auth/status')
}

// createAuthSession exchanges the access token for the HttpOnly access
// cookie. The token is never kept on the client: the cookie is the
// credential, which is also what lets PDF links and logo images through.
export async function createAuthSession(token: string): Promise<void> {
  const res = await fetch('/api/auth/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  })
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new ApiError(body.trim() || `Access check failed (${res.status})`, res.status)
  }
}

export async function deleteAuthSession(): Promise<void> {
  const res = await fetch('/api/auth/session', { method: 'DELETE' })
  await ensureOk(res, 'Could not forget this device')
}
