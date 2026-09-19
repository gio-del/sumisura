import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { ATSReports } from '@/api/types'
import ATSReportPage from '@/pages/ATSReportPage'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

const SLUG = 'acme-corp-20260911-143022'

function showReport(answer: () => Response) {
  server.use(http.get(`/api/generations/${SLUG}/ats-report`, answer))
  return renderPage(<ATSReportPage />, { at: `/generations/${SLUG}/ats`, pattern: '/generations/:slug/ats' })
}

const reports: ATSReports = {
  cv: {
    status: 'warning',
    fields: [
      { label: 'Name', group: 'identity', found: true, inOrder: true },
      { label: 'Phone', group: 'contact', found: false, inOrder: false },
      { label: 'Projects section header', group: 'section', found: true, inOrder: false },
    ],
    missingFields: ['Phone'],
    orderingViolations: ['Projects section header appears before Experience section header in the extracted text'],
    termCoverage: { present: ['Go'], missing: ['Kubernetes'] },
    extractedText: 'Jane Doe\njane@example.com\nProjects\nExperience\n',
  },
  coverLetter: { status: 'unavailable', reason: 'pdftotext failed: executable file not found' },
}

// The ATS Report view (issue #198): what a screener reads from a Generation's
// PDFs, kept on the Generation so it can be checked right before sending.
describe('ATSReportPage', () => {
  it('ATSReportPage_ShowsFieldVerdictsTermsAndTheExtractedText', async () => {
    showReport(() => HttpResponse.json(reports))

    const cv = await screen.findByRole('region', { name: 'CV ATS Report' })
    expect(within(cv).getByRole('row', { name: /Phone Contact missing/ })).toBeInTheDocument()
    expect(within(cv).getByRole('row', { name: /Projects section header Section headers out of order/ })).toBeInTheDocument()
    expect(within(cv).getByRole('row', { name: /Name Name found/ })).toBeInTheDocument()
    expect(within(cv).getByText(/1 of 2 terms/)).toBeInTheDocument()
    expect(within(cv).getByLabelText('Kubernetes (not on this CV)')).toBeInTheDocument()
    expect(within(cv).getByText(/jane@example\.com/)).toBeInTheDocument()

    const letter = screen.getByRole('region', { name: 'Cover Letter ATS Report' })
    expect(within(letter).getByText(/couldn.t run, so this says nothing about the PDF itself/)).toHaveTextContent(
      'pdftotext failed',
    )
  })

  it('ATSReportPage_NoneRecorded_SaysSoInsteadOfPassing', async () => {
    showReport(() => new HttpResponse('no ATS report recorded for this generation', { status: 404 }))

    expect(await screen.findByText(/No ATS Report was recorded for this Generation/)).toBeInTheDocument()
    expect(screen.queryByText('CV: ok', { exact: false })).not.toBeInTheDocument()
  })
})
