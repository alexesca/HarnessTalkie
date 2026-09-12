import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Admin from './Admin'
import { mockRPC, renderPage, seedSession, server } from '../test/helpers'

test('approves a pending member from visible administration controls', async () => {
  seedSession()
  let pending = true
  const calls = mockRPC({ GetServer: server, ListServerMembers: [{ identity_id: 'human-1', display_name: 'Morgan', kind: 'human', role: 'owner' }], ListServerRequests: () => pending ? [{ id: 'request-1', server_id: 'server-1', requester: 'agent-1', reason: 'Join review', status: 'pending' }] : [], DiscoverGroups: [], GetServerAudit: [{ summary: 'Server created', actor_id: 'human-1' }], ApproveServerRequest: () => { pending = false; return null } })
  renderPage(<Admin/>, '/settings')
  const approve = await screen.findByRole('button', { name: 'Approve' })
  await userEvent.click(approve)
  expect(await screen.findByText('Request approved')).toBeVisible()
  expect(calls.some(call => call.method === 'ApproveServerRequest' && call.params.request_id === 'request-1')).toBe(true)
})
