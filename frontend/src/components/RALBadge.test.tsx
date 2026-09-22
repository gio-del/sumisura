import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import RALBadge from './RALBadge'

// The RAL Range's source is a domain distinction (CONTEXT.md): a Stated
// figure is a fact, an Estimated one is Claude's guess, a Conflict means the
// two sources disagree and neither is picked, and N/A means nothing was
// found — never zero. Collapsing any of those is silently misleading, so
// each is pinned here by the text a user actually reads.
describe('RALBadge', () => {
  it('RALBadge_Stated_ShowsTheRangeLabelledAsStated', () => {
    render(<RALBadge ral={{ min: 45000, max: 55000, currency: 'EUR', source: 'stated' }} />)

    expect(screen.getByText('RAL Range: EUR 45,000 – 55,000')).toBeInTheDocument()
    expect(screen.getByText('Stated in the Job Description')).toBeInTheDocument()
  })

  it('RALBadge_Estimated_LabelsTheFigureAsAGuessRatherThanAFact', () => {
    render(<RALBadge ral={{ min: 40000, max: 50000, currency: 'EUR', source: 'estimated' }} />)

    expect(screen.getByText('RAL Range: EUR 40,000 – 50,000')).toBeInTheDocument()
    expect(screen.getByText("Claude's estimate — not a fact")).toBeInTheDocument()
  })

  it('RALBadge_Conflict_ShowsBothRangesRatherThanPickingOne', () => {
    render(
      <RALBadge
        ral={{
          source: 'conflict',
          descriptionStated: { min: 40000, max: 45000, currency: 'EUR' },
          listingStated: { min: 60000, max: 70000, currency: 'EUR' },
        }}
      />,
    )

    expect(screen.getByText('RAL Range: Conflict')).toBeInTheDocument()
    expect(screen.getByText('Job Description: EUR 40,000 – 45,000')).toBeInTheDocument()
    expect(screen.getByText('Listing: EUR 60,000 – 70,000')).toBeInTheDocument()
  })

  it('RALBadge_NotAvailable_ReadsAsAnAbsenceRatherThanZero', () => {
    render(<RALBadge ral={{ source: 'n/a' }} />)

    expect(screen.getByText('RAL Range: Not available')).toBeInTheDocument()
    expect(screen.getByText('Not found')).toBeInTheDocument()
    expect(screen.queryByText(/\b0\b/)).not.toBeInTheDocument()
  })

  it('RALBadge_SingleFigure_ShowsOneAmountRatherThanARepeatedRange', () => {
    render(<RALBadge ral={{ min: 50000, max: 50000, currency: 'EUR', source: 'stated' }} />)

    expect(screen.getByText('RAL Range: EUR 50,000')).toBeInTheDocument()
  })
})

// A figure the user entered themselves (issue #206, story 65): never
// shown as though Claude had estimated it.
describe('A RAL Range entered by hand', () => {
  it('RALBadge_ManualSource_SaysTheFigureIsYours', () => {
    render(<RALBadge ral={{ min: 55000, max: 65000, currency: 'EUR', source: 'manual' }} />)

    expect(screen.getByText(/EUR 55,000/)).toBeInTheDocument()
    expect(screen.getByText(/entered by you/i)).toBeInTheDocument()
    expect(screen.queryByText(/estimate/i)).not.toBeInTheDocument()
  })

  it('RALBadge_ManualSource_IsVisuallyDistinctFromAnEstimate', () => {
    const { container: manual } = render(<RALBadge ral={{ min: 55000, max: 65000, currency: 'EUR', source: 'manual' }} />)
    const manualClass = manual.querySelector('div')?.className ?? ''

    const { container: estimated } = render(<RALBadge ral={{ min: 55000, max: 65000, currency: 'EUR', source: 'estimated' }} />)
    const estimatedClass = estimated.querySelector('div')?.className ?? ''

    expect(manualClass).not.toEqual(estimatedClass)
  })
})
