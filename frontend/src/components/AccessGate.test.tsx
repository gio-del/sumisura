import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import AccessGate, { ForgetDeviceButton } from '@/components/AccessGate'
import { listJobListings } from '@/api/client'
import { requestsTo, server } from '@/test/server'

function status(required: boolean, authenticated: boolean) {
  server.use(http.get('/api/auth/status', () => HttpResponse.json({ required, authenticated })))
}

function renderGate() {
  const user = userEvent.setup()
  render(
    <AccessGate>
      <p>the app</p>
      <ForgetDeviceButton />
    </AccessGate>,
  )
  return { user }
}

// AccessGate is the web app's side of token mode (issue #181): the token
// screen for a device without access, and nothing at all in default mode.
describe('AccessGate', () => {
  it('AccessGate_DefaultMode_RendersAppWithoutForgetButton', async () => {
    status(false, true)
    renderGate()

    expect(await screen.findByText('the app')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Forget this device' })).not.toBeInTheDocument()
  })

  it('AccessGate_TokenModeWithoutAccess_ShowsTokenScreenAndOpensOnCorrectToken', async () => {
    status(true, false)
    server.use(http.post('/api/auth/session', () => new HttpResponse(null, { status: 204 })))
    const { user } = renderGate()

    await user.type(await screen.findByLabelText('Access token'), ' s3cret ')
    expect(screen.queryByText('the app')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Continue' }))

    expect(await screen.findByText('the app')).toBeInTheDocument()
    const [req] = (await requestsTo('/api/auth/session')).filter((r) => r.method === 'POST')
    expect(req.body).toEqual({ token: 's3cret' })
  })

  it('AccessGate_WrongToken_ShowsInlineErrorAndStaysLocked', async () => {
    status(true, false)
    server.use(
      http.post('/api/auth/session', () => new HttpResponse('invalid access token\n', { status: 401 })),
    )
    const { user } = renderGate()

    await user.type(await screen.findByLabelText('Access token'), 'nope')
    await user.click(screen.getByRole('button', { name: 'Continue' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid access token')
    expect(screen.queryByText('the app')).not.toBeInTheDocument()
  })

  it('AccessGate_TokenModeWithAccess_ForgetThisDeviceClearsCookieAndLocks', async () => {
    status(true, true)
    server.use(http.delete('/api/auth/session', () => new HttpResponse(null, { status: 204 })))
    const { user } = renderGate()

    await user.click(await screen.findByRole('button', { name: 'Forget this device' }))

    expect(await screen.findByLabelText('Access token')).toBeInTheDocument()
    expect((await requestsTo('/api/auth/session')).filter((r) => r.method === 'DELETE')).toHaveLength(1)
  })

  it('AccessGate_ApiAnswers401_LocksTheApp', async () => {
    status(true, true)
    server.use(http.get('/api/job-listings', () => new HttpResponse('missing or invalid LAN auth token', { status: 401 })))
    renderGate()
    await screen.findByText('the app')

    await expect(listJobListings()).rejects.toThrow()

    expect(await screen.findByLabelText('Access token')).toBeInTheDocument()
  })

  it('AccessGate_BackendUnreachable_OffersRetry', async () => {
    server.use(http.get('/api/auth/status', () => HttpResponse.error()))
    renderGate()

    expect(await screen.findByRole('alert')).toHaveTextContent("Can't reach the Sumisura backend.")
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })
})
