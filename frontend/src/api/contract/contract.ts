import type { Contract } from './check'
import type {
  AddedTrackedBoard,
  Application,
  ApplicationGroups,
  ApplicationMailto,
  ApplicationStats,
  AddPendingCaptureResult,
  AtsListing,
  AuthStatus,
  Contact,
  Entry,
  GenerateResult,
  IndexedGeneration,
  JobListingResponse,
  JobListingSummaryWithApplication,
  JobListingWithApplication,
  PendingCapture,
  Profile,
  RenderResult,
  SaveJobListingResult,
  Snippet,
  TagLintReport,
  TrackedBoard,
  UsageSummary,
} from '../types'

import type listEntriesPopulated from './fixtures/list-entries.populated.json'
import type getEntrySparse from './fixtures/get-entry.sparse.json'
import type createEntry from './fixtures/create-entry.json'
import type updateEntry from './fixtures/update-entry.json'
import type getProfilePopulated from './fixtures/get-profile.populated.json'
import type getProfileSparse from './fixtures/get-profile.sparse.json'
import type updateProfileSparse from './fixtures/update-profile.sparse.json'
import type tagLint from './fixtures/tag-lint.json'
import type listSnippetsPopulated from './fixtures/list-snippets.populated.json'
import type getSnippetSparse from './fixtures/get-snippet.sparse.json'
import type createSnippet from './fixtures/create-snippet.json'
import type updateSnippet from './fixtures/update-snippet.json'
import type listJobListingsPopulated from './fixtures/list-job-listings.populated.json'
import type listJobListingsSparse from './fixtures/list-job-listings.sparse.json'
import type getJobListingPopulated from './fixtures/get-job-listing.populated.json'
import type getJobListingSparse from './fixtures/get-job-listing.sparse.json'
import type saveJobListingSparse from './fixtures/save-job-listing.sparse.json'
import type saveJobListingDuplicate from './fixtures/save-job-listing.duplicate.json'
import type captureJobListingFromExtension from './fixtures/capture-job-listing-from-extension.json'
import type suggestContact from './fixtures/suggest-contact.json'
import type resolveJobListing from './fixtures/resolve-job-listing.json'
import type checkFreshness from './fixtures/check-freshness.json'
import type archiveJobListing from './fixtures/archive-job-listing.json'
import type unarchiveJobListing from './fixtures/unarchive-job-listing.json'
import type listApplicationsPopulated from './fixtures/list-applications.populated.json'
import type listGenerationsPopulated from './fixtures/list-generations.populated.json'
import type listApplicationsSparse from './fixtures/list-applications.sparse.json'
import type applicationStatsPopulated from './fixtures/application-stats.populated.json'
import type applicationStatsEmpty from './fixtures/application-stats.empty.json'
import type updateApplicationStatus from './fixtures/update-application-status.json'
import type updateApplicationMethod from './fixtures/update-application-method.json'
import type updateApplicationContact from './fixtures/update-application-contact.json'
import type applicationMailto from './fixtures/application-mailto.json'
import type recordGenerationPopulated from './fixtures/record-generation.populated.json'
import type recordGenerationSparse from './fixtures/record-generation.sparse.json'
import type addApplicationNote from './fixtures/add-application-note.json'
import type editApplicationNote from './fixtures/edit-application-note.json'
import type deleteApplicationNote from './fixtures/delete-application-note.json'
import type createGenerationPopulated from './fixtures/create-generation.populated.json'
import type createGenerationSparse from './fixtures/create-generation.sparse.json'
import type previewGeneration from './fixtures/preview-generation.json'
import type renderGenerationPopulated from './fixtures/render-generation.populated.json'
import type listAtsListingsPopulated from './fixtures/list-ats-listings.populated.json'
import type listTrackedBoardsPopulated from './fixtures/list-tracked-boards.populated.json'
import type addTrackedBoard from './fixtures/add-tracked-board.json'
import type authStatus from './fixtures/auth-status.json'
import type listPendingCapturesPopulated from './fixtures/list-pending-captures.populated.json'
import type listPendingCapturesSparse from './fixtures/list-pending-captures.sparse.json'
import type addPendingCapturePending from './fixtures/add-pending-capture.pending.json'
import type addPendingCaptureAlreadyTracked from './fixtures/add-pending-capture.already-tracked.json'
import type completePendingCapture from './fixtures/complete-pending-capture.json'
import type usagePopulated from './fixtures/usage.populated.json'
import type usageSparse from './fixtures/usage.sparse.json'

// The API contract (issue #99): every golden response fixture the backend's
// handler tests capture (backend/internal/api/contract_test.go) is asserted
// here against the type types.ts declares for that route, by `tsc -b` alone.
// A failure names the route, the JSON path and what disagrees; see check.ts
// for exactly what is compared and what 'populated'/'sparse' add.
//
// When a response changes on purpose, regenerate the fixtures with
//   cd backend && UPDATE_CONTRACT_FIXTURES=1 go test ./internal/api -run TestContract
// then change types.ts until `npm run build` passes. Never hand-edit a
// fixture to make this file compile.
//
// The last argument exempts a path from the populated/sparse extra check
// only, where the scenario cannot reach that variant; each says why.

// Master Data

export const listEntries: Contract<typeof listEntriesPopulated, Entry[], 'GET /api/master-data/entries (populated)', 'populated'> =
  true
export const getEntry: Contract<
  typeof getEntrySparse,
  Entry,
  'GET /api/master-data/entries/{id} (sparse)',
  'sparse',
  // An experience Entry cannot be valid without its employer and role.
  '$.employer' | '$.role'
> = true
export const postEntry: Contract<typeof createEntry, Entry, 'POST /api/master-data/entries'> = true
export const putEntry: Contract<typeof updateEntry, Entry, 'PUT /api/master-data/entries/{id}'> = true

export const getProfile: Contract<typeof getProfilePopulated, Profile, 'GET /api/master-data/profile (populated)', 'populated'> =
  true
export const getProfileWithoutSections: Contract<typeof getProfileSparse, Profile, 'GET /api/master-data/profile (sparse)', 'sparse'> =
  true
export const putProfile: Contract<typeof updateProfileSparse, Profile, 'PUT /api/master-data/profile (sparse)', 'sparse'> = true

export const listGenerations: Contract<
  typeof listGenerationsPopulated,
  IndexedGeneration[],
  'GET /api/generations',
  'populated'
> = true

export const getTagLint: Contract<typeof tagLint, TagLintReport, 'GET /api/master-data/tag-lint'> = true

export const listSnippets: Contract<
  typeof listSnippetsPopulated,
  Snippet[],
  'GET /api/master-data/cover-letter-snippets (populated)',
  'populated'
> = true
export const getSnippet: Contract<typeof getSnippetSparse, Snippet, 'GET /api/master-data/cover-letter-snippets/{id} (sparse)', 'sparse'> =
  true
export const postSnippet: Contract<typeof createSnippet, Snippet, 'POST /api/master-data/cover-letter-snippets'> = true
export const putSnippet: Contract<typeof updateSnippet, Snippet, 'PUT /api/master-data/cover-letter-snippets/{id}'> = true

// Job Listings

export const listJobListings: Contract<
  typeof listJobListingsPopulated,
  JobListingSummaryWithApplication[],
  'GET /api/job-listings (populated)',
  'populated'
> = true
export const listJobListingsWithoutOptionals: Contract<
  typeof listJobListingsSparse,
  JobListingSummaryWithApplication[],
  'GET /api/job-listings (sparse)',
  'sparse'
> = true
export const getJobListing: Contract<
  typeof getJobListingPopulated,
  JobListingWithApplication,
  'GET /api/job-listings/{id} (populated)',
  'populated',
  // One RAL Range is either a stated figure or a conflict between two; the
  // list fixture carries both.
  '$.jobListing.ral.min' | '$.jobListing.ral.max' | '$.jobListing.ral.currency'
> = true
export const getJobListingWithoutOptionals: Contract<
  typeof getJobListingSparse,
  JobListingWithApplication,
  'GET /api/job-listings/{id} (sparse)',
  'sparse'
> = true
export const saveJobListing: Contract<
  typeof saveJobListingSparse,
  SaveJobListingResult,
  'POST /api/job-listings (sparse)',
  'sparse',
  // A freshly saved Application always has both; only a migrated legacy one
  // lacks them (ADR-0034), which the GET sparse fixtures cover.
  '$.application.statusUpdatedAt' | '$.application.statusHistory'
> = true
export const saveJobListingWithDuplicate: Contract<
  typeof saveJobListingDuplicate,
  SaveJobListingResult,
  'POST /api/job-listings (duplicate warning)'
> = true
export const captureFromExtension: Contract<
  typeof captureJobListingFromExtension,
  SaveJobListingResult,
  'POST /api/job-listings/from-extension'
> = true
export const postSuggestContact: Contract<typeof suggestContact, Contact, 'POST /api/job-listings/{id}/suggest-contact'> = true
export const postResolve: Contract<typeof resolveJobListing, JobListingWithApplication, 'POST /api/job-listings/{id}/resolve'> = true
export const postCheckFreshness: Contract<typeof checkFreshness, JobListingResponse, 'POST /api/job-listings/{id}/check-freshness'> =
  true
export const postArchive: Contract<typeof archiveJobListing, JobListingResponse, 'POST /api/job-listings/{id}/archive'> = true
export const postUnarchive: Contract<typeof unarchiveJobListing, JobListingResponse, 'POST /api/job-listings/{id}/unarchive'> = true

// Applications

export const listApplications: Contract<
  typeof listApplicationsPopulated,
  ApplicationGroups,
  'GET /api/applications (populated)',
  'populated',
  // The Applications view computes no stale-Entry information; only the Job
  // Listings list and detail routes attach it.
  '$.groups[].items[].application.generations[].staleEntries'
> = true
export const listApplicationsWithoutOptionals: Contract<typeof listApplicationsSparse, ApplicationGroups, 'GET /api/applications (sparse)', 'sparse'> =
  true
export const getStats: Contract<typeof applicationStatsPopulated, ApplicationStats, 'GET /api/applications/stats (populated)'> = true
export const getStatsEmpty: Contract<typeof applicationStatsEmpty, ApplicationStats, 'GET /api/applications/stats (empty)'> = true
export const patchStatus: Contract<typeof updateApplicationStatus, Application, 'PATCH /api/applications/{id}/status'> = true
export const patchMethod: Contract<typeof updateApplicationMethod, Application, 'PATCH /api/applications/{id}/method'> = true
export const patchContact: Contract<typeof updateApplicationContact, Application, 'PATCH /api/applications/{id}/contact'> = true
export const getMailto: Contract<typeof applicationMailto, ApplicationMailto, 'GET /api/applications/{id}/mailto'> = true
export const recordGeneration: Contract<
  typeof recordGenerationPopulated,
  Application,
  'POST /api/applications/{id}/generations (populated)'
> = true
export const recordMinimalGeneration: Contract<
  typeof recordGenerationSparse,
  Application,
  'POST /api/applications/{id}/generations (sparse)',
  'sparse',
  // A new Application and a new Generation are always stamped; only legacy
  // records lack these (ADR-0034).
  | '$.statusUpdatedAt'
  | '$.statusHistory'
  | '$.generations'
  | '$.generations[].schemaVersion'
> = true
export const postNote: Contract<typeof addApplicationNote, Application, 'POST /api/applications/{id}/notes'> = true
export const patchNote: Contract<typeof editApplicationNote, Application, 'PATCH /api/applications/{id}/notes/{noteId}'> = true
export const deleteNote: Contract<typeof deleteApplicationNote, Application, 'DELETE /api/applications/{id}/notes/{noteId}'> = true

// Generation pipeline

export const generate: Contract<
  typeof createGenerationPopulated,
  GenerateResult,
  'POST /api/generations (populated)',
  'populated',
  // Generate reads no listing salary field, so its RAL Range is never a
  // conflict between two stated figures.
  '$.ral.descriptionStated' | '$.ral.listingStated'
> = true
export const generateDefaultMode: Contract<typeof createGenerationSparse, GenerateResult, 'POST /api/generations (sparse)', 'sparse'> =
  true
export const preview: Contract<typeof previewGeneration, GenerateResult, 'POST /api/generations/preview'> = true
export const render: Contract<
  typeof renderGenerationPopulated,
  RenderResult,
  'POST /api/generations/render (populated)',
  'populated',
  // A clean render of the real template parses 'ok', with nothing to report.
  | '$.cvParsability.missingFields'
  | '$.cvParsability.orderingViolations'
  | '$.cvParsability.reason'
  | '$.coverLetterParsability.missingFields'
  | '$.coverLetterParsability.orderingViolations'
  | '$.coverLetterParsability.reason'
> = true

// ATS job boards

export const listAtsListings: Contract<
  typeof listAtsListingsPopulated,
  AtsListing[],
  'GET /api/ats/{provider}/{slug}/listings (populated)',
  'populated',
  // No supported provider exposes a Company Logo yet (atsboard/types.go).
  '$[].logoUrl'
> = true
export const listTrackedBoards: Contract<
  typeof listTrackedBoardsPopulated,
  TrackedBoard[],
  'GET /api/ats/tracked-boards (populated)',
  'populated'
> = true
export const postTrackedBoard: Contract<typeof addTrackedBoard, AddedTrackedBoard, 'POST /api/ats/tracked-boards'> = true

// Pending Captures
export const getPendingCaptures: Contract<
  typeof listPendingCapturesPopulated,
  PendingCapture[],
  'GET /api/pending-captures (populated)',
  'populated'
> = true
export const getPendingCapturesSparse: Contract<
  typeof listPendingCapturesSparse,
  PendingCapture[],
  'GET /api/pending-captures (sparse)',
  'sparse'
> = true
export const postPendingCapture: Contract<typeof addPendingCapturePending, AddPendingCaptureResult, 'POST /api/pending-captures (pending)'> =
  true
export const postPendingCaptureAlreadyTracked: Contract<
  typeof addPendingCaptureAlreadyTracked,
  AddPendingCaptureResult,
  'POST /api/pending-captures (already tracked)'
> = true
export const postCompletePendingCapture: Contract<
  typeof completePendingCapture,
  SaveJobListingResult,
  'POST /api/pending-captures/{id}/complete'
> = true

// Access token
export const getAuthStatus: Contract<typeof authStatus, AuthStatus, 'GET /api/auth/status'> = true

// Usage

export const getUsage: Contract<typeof usagePopulated, UsageSummary, 'GET /api/usage (populated)', 'populated'> = true
export const getUsageComplete: Contract<typeof usageSparse, UsageSummary, 'GET /api/usage (sparse)', 'sparse'> = true
