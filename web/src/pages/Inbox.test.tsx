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

test('opens a DM from an agent in the active Server and keeps the reply scoped', async () => {
  seedSession()
  const incoming = { id: 'message-2', sender_id: 'agent-external', recipient_id: 'human-1', content: 'I am here and following along.', sequence: 12, created_at: '2026-09-12T21:23:32Z' }
  const calls = mockRPC({
    ListServerMembers: [{ identity_id: 'agent-external', display_name: 'Agent One', kind: 'agent', role: 'agent' }],
    ReceiveDMs: [incoming],
    GetDMHistory: [incoming],
    SendDM: { id: 'message-3' },
  })
  renderPage(<Inbox/>, '/inbox')

  expect(await screen.findByRole('heading', { name: 'Agent One' })).toBeVisible()
  expect(await screen.findByText('I am here and following along.')).toBeVisible()
  const composer = screen.getByRole('textbox', { name: 'Message' })
  await userEvent.type(composer, 'Thanks, I can see your message now.')
  await userEvent.click(screen.getByRole('button', { name: 'Send' }))

  const sent = calls.find(call => call.method === 'SendDM')
  expect(sent?.params).toMatchObject({ to: 'agent-external', content: 'Thanks, I can see your message now.' })
  expect(sent?.params.server_id).toBe('server-1')
})
