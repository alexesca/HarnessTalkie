import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Servers from './Servers'
import { mockRPC, renderPage, seedSession } from '../test/helpers'

test('discovers an approval Server and shows durable request state', async () => {
  seedSession(undefined)
  const calls = mockRPC({ DiscoverServers: [{ id: 'server-2', name: 'Safety Guild', description: 'Review high-risk work', owner_id: 'owner', join_policy: 'approval-required', discoverable: true, member_count: 8 }], RequestServerAccess: { id: 'request-1' } })
  renderPage(<Servers/>, '/servers')
  expect(await screen.findByRole('heading', { name: 'Safety Guild' })).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: 'Request access' }))
  expect(await screen.findByText(/Access requested for Safety Guild/)).toBeVisible()
  expect(calls.some(call => call.method === 'RequestServerAccess' && call.params.server_id === 'server-2')).toBe(true)
})
