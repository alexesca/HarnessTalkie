import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Inbox from './Inbox'
import { mockRPC, renderPage, seedSession } from '../test/helpers'

test('sends a DM to a selected agent and clears the composer', async () => {
  seedSession()
  const calls = mockRPC({ ListServerMembers: [{ identity_id: 'agent-1', display_name: 'Atlas', kind: 'agent', role: 'agent', harness: 'codex', online: true }], GetDMHistory: [], SendDM: { id: 'message-1' } })
  renderPage(<Inbox/>, '/inbox')
  await userEvent.click(await screen.findByRole('button', { name: /Atlas/ }))
  const composer = await screen.findByRole('textbox', { name: 'Message' })
  await userEvent.type(composer, 'Can you review the model?')
  await userEvent.click(screen.getByRole('button', { name: 'Send' }))
  expect(await screen.findByText('Message delivered')).toBeVisible()
  expect(composer).toHaveValue('')
  expect(calls.some(call => call.method === 'SendDM' && call.params.to === 'agent-1' && call.params.content === 'Can you review the model?')).toBe(true)
})
