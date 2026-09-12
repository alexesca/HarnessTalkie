import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

type RPCEnvelope<T> = { result?: T; error?: { code: number; message: string } }
type Identity = { id: string; display_name: string; session_token: string }
type Server = { id: string; name: string }

async function rpc<T>(request: APIRequestContext, method: string, params: unknown = {}, token = ''): Promise<T> {
  const response = await request.post('/rpc', {
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    data: JSON.stringify({ jsonrpc: '2.0', id: Date.now(), method, params }),
  })
  expect(response.ok(), `${method} RPC should be available`).toBeTruthy()
  const body = await response.json() as RPCEnvelope<T>
  if (body.error) throw new Error(`${method}: ${body.error.message}`)
  return body.result as T
}

async function connect(page: Page, name: string, token = '') {
  await page.getByLabel('Identity name').fill(name)
  if (token) await page.getByLabel('Session token for an existing identity').fill(token)
  await page.getByRole('button', { name: 'Connect', exact: true }).click()
  await expect(page.getByTestId('identity-status')).toHaveText('Connected')
}

test.describe('human collaboration application', () => {
  test('connects, discovers a Server, creates a group, preserves a deep link, and remains usable on mobile', async ({ page, request }) => {
    const suffix = `${Date.now()}-${Math.floor(Math.random() * 10000)}`
    const identityName = `Playwright human ${suffix}`
    const serverName = `Playwright workspace ${suffix}`
    const groupName = `Review room ${suffix}`
    const peerName = `Playwright agent ${suffix}`

    const identity = await rpc<Identity>(request, 'CreateOrLoadIdentity', { identity: identityName })
    const server = await rpc<Server>(request, 'CreateServer', {
      name: serverName,
      description: 'A real backend E2E workspace',
      purpose: 'Browser collaboration verification',
      join_policy: 'public',
      discoverable: true,
      tags: ['e2e'],
    }, identity.session_token)
    const peer = await rpc<Identity>(request, 'CreateOrLoadIdentity', { identity: peerName })
    await rpc(request, 'JoinServer', { server_id: server.id }, peer.session_token)

    await page.goto('/')
    await expect(page.getByRole('heading', { name: /Collaboration without the ceremony/i })).toBeVisible()
    await connect(page, identityName, identity.session_token)

    await page.getByRole('link', { name: 'Servers', exact: true }).click()
    await expect(page).toHaveURL(/\/servers$/)
    await expect(page.getByRole('heading', { name: 'Servers', exact: true })).toBeVisible()
    const serverCard = page.locator('.server-card').filter({ hasText: serverName })
    await expect(serverCard).toBeVisible()
    await serverCard.getByRole('button', { name: 'Open Server', exact: true }).click()
    await expect(page).toHaveURL(new RegExp(`/servers/${server.id}/overview$`))
    await expect(page.getByTestId('server-visible')).toContainText(server.id)
    await expect(page.getByRole('heading', { name: serverName, exact: true })).toBeVisible()

    await page.getByRole('link', { name: 'Members', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Members & agents', exact: true })).toBeVisible()
    await expect(page.getByTestId('server-members-visible')).toBeVisible()
    await expect(page.getByTestId('server-members-visible').getByRole('heading', { name: identityName, exact: true })).toBeVisible()
    await expect(page.getByTestId('server-members-visible').getByRole('heading', { name: peerName, exact: true })).toBeVisible()

    await page.goto(`/dm/${peer.id}`)
    await expect(page.getByRole('heading', { name: peerName, exact: true })).toBeVisible()
    await page.getByTestId('dm-content').fill('Hello from the real Playwright workflow')
    await page.getByTestId('dm-send').click()
    await expect(page.getByTestId('dm-messages')).toContainText('Hello from the real Playwright workflow')

    await page.getByRole('link', { name: 'Groups', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Groups', exact: true })).toBeVisible()
    await page.getByTestId('group-name').fill(groupName)
    await page.getByRole('button', { name: 'Create', exact: true }).click()
    await expect(page.getByRole('status')).toContainText(`Created ${groupName}`)
    await expect(page.getByRole('button', { name: new RegExp(`# ${groupName}`) })).toBeVisible()
    await page.getByTestId('group-message').fill('Playwright group collaboration')
    await page.getByTestId('group-send').click()
    await expect(page.getByTestId('group-messages')).toContainText('Playwright group collaboration')

    const postTitle = `Playwright decision ${suffix}`
    await page.getByRole('link', { name: 'Forums', exact: true }).click()
    await page.getByTestId('post-title').fill(postTitle)
    await page.getByTestId('post-content').fill('A real forum workflow with a durable reply')
    await page.getByTestId('post-create').click()
    const postLink = page.getByRole('link').filter({ hasText: postTitle })
    await expect(postLink).toBeVisible()
    await postLink.click()
    await page.getByTestId('comment-content').fill('Playwright nested discussion reply')
    await page.getByTestId('comment-send').click()
    await expect(page.getByTestId('comments')).toContainText('Playwright nested discussion reply')
    await page.getByTestId('thread-follow').click()
    await expect(page.getByRole('status')).toContainText('Following this discussion')
    await page.getByTestId('react').click()
    await expect(page.getByRole('status')).toContainText('Reaction added')

    // The routed Server URL is a bookmarkable deep link. Refreshing it must
    // reuse the session and active-server context rather than showing setup.
    await page.goto(`/servers/${server.id}/overview`)
    await expect(page.getByTestId('server-visible')).toContainText(server.id)
    await page.reload()
    await expect(page.getByTestId('server-visible')).toContainText(server.id)
    await expect(page.getByRole('heading', { name: serverName, exact: true })).toBeVisible()

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.getByRole('button', { name: 'Open navigation' })).toBeVisible()
    await page.getByRole('button', { name: 'Open navigation' }).click()
    await expect(page.getByRole('link', { name: 'Forums', exact: true })).toBeVisible()
    await expect(page.getByRole('main')).toBeVisible()
  })

  test('approves an agent and assigns its Server role through administration', async ({ page, request }) => {
    const suffix = `${Date.now()}-${Math.floor(Math.random() * 10000)}`
    const ownerName = `Playwright owner ${suffix}`
    const agentName = `Playwright agent ${suffix}`
    const owner = await rpc<Identity>(request, 'CreateOrLoadIdentity', { identity: ownerName })
    const agent = await rpc<Identity>(request, 'CreateOrLoadIdentity', { identity: agentName })
    const server = await rpc<Server>(request, 'CreateServer', {
      name: `Approval workspace ${suffix}`,
      join_policy: 'approval-required',
      discoverable: true,
    }, owner.session_token)
    await rpc(request, 'RequestServerAccess', { server_id: server.id, reason: 'E2E approval' }, agent.session_token)

    await page.goto('/')
    await connect(page, ownerName, owner.session_token)
    await page.goto('/settings')
    await page.getByLabel('Open Server administration').fill(server.id)
    await page.getByRole('button', { name: 'Open', exact: true }).click()
    await expect(page.getByTestId('server-visible')).toContainText(server.id)
    await page.getByRole('button', { name: 'Approve', exact: true }).click()
    await expect(page.getByTestId('server-requests-visible')).toContainText('No pending request')

    await page.getByLabel('Participant').selectOption(agent.id)
    await page.getByTestId('server-role-value').selectOption('moderator')
    await page.getByRole('button', { name: 'Assign role', exact: true }).click()
    await expect(page.getByTestId('server-role-result')).toContainText('Role updated to moderator')

    const member = await rpc<{ role: string }>(request, 'GetServerMember', {
      server_id: server.id,
      participant_id: agent.id,
    }, owner.session_token)
    expect(member.role).toBe('moderator')
  })
})
