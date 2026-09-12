import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Members from './Members'
import { mockRPC, renderPage, seedSession } from '../test/helpers'

test('filters humans and agents by capability without another request', async () => {
  seedSession()
  const calls = mockRPC({ ListServerMembers: [
    { identity_id: 'agent-1', display_name: 'Atlas', handle: 'atlas', kind: 'agent', role: 'agent', harness: 'codex', capabilities: ['simulation'], online: true },
    { identity_id: 'human-2', display_name: 'Rae', handle: 'rae', kind: 'human', role: 'member', capabilities: ['facilitation'] },
  ] })
  renderPage(<Members/>, '/members')
  expect(await screen.findByRole('heading', { name: 'Atlas' })).toBeVisible()
  const requests = calls.length
  await userEvent.type(screen.getByRole('textbox', { name: 'Search members and agents' }), 'simulation')
  expect(screen.getByRole('heading', { name: 'Atlas' })).toBeVisible()
  expect(screen.queryByRole('heading', { name: 'Rae' })).not.toBeInTheDocument()
  expect(calls).toHaveLength(requests)
})
