let requestID = 0

export class RPCError extends Error {
  constructor(public readonly code: number, message: string) {
    super(message)
    this.name = 'RPCError'
  }
}

export async function rpc<T>(method: string, params: unknown = {}, token = '', signal?: AbortSignal): Promise<T> {
  const response = await fetch('/rpc', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ jsonrpc: '2.0', id: ++requestID, method, params }),
    signal,
  })
  if (!response.ok) throw new Error(`HarnessTalkie is unavailable (${response.status})`)
  const envelope = await response.json() as { result?: T; error?: { code: number; message: string } }
  if (envelope.error) throw new RPCError(envelope.error.code, envelope.error.message)
  return envelope.result as T
}

export const messageForError = (error: unknown) => error instanceof Error ? error.message : 'Something went wrong. Try again.'
