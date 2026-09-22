export type EntryType = 'experience' | 'project'

export interface EntryLastModified {
  at?: string
  subject?: string
}

export interface Entry {
  id: string
  type: EntryType
  employer?: string
  client?: string
  role?: string
  name?: string
  location?: string
  start: string
  end: string | null
  flagship?: boolean
  tags: string[]
  repo?: string
  bullets?: string[]
  /** Absent when the Entry has no git history yet (freshly added, uncommitted). */
  lastModified?: EntryLastModified
  // version is the opaque token of the Entry file as read — sent back as
  // If-Match on a save or delete, never shown (issue #89).
  version: string
}

export type EntryInput = Omit<Entry, 'id' | 'version' | 'lastModified'>

export interface Education {
  degree: string
  institution: string
  program: string
  start: string
  end: string
  grade: string
  courses?: string[]
}

export interface Publication {
  title: string
  authors: string
  venue: string
  link?: string
  note?: string
}

export interface Certification {
  title: string
  issuer?: string
  date?: string
  link?: string
}

export interface Award {
  title: string
  description?: string
}

export interface Activity {
  title: string
  description?: string
}

export interface Language {
  name: string
  level: string
}

export interface Profile {
  name: string
  location: string
  email: string
  phone: string
  linkedin: string
  github: string
  education: Education[]
  publications: Publication[]
  certifications: Certification[]
  awards: Award[]
  activities: Activity[]
  languages: Language[]
  // version is the opaque token of the Profile file as read (issue #89).
  version: string
}

export interface Snippet {
  id: string
  kind: string
  // lang is the ISO 639-1 code this Snippet is written in, absent when it is
  // unmarked. Cover Letter drafting prefers a Snippet already in the target
  // language over translating one (issue #168).
  lang?: string
  tags: string[]
  body: string
  lastUsedAt?: string
  // version is the opaque token of the Snippet file as read (issue #89).
  version: string
}

export type SnippetInput = Omit<Snippet, 'id' | 'version'>

export interface SelectedBullet {
  sourceIndex: number
  source: string
  rewritten: string
}

export interface SelectedEntry {
  entryId: string
  reason: string
  bullets: SelectedBullet[]
}

export interface SelectionResult {
  entries: SelectedEntry[]
  // language is the ISO 639-1 code Selection detected (or was told via
  // languageOverride). Absent in Default Mode and from Preview; the
  // normalized value Text Review uses is GenerateResult.language.
  language?: string
}

export type GenerateMode = 'default' | 'tailored'

export interface CoverLetterResult {
  body: string
  sourceSnippetIds?: string[]
}

// 'manual' is the figure the user entered themselves (issue #206) — the
// most reliable source on the record, and the only one no inference
// produces. Re-resolution leaves it alone.
export type RALSource = 'stated' | 'estimated' | 'n/a' | 'unresolved' | 'conflict' | 'manual'

export interface RALFigure {
  min: number
  max: number
  currency: string
}

export interface RALRange {
  min?: number
  max?: number
  currency?: string
  source: RALSource
  descriptionStated?: RALFigure
  listingStated?: RALFigure
}

export interface CallUsage {
  callType: string
  model: string
  inputTokens: number
  outputTokens: number
  cacheReadTokens?: number
  cacheWriteTokens?: number
  webSearchUses?: number
  estimatedCostUsd: number
}

export interface GenerationUsage {
  inputTokens: number
  outputTokens: number
  cacheReadTokens?: number
  cacheWriteTokens?: number
  webSearchUses?: number
  estimatedCostUsd: number
  calls?: CallUsage[]
}

// PendingCapture is a link to a job posting shared from another device (a
// phone's share sheet), waiting in the To complete inbox until its Job
// Description is known (issue #182). Not a Job Listing: no Application, no
// Status. title and company are unconfirmed hints from what was shared.
export interface PendingCapture {
  schemaVersion: number
  id: string
  url: string
  postingKey: string
  provider: PostingProvider
  title?: string
  company?: string
  sharedText?: string
  savedAt: string
}

export type PostingProvider = 'linkedin' | 'indeed' | 'greenhouse' | 'lever' | 'ashby' | 'other'

export interface AddPendingCaptureRequest {
  url?: string
  text?: string
  title?: string
}

export type PendingCaptureOutcome = 'pending' | 'already-pending' | 'already-tracked' | 'job-listing'

// AddPendingCaptureResult is POST /api/pending-captures' body: what sharing
// did, and a ready-made sentence saying so. pendingCapture is present for
// pending/already-pending, jobListingId for already-tracked and
// job-listing, and jobListing/application for job-listing — a public ATS
// posting saved straight away (issue #184).
export interface AddPendingCaptureResult {
  outcome: PendingCaptureOutcome
  message: string
  pendingCapture?: PendingCapture
  jobListingId?: string
  jobListing?: JobListing
  application?: Application
}

export interface CompletePendingCaptureRequest {
  company: string
  title?: string
  jobDescription: string
}

// CaptureHints are the Company and Job Title read out of a pasted Job
// Description (issue #200), to pre-fill the completion form. Either is empty
// when the text doesn't say.
export interface CaptureHints {
  company: string
  title: string
}

// AuthStatus is GET /api/auth/status's body (issue #181): whether this
// installation requires the access token, and whether this device already
// has access (the access cookie, or the header for non-browser clients).
export interface AuthStatus {
  required: boolean
  authenticated: boolean
}

// UsageSummary is GET /api/usage's body: the lifetime usage totals plus a
// completeness signal (issue #102). Distinct from GenerationUsage, which is
// also persisted per Generation where completeness has no meaning.
export interface UsageSummary extends GenerationUsage {
  incomplete?: boolean
  incompleteReason?: string
}

export type GroundednessReason = 'no-source-match' | 'numeric-mismatch'

export interface GroundednessFlag {
  sentence: string
  reason: GroundednessReason
}

export interface BulletGroundedness {
  entryId: string
  sourceIndex: number
  flags: GroundednessFlag[]
}

export interface GroundednessResult {
  bullets?: BulletGroundedness[]
  coverLetter?: GroundednessFlag[]
}

export interface GenerateResult {
  mode: GenerateMode
  jobDescription?: string
  selection: SelectionResult
  coverLetter?: CoverLetterResult
  ral?: RALRange
  usage: GenerationUsage
  groundedness?: GroundednessResult
  language: string
}

export interface GenerateRequest {
  jobDescription?: string
  jobDescriptionUrl?: string
  languageOverride?: string
}

export interface RenderRequest {
  slug: string
  selection: SelectionResult
  coverLetter?: { body: string }
  language?: string
  // jobDescription, when given, adds term coverage to the CV's ATS Report.
  jobDescription?: string
}

export type ParsabilityStatus = 'ok' | 'warning' | 'unavailable'

export type ATSFieldGroup = 'identity' | 'contact' | 'section' | 'experience' | 'project' | 'body'

// ATSField is one expected field's verdict; inOrder only means something
// when found.
export interface ATSField {
  label: string
  group: ATSFieldGroup
  found: boolean
  inOrder: boolean
}

// TermCoverage lists the Job Description's terms that are also Master Data
// tags, by whether the text layer contains them. Informational only.
export interface TermCoverage {
  present: string[]
  missing: string[]
}

// ATSReport is the ATS-parsability check's outcome for one rendered PDF
// (backend/internal/generation/parsability.go, issues #49 and #198):
// non-blocking, shown at Visual Review and kept on the Generation, with the
// extracted text an ATS would read.
export interface ATSReport {
  status: ParsabilityStatus
  reason?: string
  fields?: ATSField[]
  missingFields?: string[]
  orderingViolations?: string[]
  termCoverage?: TermCoverage
  extractedText?: string
}

// ATSReports are a Generation's ATS Reports: the CV's, and the Cover
// Letter's when there is one. GET /api/generations/{slug}/ats-report's body.
export interface ATSReports {
  cv: ATSReport
  coverLetter?: ATSReport
}

export interface RenderResult {
  slug: string
  cvPath: string
  coverLetterPath?: string
  cvPageCount: number
  atsReports: ATSReports
}

// RALListQuery is GET /api/job-listings' optional RAL Range sort/filter
// query params (issue #51). sortOrder is only meaningful once a RAL sort
// is applied (asc/desc); ralCurrency defaults server-side to EUR when a
// min/max filter is set without one.
export interface RALListQuery {
  sortByRAL?: 'asc' | 'desc'
  ralMin?: number
  ralMax?: number
  ralCurrency?: string
}

export type JobListingSource = 'manual'

export type FreshnessStatus = 'not-yet-checked' | 'live' | 'unreachable' | 'unknown'

export interface JobListing {
  // schemaVersion is the record format generation (ADR-0034): 0 for a
  // legacy record not yet migrated.
  schemaVersion: number
  id: string
  title?: string
  company: string
  // location is where the role is, free text exactly as the source wrote
  // it (issue #206). Absent means the source had none, or — on a record
  // below schema version 2 — that it predates the field.
  location?: string
  url?: string
  source: JobListingSource
  savedAt: string
  jobDescription: string
  ral: RALRange
  logo?: string
  freshnessStatus: FreshnessStatus
  freshnessCheckedAt?: string
  // archived is set only by the user (issue #98): it takes the Job Listing
  // out of the default list and changes nothing else.
  archived: boolean
  // version is the opaque token of the Job Listing file as read — what
  // the Job Listing delete presents (issue #89).
  version: string
}

// ArchivedView is which Job Listings the list shows by their archived flag
// (issue #98), matching GET /api/job-listings's archived query param:
// exclude (the default) leaves archived ones out, only keeps just them.
export type ArchivedView = 'exclude' | 'only' | 'all'

export type ApplicationStatus = 'saved' | 'tailoring' | 'sent' | 'interviewing' | 'rejected' | 'offer' | 'withdrawn'

export type ApplicationMethodKind = 'portal' | 'email' | 'easy_apply' | 'other' | 'unresolved'

export interface ApplicationMethod {
  kind: ApplicationMethodKind
  value?: string
}

// IndexedGeneration is one row of the "every CV generated so far" index
// (GET /api/generations, issue #173). recorded=false means the row came
// from an output/ directory no Application record mentions — a Default Mode
// or skill run — so it has no application, groundedness or language.
export interface IndexedGeneration {
  slug: string
  createdAt: string
  recorded: boolean
  applicationId?: string
  company?: string
  jobTitle?: string
  cvPath?: string
  coverLetterPath?: string
  language?: string
  groundedness?: GroundednessResult
  hasCv: boolean
  hasCoverLetter: boolean
  // hasAtsReport says whether GET /api/generations/{slug}/ats-report has
  // something to show.
  hasAtsReport: boolean
}

export interface GenerationRecord {
  // schemaVersion is absent on a Generation recorded before ADR-0034, whose
  // missing sourceSnippetIds/entryIds/language are then unknowable rather
  // than genuinely empty.
  schemaVersion?: number
  slug: string
  createdAt: string
  cvPath: string
  coverLetterPath?: string
  groundedness?: GroundednessResult
  // atsReports is absent on a Generation recorded before ATS Reports were
  // kept, or whose check couldn't be attached: "not recorded", never "ok".
  atsReports?: ATSReports
  sourceSnippetIds?: string[]
  // usage is always sent: zero-valued for a Default Mode Generation or one
  // recorded before usage was kept.
  usage: GenerationUsage
  language?: string
  entryIds?: string[]
  // staleEntries names (by employer/client + role) which of entryIds have
  // been edited in Master Data since createdAt — computed read-time by the
  // backend, never stored (issue #52). Empty/absent means not stale (or
  // not checkable, e.g. a record with no stored entryIds).
  staleEntries?: string[]
}

export interface Contact {
  name: string
  email: string
}

// StatusChange is one entry of an Application's Status history: the Status
// it moved to, and when.
export interface StatusChange {
  status: ApplicationStatus
  changedAt: string
}

export interface Application {
  // schemaVersion is the record format generation (ADR-0034): 0 for a
  // legacy record not yet migrated.
  schemaVersion: number
  id: string
  jobListingId: string
  status: ApplicationStatus
  statusUpdatedAt?: string
  method: ApplicationMethod
  contact?: Contact
  isStale: boolean
  generations?: GenerationRecord[]
  // statusHistory is every Status the Application moved to, oldest first,
  // its initial Saved included. Absent only on a record migrated while
  // already past Saved, whose transitions were never recorded (ADR-0034).
  statusHistory?: StatusChange[]
  // notes are the user's own log on this Application (issue #96), newest
  // first. Absent means no Notes.
  notes?: Note[]
  // version is the opaque token of the Application file alone (not its
  // Job Listing's) — what the Status/Method/Contact patches present
  // (issue #89).
  version: string
}

// Note is one timestamped Markdown observation on an Application (issue
// #96). id and createdAt are server-assigned: id is opaque and only ever
// echoed back, createdAt never changes. editedAt appears once the body has
// been corrected.
export interface Note {
  id: string
  createdAt: string
  editedAt?: string
  body: string
}

export interface StatusCount {
  status: ApplicationStatus
  count: number
}

export interface ConversionRate {
  from: ApplicationStatus
  to: ApplicationStatus
  rate: number
}

export interface StageTime {
  status: ApplicationStatus
  averageDays: number
  sampleSize: number
}

export interface ApplicationStats {
  total: number
  counts: StatusCount[]
  conversions: ConversionRate[]
  timeInStage: StageTime[]
}

export interface RecordGenerationRequest {
  slug: string
  cvPath: string
  coverLetterPath?: string
  sourceSnippetIds?: string[]
  usage?: GenerationUsage
  language?: string
  groundedness?: GroundednessResult
  atsReports?: ATSReports
  entryIds?: string[]
}

export interface SaveJobListingRequest {
  title?: string
  company: string
  url?: string
  jobDescription?: string
  jobDescriptionUrl?: string
  logoUrl?: string
}

export interface DuplicateMatch {
  jobListingId: string
  company: string
  title?: string
  savedAt: string
  score: number
}

export interface JobListingWithApplication {
  jobListing: JobListing
  application: Application
}

// JobListingResponse is what the check-freshness, archive and unarchive
// actions answer with: the updated Job Listing alone, since none of them
// touches the Application.
export interface JobListingResponse {
  jobListing: JobListing
}

// ApplicationMailto is GET /api/applications/{id}/mailto's draft.
export interface ApplicationMailto {
  uri: string
}

// JobListingSummary is a Job Listing as GET /api/job-listings returns it
// (issue #97): every field but the Job Description text, which only the
// detail endpoint carries — a separate type rather than an optional field,
// so reading a Job Description off a list row does not compile.
export type JobListingSummary = Omit<JobListing, 'jobDescription'> & {
  hasJobDescription: boolean
}

// JobListingSummaryWithApplication is one Job Listings list row: the
// summary paired with its whole Application.
export interface JobListingSummaryWithApplication {
  jobListing: JobListingSummary
  application: Application
}

// ApplicationGroup is one Status group of GET /api/applications (issue
// #95): its Applications, most overdue for attention first, each paired with
// its Job Listing summary exactly as a Job Listings list row is.
export interface ApplicationGroup {
  status: ApplicationStatus
  count: number
  items: JobListingSummaryWithApplication[]
}

// ApplicationGroups is the Applications view's data: always one group per
// Status in pipeline order, empty ones included, grouped and ordered by the
// backend so the page never re-derives either.
export interface ApplicationGroups {
  total: number
  groups: ApplicationGroup[]
}

export type SaveJobListingResult = JobListingWithApplication & {
  duplicateWarning?: DuplicateMatch
  // completedPendingCaptureId is the Pending Capture this save completed:
  // the same posting, shared earlier from a phone (issue #183).
  completedPendingCaptureId?: string
  // archivedJobListingId is the role a `replace` resolution archived in
  // the same action (issue #206).
  archivedJobListingId?: string
  // archiveFailed reports a replace whose save succeeded but whose archive
  // did not, so the user is never left believing they consolidated
  // something they didn't.
  archiveFailed?: boolean
}

// TrackedPosting is what the extension card shows for a posting already
// tracked, plus the Status moves it may offer (issue #206).
export interface TrackedPosting {
  id: string
  title?: string
  savedAt: string
  status: ApplicationStatus
  archived: boolean
  // allowedTransitions comes straight from the backend's Status state
  // machine, so the card never holds a second definition of it. Every move
  // is re-validated on the way in regardless.
  allowedTransitions: ApplicationStatus[]
}

// SiblingListing is one other role tracked at the same company: enough to
// answer "do I want this one too?" without leaving the job board.
export interface SiblingListing {
  id: string
  title?: string
  savedAt: string
  status: ApplicationStatus
}

// CaptureLookupResult answers the single form of POST
// /api/job-listings/capture-lookup: this posting's tracked state, and the
// company's other active roles. Read-only.
export interface CaptureLookupResult {
  tracked: TrackedPosting | null
  company: { listings: SiblingListing[] }
}

// BadgedPosting is the batch form's answer per row: only what a badge
// needs, since a search result needs a badge, not a decision.
export interface BadgedPosting {
  id: string
  status: ApplicationStatus
}

export interface CaptureLookupBatchResult {
  results: { url: string; tracked: BadgedPosting | null }[]
}

// ExistingJobListingRef is the Job Listing a refused save names: enough to
// recognise it, link to it and decide what to do (issue #206).
export interface ExistingJobListingRef {
  id: string
  title?: string
  company: string
  savedAt: string
  // archived is what lets the UI additionally offer to unarchive the match.
  // It never changes the message — an archived match reads exactly like any
  // other refusal.
  archived: boolean
}

// CompanyConflict is the same-company question the extension capture route
// raises when a save arrives with no decision on it and the company has
// other active roles (issue #206, story 24). Nothing is written while it
// stands. It is not a refusal to save: it asks, and the client answers
// with a CaptureResolution.
export interface CompanyConflict {
  reason: 'company-has-listings'
  message: string
  company: { listings: SiblingListing[] }
}

// CaptureResolution is the client's answer. `save-anyway` adds the role
// alongside the existing ones; `replace` saves it and archives one named
// existing role in the same action; `unarchive-existing` brings an
// archived listing back and saves nothing, answering a duplicate-posting
// refusal whose match was archived.
export type CaptureResolution =
  | { kind: 'save-anyway' }
  | { kind: 'replace'; jobListingId: string }
  | { kind: 'unarchive-existing'; jobListingId: string }

// SaveConflict is the body of a save refused with 409. Every save path
// answers with this one shape, so the UI has a single branch to read
// whichever route it called (issue #206).
export interface SaveConflict {
  // `replace-target-unavailable` means the listing chosen to replace was
  // deleted, already archived or at another company — the card was stale,
  // and nothing was written.
  reason: 'duplicate-posting' | 'replace-target-unavailable'
  message: string
  existing?: ExistingJobListingRef
}

export type AtsProvider = 'greenhouse' | 'lever' | 'ashby'

export interface AtsListing {
  title: string
  location: string
  url: string
  description: string
  alreadySaved: boolean
  logoUrl?: string
  new: boolean
}

export interface TrackedBoard {
  id: string
  provider: AtsProvider
  slug: string
  label?: string
  newCount: number
}

// AddedTrackedBoard is what POST /api/ats/tracked-boards answers with: the
// stored board, without the newCount only the list computes.
export type AddedTrackedBoard = Omit<TrackedBoard, 'newCount'>

export interface AddTrackedBoardRequest {
  provider: AtsProvider
  slug: string
  label?: string
}

export type TagLintConfidence = 'confident' | 'suggested'

export interface TagLintOccurrence {
  tag: string
  entryId: string
  entryType: EntryType
}

export interface TagLintGroup {
  key: string
  confidence: TagLintConfidence
  occurrences: TagLintOccurrence[]
}

export interface TagLintReport {
  groups: TagLintGroup[]
  singletons: TagLintOccurrence[]
}

