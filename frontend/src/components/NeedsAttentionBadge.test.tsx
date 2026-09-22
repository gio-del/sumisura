import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import NeedsAttentionBadge from './NeedsAttentionBadge'
import { attentionCauses } from '@/lib/attentionCauses'
import type { ApplicationMethod, RALRange } from '@/api/types'

// "Needs attention" says what needs attention (issue #206, stories 71-78).
// The badge was a single word fired by two unrelated conditions, with no
// indication of which, what it meant, or what to do — and for an
// unresolved RAL Range there was no manual fix anywhere in the app.

const UNRESOLVED_RAL: RALRange = { source: 'unresolved' }
const RESOLVED_RAL: RALRange = { min: 50000, max: 60000, currency: 'EUR', source: 'estimated' }
const UNRESOLVED_METHOD: ApplicationMethod = { kind: 'unresolved' }
const RESOLVED_METHOD: ApplicationMethod = { kind: 'portal', value: 'https://acme.example/apply' }

describe('Which causes are firing', () => {
  // Story 72: only the causes actually firing are listed.
  it('Causes_OnlyTheOnesFiring_AreReported', () => {
    expect(attentionCauses(UNRESOLVED_RAL, RESOLVED_METHOD).map((c) => c.id)).toEqual(['ral'])
    expect(attentionCauses(RESOLVED_RAL, UNRESOLVED_METHOD).map((c) => c.id)).toEqual(['method'])
    expect(attentionCauses(UNRESOLVED_RAL, UNRESOLVED_METHOD).map((c) => c.id)).toEqual(['ral', 'method'])
  })

  // Story 78: the badge stays a reliable signal.
  it('Causes_NothingFiring_IsEmptySoTheBadgeDisappears', () => {
    expect(attentionCauses(RESOLVED_RAL, RESOLVED_METHOD)).toEqual([])
  })

  // A conflict, an N/A and a manual figure are answers, not problems.
  it('Causes_ARALThatWasResolved_IsNotACause', () => {
    for (const source of ['stated', 'estimated', 'n/a', 'conflict', 'manual'] as const) {
      expect(attentionCauses({ source }, RESOLVED_METHOD)).toEqual([])
    }
  })

  // Story 73: one plain sentence each, no internals.
  it('Causes_EachOne_ExplainsItselfInASentence', () => {
    for (const cause of attentionCauses(UNRESOLVED_RAL, UNRESOLVED_METHOD)) {
      expect(cause.explanation.length).toBeGreaterThan(20)
      expect(cause.explanation).not.toMatch(/unresolved|null|undefined/i)
    }
  })
})

function renderBadge(ral = UNRESOLVED_RAL, method = UNRESOLVED_METHOD, handlers = {}) {
  const user = userEvent.setup()
  const props = {
    ral,
    method,
    onRetry: vi.fn(),
    onEnterRAL: vi.fn(),
    onSetMethod: vi.fn(),
    retrying: false,
    ...handlers,
  }
  render(<NeedsAttentionBadge {...props} />)
  return { user, props }
}

describe('The badge', () => {
  it('Badge_NothingFiring_IsNotRendered', () => {
    render(
      <NeedsAttentionBadge
        ral={RESOLVED_RAL}
        method={RESOLVED_METHOD}
        onRetry={vi.fn()}
        onEnterRAL={vi.fn()}
        onSetMethod={vi.fn()}
        retrying={false}
      />,
    )

    expect(screen.queryByRole('button', { name: /needs attention/i })).not.toBeInTheDocument()
  })

  // Story 77: usable on touch, which is why this is a popover and not the
  // hover tooltip the page already has.
  it('Badge_Clicked_OpensWithoutNeedingHover', async () => {
    const { user } = renderBadge()

    await user.click(screen.getByRole('button', { name: /needs attention/i }))

    expect(await screen.findByText(/how much this role pays/i)).toBeInTheDocument()
  })

  it('Badge_OnlyTheRALFiring_NamesThatCauseAlone', async () => {
    const { user } = renderBadge(UNRESOLVED_RAL, RESOLVED_METHOD)

    await user.click(screen.getByRole('button', { name: /needs attention/i }))

    expect(await screen.findByText(/how much this role pays/i)).toBeInTheDocument()
    expect(screen.queryByText(/how to apply/i)).not.toBeInTheDocument()
  })

  // Stories 74, 76: the common fix is immediate, for both causes.
  it('Badge_Retry_RunsTheResolveTheAppAlreadyHad', async () => {
    const { user, props } = renderBadge()

    await user.click(screen.getByRole('button', { name: /needs attention/i }))
    await user.click((await screen.findAllByRole('button', { name: /retry/i }))[0])

    expect(props.onRetry).toHaveBeenCalledOnce()
  })

  // Story 75: a retry that keeps failing isn't a dead end.
  it('Badge_EnterTheRALMyself_IsOfferedBesideTheRetry', async () => {
    const { user, props } = renderBadge()

    await user.click(screen.getByRole('button', { name: /needs attention/i }))
    await user.click(await screen.findByRole('button', { name: /enter it myself/i }))

    expect(props.onEnterRAL).toHaveBeenCalledOnce()
  })

  it('Badge_SetTheMethodMyself_IsOfferedBesideItsRetry', async () => {
    const { user, props } = renderBadge()

    await user.click(screen.getByRole('button', { name: /needs attention/i }))
    await user.click(await screen.findByRole('button', { name: /set it myself/i }))

    expect(props.onSetMethod).toHaveBeenCalledOnce()
  })

  it('Badge_RetryInFlight_DoesNotOfferItTwice', async () => {
    const { user } = renderBadge(UNRESOLVED_RAL, UNRESOLVED_METHOD, { retrying: true })

    await user.click(screen.getByRole('button', { name: /needs attention/i }))

    for (const retry of await screen.findAllByRole('button', { name: /retry/i })) {
      expect(retry).toBeDisabled()
    }
  })
})
