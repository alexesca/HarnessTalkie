import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { ReactNode } from 'react'
import { SessionProvider } from '../session'
import type { Identity, Server } from '../lib/types'

export const identity: Identity = { id: 'human-1', display_name: 'Morgan', session_token: 'test-token' }
export const server: Server = { id: 'server-1', name: 'Research Commons', description: 'Shared research workspace', owner_id: identity.id, join_policy: 'public', discoverable: true, member_count: 3 }

export function seedSession(activeServer: Server | undefined = server) {
  sessionStorage.setItem('ht.session.v2', JSON.stringify(identity))
  if (activeServer) localStorage.setItem('ht.active-server.v2', JSON.stringify(activeServer))
  else localStorage.removeItem('ht.active-server.v2')
}

export function renderPage(page: ReactNode, route = '/') {
  return render(<MemoryRouter initialEntries={[route]}><SessionProvider>{page}</SessionProvider></MemoryRouter>)
}

export function mockRPC(results: Record<string, unknown | ((params: any) => unknown)>) {
  const calls: Array<{ method: string; params: any }> = []
  const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    const request = JSON.parse(String(init?.body)) as { method: string; params: any }
    calls.push(request)
    const configured = results[request.method]
    const result = typeof configured === 'function' ? configured(request.params) : configured
    return new Response(JSON.stringify({ jsonrpc: '2.0', id: 1, result }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fetchMock)
  return calls
}
