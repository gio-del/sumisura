import type {
  Application,
  ApplicationGroups,
  ApplicationStatus,
  Entry,
  GenerateResult,
  GenerationRecord,
  JobListing,
  JobListingSummary,
  JobListingSummaryWithApplication,
  JobListingWithApplication,
  RenderResult,
} from '@/api/types'

// Response builders mirroring what the Go handlers actually return, so a
// test states only the field it is about. The backend's `seedDataDir` /
// `saveListing` helpers are the model (see backend/internal/api/*_test.go).
//
// Every record read carries an opaque version token (issue #89), so the
// builders give each one a fixed default a test can assert is sent back as
// If-Match. The Job Listing and its Application are two files with two
// tokens, so their defaults differ.
export const JOB_LISTING_VERSION = 'job-listing-v1'
export const APPLICATION_VERSION = 'application-v1'

/**
 * jobListing is a Job Listing as the backend returns it, with every optional
 * field (title, url, logo, freshnessCheckedAt) left absent unless asked for —
 * the "older record, written before those fields existed" case the backend
 * types document.
 */
export function jobListing(overrides: Partial<JobListing> = {}): JobListing {
  return {
    schemaVersion: 1,
    id: 'acme',
    company: 'Acme',
    source: 'manual',
    savedAt: '2026-01-05T10:00:00Z',
    jobDescription: 'We are hiring a Backend Engineer.',
    ral: { source: 'n/a' },
    freshnessStatus: 'not-yet-checked',
    archived: false,
    version: JOB_LISTING_VERSION,
    ...overrides,
  }
}

export function application(overrides: Partial<Application> = {}): Application {
  return {
    schemaVersion: 1,
    id: 'acme',
    jobListingId: 'acme',
    status: 'saved',
    method: { kind: 'portal', value: 'https://acme.example/apply' },
    isStale: false,
    version: APPLICATION_VERSION,
    ...overrides,
  }
}

/**
 * generationRecord is one Generation recorded against an Application, with
 * the zero-valued usage the backend always sends and no optional field set.
 */
export function generationRecord(overrides: Partial<GenerationRecord> = {}): GenerationRecord {
  return {
    schemaVersion: 1,
    slug: 'acme-1',
    createdAt: '2026-01-10T10:00:00Z',
    cvPath: 'output/acme-1/cv.pdf',
    usage: { inputTokens: 0, outputTokens: 0, estimatedCostUsd: 0 },
    ...overrides,
  }
}

export function listingWithApplication(
  listingOverrides: Partial<JobListing> = {},
  applicationOverrides: Partial<Application> = {},
): JobListingWithApplication {
  const listing = jobListing(listingOverrides)
  return {
    jobListing: listing,
    application: application({ id: listing.id, jobListingId: listing.id, ...applicationOverrides }),
  }
}

/**
 * jobListingSummary is a Job Listing as GET /api/job-listings returns it
 * (issue #97): the same fields as jobListing, but only whether there is a Job
 * Description rather than its text.
 */
export function jobListingSummary(overrides: Partial<JobListingSummary> = {}): JobListingSummary {
  return {
    schemaVersion: 1,
    id: 'acme',
    company: 'Acme',
    source: 'manual',
    savedAt: '2026-01-05T10:00:00Z',
    hasJobDescription: true,
    ral: { source: 'n/a' },
    freshnessStatus: 'not-yet-checked',
    archived: false,
    version: JOB_LISTING_VERSION,
    ...overrides,
  }
}

export function listingSummaryWithApplication(
  listingOverrides: Partial<JobListingSummary> = {},
  applicationOverrides: Partial<Application> = {},
): JobListingSummaryWithApplication {
  const listing = jobListingSummary(listingOverrides)
  return {
    jobListing: listing,
    application: application({ id: listing.id, jobListingId: listing.id, ...applicationOverrides }),
  }
}

const APPLICATION_GROUP_ORDER: ApplicationStatus[] = [
  'saved',
  'tailoring',
  'sent',
  'interviewing',
  'offer',
  'rejected',
  'withdrawn',
]

/**
 * applicationGroups is GET /api/applications's envelope: every Status group
 * in the backend's order, counts filled in, holding the rows given for each
 * Status in the order given (the backend has already sorted them).
 */
export function applicationGroups(
  byStatus: Partial<Record<ApplicationStatus, JobListingSummaryWithApplication[]>> = {},
): ApplicationGroups {
  const groups = APPLICATION_GROUP_ORDER.map((status) => {
    const items = byStatus[status] ?? []
    return { status, count: items.length, items }
  })
  return { total: groups.reduce((sum, g) => sum + g.count, 0), groups }
}

export function entry(overrides: Partial<Entry> = {}): Entry {
  return {
    id: 'acme-backend',
    type: 'experience',
    employer: 'Globex',
    role: 'Backend Engineer',
    start: '2021-01',
    end: null,
    tags: ['go'],
    version: 'entry-v1',
    ...overrides,
  }
}

/**
 * generateResult is a Generate call's result: two Entries of two bullets
 * each, usage, and no Cover Letter unless asked for. Each bullet's rewritten
 * text equals its source so the Text Review checkboxes have stable
 * accessible names — the word-level diff is BulletDiff's own concern.
 */
export function generateResult(overrides: Partial<GenerateResult> = {}): GenerateResult {
  return {
    mode: 'tailored',
    selection: {
      entries: [
        {
          entryId: 'globex-backend',
          reason: 'Closest match to the Job Description.',
          bullets: [
            { sourceIndex: 0, source: 'Built a Go HTTP API.', rewritten: 'Built a Go HTTP API.' },
            { sourceIndex: 1, source: 'Owned deploys end to end.', rewritten: 'Owned deploys end to end.' },
          ],
        },
        {
          entryId: 'pathfinder',
          reason: 'Shows routing work.',
          bullets: [
            { sourceIndex: 0, source: 'Wrote a route planner.', rewritten: 'Wrote a route planner.' },
            { sourceIndex: 1, source: 'Published it as open source.', rewritten: 'Published it as open source.' },
          ],
        },
      ],
    },
    usage: { inputTokens: 1200, outputTokens: 340, estimatedCostUsd: 0.0421 },
    language: 'en',
    ...overrides,
  }
}

/** selectedEntries are the Entries a Generate result offers at Text Review. */
export function generationEntries(): Entry[] {
  return [
    entry({ id: 'globex-backend', type: 'experience', employer: 'Globex' }),
    entry({ id: 'pathfinder', type: 'project', name: 'Pathfinder', employer: undefined, role: undefined }),
  ]
}

export function renderResult(overrides: Partial<RenderResult> = {}): RenderResult {
  return {
    slug: 'acme',
    cvPath: 'output/acme/cv.pdf',
    cvPageCount: 1,
    atsReports: { cv: { status: 'ok' } },
    ...overrides,
  }
}
