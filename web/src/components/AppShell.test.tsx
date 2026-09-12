import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { AppShell } from './AppShell'
import Overview from '../pages/Overview'
import { SessionProvider } from '../session'
import { mockRPC, seedSession, server } from '../test/helpers'
import type { Server } from '../lib/types'

const second: Server = { ...server, id: 'server-2', name: 'Applied Safety Lab', owner_id: 'owner-2', member_count: 9 }

function renderShell(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}><SessionProvider><Routes><Route element={<AppShell/>}>
      <Route index element={<div>Home content</div>}/>
      <Route path="servers/:serverId/overview" element={<div>Server content</div>}/>
      <Route path="servers/:serverId/members" element={<div>Members content</div>}/>
    </Route></Routes></SessionProvider></MemoryRouter>,
  )
}

test('loads joined Servers, recovers a stale selection, and switches scoped navigation', async () => {
  seedSession({ ...server, id: 'removed-server', name: 'Removed' })
  const calls = mockRPC({ ListServers: [server, second] })
  renderShell()
  expect(await screen.findByRole('button', { name: 'Research Commons' })).toHaveAttribute('aria-current', 'page')
  expect(screen.getByRole('button', { name: 'Applied Safety Lab' })).toHaveTextContent('AS')
  expect(screen.getByTestId('nav-members')).toHaveAttribute('href', '/servers/server-1/members')
  expect(screen.getByRole('link', { name: 'HarnessTalkie Home' })).toHaveAttribute('href', '/')
  expect(screen.getByRole('link', { name: 'Explore Servers' })).toHaveAttribute('href', '/servers')
  expect(screen.getByRole('link', { name: 'Create Server' })).toHaveAttribute('href', '/servers?intent=create')
  expect(screen.getByTestId('nav-inbox')).toHaveAttribute('href', '/inbox')
  expect(screen.getByTestId('nav-notifications')).toHaveAttribute('href', '/notifications')
  expect(screen.getByTestId('nav-profile')).toHaveAttribute('href', '/profile')
  expect(screen.getByTestId('nav-admin')).toHaveAttribute('href', '/servers/server-1/settings')
  expect(screen.getByRole('button', { name: 'Disconnect identity' })).toBeVisible()

  await userEvent.click(screen.getByRole('button', { name: 'Applied Safety Lab' }))
  expect(await screen.findByText('Server content')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Applied Safety Lab' })).toHaveAttribute('aria-current', 'page')
  expect(screen.getByTestId('nav-members')).toHaveAttribute('href', '/servers/server-2/members')
  expect(calls.filter(call => call.method === 'ListServers')).toHaveLength(1)
})

test('synchronizes a different Server deep link through GetServer', async () => {
  seedSession(server)
  const calls = mockRPC({ ListServers: [server, second], GetServer: second })
  renderShell('/servers/server-2/members')
  expect(await screen.findByText('Members content')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Applied Safety Lab' })).toHaveAttribute('aria-current', 'page')
  expect(calls.some(call => call.method === 'GetServer' && call.params.server_id === 'server-2')).toBe(true)
})

test('shows loading before a successful zero-Server onboarding state', async () => {
  seedSession(undefined)
  let resolve!: (response: Response) => void
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(done => { resolve = done })))
  render(<MemoryRouter><SessionProvider><Overview/></SessionProvider></MemoryRouter>)
  expect(screen.getByRole('status', { name: 'Loading your Servers' })).toBeVisible()
  resolve(new Response(JSON.stringify({ result: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  expect(await screen.findByRole('heading', { name: 'Create your first Server.' })).toBeVisible()
  expect(screen.getByRole('link', { name: 'Create a Server' })).toHaveAttribute('href', '/servers?intent=create')
  expect(screen.getByRole('link', { name: 'Explore public Servers' })).toBeVisible()
})

test('shows membership errors with a working Retry instead of onboarding', async () => {
  seedSession(undefined)
  let request = 0
  vi.stubGlobal('fetch', vi.fn(async () => ++request === 1
    ? new Response('', { status: 503 })
    : new Response(JSON.stringify({ result: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })))
  render(<MemoryRouter><SessionProvider><Overview/></SessionProvider></MemoryRouter>)
  expect(await screen.findByRole('alert')).toHaveTextContent('HarnessTalkie is unavailable (503)')
  expect(screen.queryByText('Create your first Server.')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  await waitFor(() => expect(screen.getByRole('heading', { name: 'Create your first Server.' })).toBeVisible())
})
