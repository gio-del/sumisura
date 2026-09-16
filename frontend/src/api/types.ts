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

export type RALSource = 'stated' | 'estimated' | 'n/a' | 'unresolved' | 'conflict'

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
}

export type ParsabilityStatus = 'ok' | 'warning' | 'unavailable'

// ParsabilityResult is the ATS-parsability check's structured outcome
// (backend/internal/generation/parsability.go): non-blocking, surfaced at
// Visual Review as a warning badge alongside the CV/Cover Letter preview.
export interface ParsabilityResult {
  status: ParsabilityStatus
  missingFields?: string[]
  orderingViolations?: string[]
  reason?: string
}

export interface RenderResult {
  slug: string
  cvPath: string
  coverLetterPath?: string
  cvPageCount: number
  cvParsability: ParsabilityResult
  coverLetterParsability?: ParsabilityResult
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

